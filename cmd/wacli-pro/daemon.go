package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	appPkg "github.com/UAEpro/wacli-pro/internal/app"
	"github.com/UAEpro/wacli-pro/internal/config"
	"github.com/UAEpro/wacli-pro/internal/ipc"
	"github.com/UAEpro/wacli-pro/internal/out"
	"github.com/kardianos/service"
	"github.com/spf13/cobra"
)

// serviceName is the OS-level service identifier (systemd unit / launchd label /
// Windows service name). Kept stable so start/stop/status/uninstall all resolve
// to the same registered service.
const serviceName = "wacli-sync"

// systemdUserScript installs wacli-sync as a *user* systemd service that
// auto-starts on boot (via lingering) and restarts on crash. The default
// kardianos template targets multi-user.target, which does not reliably
// activate for `systemctl --user`; default.target does.
const systemdUserScript = `[Unit]
Description={{.Description}}
ConditionFileIsExecutable={{.Path|cmdEscape}}
StartLimitIntervalSec=0

[Service]
ExecStart={{.Path|cmdEscape}}{{range .Arguments}} {{.|cmd}}{{end}}
Restart=always
RestartSec=10

[Install]
WantedBy=default.target
`

// daemonOpts are the sync options baked into the installed service definition.
type daemonOpts struct {
	downloadMedia   bool
	refreshContacts bool
	refreshGroups   bool
}

func newDaemonCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Run background sync as a native OS service (auto-starts on boot)",
		Long: "Install and manage a background WhatsApp sync service.\n\n" +
			"`daemon start` registers wacli as a native service for the current OS\n" +
			"(systemd on Linux/WSL, launchd on macOS, Service Manager on Windows) so it\n" +
			"keeps syncing across reboots and restarts automatically if it crashes.",
	}
	cmd.AddCommand(newDaemonStartCmd(flags))
	cmd.AddCommand(newDaemonStopCmd(flags))
	cmd.AddCommand(newDaemonRestartCmd(flags))
	cmd.AddCommand(newDaemonStatusCmd(flags))
	cmd.AddCommand(newDaemonUninstallCmd(flags))
	cmd.AddCommand(newDaemonLogsCmd(flags))
	cmd.AddCommand(newDaemonRunCmd(flags))
	return cmd
}

func daemonStoreDir(flags *rootFlags) string {
	dir := flags.storeDir
	if dir == "" {
		dir = config.DefaultStoreDir()
	}
	dir, _ = filepath.Abs(dir)
	return dir
}

func daemonLogPath(storeDir string) string {
	return filepath.Join(storeDir, "daemon.log")
}

// buildService constructs the kardianos service for the current platform. The
// same Config is used for control (start/stop/status/uninstall) and for the
// in-process run entrypoint; only `daemon run` actually invokes prg.Start.
func buildService(flags *rootFlags, dopts daemonOpts, prg service.Interface) (service.Service, string, error) {
	storeDir := daemonStoreDir(flags)

	args := []string{"daemon", "run", "--store", storeDir}
	if dopts.downloadMedia {
		args = append(args, "--download-media")
	}
	if dopts.refreshContacts {
		args = append(args, "--refresh-contacts")
	}
	if dopts.refreshGroups {
		args = append(args, "--refresh-groups")
	}

	opts := service.KeyValue{
		// Install as a per-user service where supported (systemd --user,
		// launchd LaunchAgent) so no root/admin is required and it starts via
		// user lingering. Windows services are always system-level.
		"UserService": runtime.GOOS != "windows",
		// systemd: auto-restart on crash.
		"Restart": "always",
		// launchd: keep alive + start at load.
		"KeepAlive": true,
		"RunAtLoad": true,
	}
	if runtime.GOOS == "linux" {
		opts["SystemdScript"] = systemdUserScript
	}

	cfg := &service.Config{
		Name:        serviceName,
		DisplayName: "wacli WhatsApp Sync",
		Description: "Keeps WhatsApp messages synced into the local wacli-pro store.",
		Arguments:   args,
		Option:      opts,
	}

	s, err := service.New(prg, cfg)
	return s, storeDir, err
}

// program is the kardianos service.Interface implementation. When the OS service
// manager launches `daemon run`, prg.Start spawns the follow-sync loop.
type program struct {
	flags *rootFlags
	dopts daemonOpts

	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}

func (p *program) Start(s service.Service) error {
	// Must not block: kardianos (and Windows SCM) require Start to return promptly.
	p.ctx, p.cancel = context.WithCancel(context.Background())
	p.done = make(chan struct{})
	go p.run()
	return nil
}

func (p *program) run() {
	defer close(p.done)

	a, lk, err := newApp(p.ctx, p.flags, true, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s daemon: open store: %v\n", time.Now().Format(time.RFC3339), err)
		p.fail()
		return
	}
	defer closeApp(a, lk)

	if err := a.EnsureAuthed(); err != nil {
		fmt.Fprintf(os.Stderr, "%s daemon: not authenticated (run `wacli auth`): %v\n", time.Now().Format(time.RFC3339), err)
		p.fail()
		return
	}

	// Serve IPC once connected so other wacli commands can delegate to this
	// live process (e.g. `send`) while it holds the store lock + connection.
	var ipcServer *ipc.Server
	afterConnect := func(_ context.Context) error {
		srv, serr := ipc.NewServer(ipc.SocketPath(a.StoreDir()), a)
		if serr != nil {
			fmt.Fprintf(os.Stderr, "%s daemon: IPC server: %v\n", time.Now().Format(time.RFC3339), serr)
			return nil
		}
		registerIPCHandlers(srv)
		go srv.Serve(p.ctx)
		ipcServer = srv
		return nil
	}

	fmt.Fprintf(os.Stderr, "%s daemon: starting follow sync (store %s)\n", time.Now().Format(time.RFC3339), a.StoreDir())
	_, err = a.Sync(p.ctx, appPkg.SyncOptions{
		Mode:            appPkg.SyncModeFollow,
		AllowQR:         false,
		AfterConnect:    afterConnect,
		DownloadMedia:   p.dopts.downloadMedia,
		RefreshContacts: p.dopts.refreshContacts,
		RefreshGroups:   p.dopts.refreshGroups,
	})
	if ipcServer != nil {
		_ = ipcServer.Close()
	}
	if err != nil && p.ctx.Err() == nil {
		fmt.Fprintf(os.Stderr, "%s daemon: sync exited: %v\n", time.Now().Format(time.RFC3339), err)
		p.fail()
	}
}

// fail exits non-zero so the service manager restarts the process. The store
// lock and DB handles are reclaimed by the OS on exit.
func (p *program) fail() {
	os.Exit(1)
}

func (p *program) Stop(s service.Service) error {
	if p.cancel != nil {
		p.cancel()
	}
	if p.done != nil {
		select {
		case <-p.done:
		case <-time.After(20 * time.Second):
		}
	}
	return nil
}

func addDaemonSyncFlags(cmd *cobra.Command, dopts *daemonOpts) {
	cmd.Flags().BoolVar(&dopts.downloadMedia, "download-media", false, "download media during sync")
	cmd.Flags().BoolVar(&dopts.refreshContacts, "refresh-contacts", false, "refresh contacts on each start")
	cmd.Flags().BoolVar(&dopts.refreshGroups, "refresh-groups", false, "refresh groups on each start")
}

func newDaemonStartCmd(flags *rootFlags) *cobra.Command {
	var dopts daemonOpts
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Install (if needed) and start the sync service; auto-starts on boot",
		RunE: func(cmd *cobra.Command, args []string) error {
			prg := &program{flags: flags, dopts: dopts}
			s, storeDir, err := buildService(flags, dopts, prg)
			if err != nil {
				return err
			}

			// (Re)install so the unit always reflects the current binary path
			// and options, overwriting any stale same-named definition (e.g.
			// one left by a previous install at a different path).
			wasInstalled := true
			if _, statusErr := s.Status(); errors.Is(statusErr, service.ErrNotInstalled) {
				wasInstalled = false
			}
			if wasInstalled {
				_ = s.Stop()
				_ = s.Uninstall()
			}
			if err := s.Install(); err != nil {
				return fmt.Errorf("install service: %w", err)
			}
			if err := s.Start(); err != nil {
				return fmt.Errorf("start service: %w", err)
			}

			if flags.asJSON {
				return out.WriteJSON(os.Stdout, map[string]any{
					"service":     serviceName,
					"platform":    service.Platform(),
					"reinstalled": wasInstalled,
					"running":     true,
					"store":       storeDir,
					"log":         daemonLogPath(storeDir),
				})
			}
			verb := "installed and started"
			if wasInstalled {
				verb = "reinstalled and started"
			}
			fmt.Fprintf(os.Stdout,
				"Daemon %s as service %q (%s).\nIt will auto-start on boot and restart on crash.\nStore: %s\nLogs:  wacli daemon logs -f\n",
				verb, serviceName, service.Platform(), storeDir)
			return nil
		},
	}
	addDaemonSyncFlags(cmd, &dopts)
	return cmd
}

func newDaemonStopCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the sync service (stays installed; use `uninstall` to remove)",
		RunE: func(cmd *cobra.Command, args []string) error {
			prg := &program{flags: flags}
			s, _, err := buildService(flags, daemonOpts{}, prg)
			if err != nil {
				return err
			}
			if _, statusErr := s.Status(); errors.Is(statusErr, service.ErrNotInstalled) {
				if flags.asJSON {
					return out.WriteJSON(os.Stdout, map[string]any{"installed": false})
				}
				fmt.Fprintln(os.Stdout, "Service is not installed.")
				return nil
			}
			if err := s.Stop(); err != nil {
				return fmt.Errorf("stop service: %w", err)
			}
			if flags.asJSON {
				return out.WriteJSON(os.Stdout, map[string]any{"stopped": true, "service": serviceName})
			}
			fmt.Fprintf(os.Stdout, "Service %q stopped (still installed; run `wacli daemon uninstall` to remove from boot).\n", serviceName)
			return nil
		},
	}
}

func newDaemonRestartCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "restart",
		Short: "Restart the sync service",
		RunE: func(cmd *cobra.Command, args []string) error {
			prg := &program{flags: flags}
			s, _, err := buildService(flags, daemonOpts{}, prg)
			if err != nil {
				return err
			}
			if err := s.Restart(); err != nil {
				return fmt.Errorf("restart service: %w", err)
			}
			if flags.asJSON {
				return out.WriteJSON(os.Stdout, map[string]any{"restarted": true, "service": serviceName})
			}
			fmt.Fprintf(os.Stdout, "Service %q restarted.\n", serviceName)
			return nil
		},
	}
}

func newDaemonStatusCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check service status",
		RunE: func(cmd *cobra.Command, args []string) error {
			prg := &program{flags: flags}
			s, storeDir, err := buildService(flags, daemonOpts{}, prg)
			if err != nil {
				return err
			}
			st, statusErr := s.Status()

			installed := !errors.Is(statusErr, service.ErrNotInstalled)
			running := statusErr == nil && st == service.StatusRunning

			if flags.asJSON {
				return out.WriteJSON(os.Stdout, map[string]any{
					"service":   serviceName,
					"platform":  service.Platform(),
					"installed": installed,
					"running":   running,
					"store":     storeDir,
					"log":       daemonLogPath(storeDir),
				})
			}

			switch {
			case !installed:
				fmt.Fprintln(os.Stdout, "Service is not installed. Run `wacli daemon start`.")
			case running:
				fmt.Fprintf(os.Stdout, "Service %q is running (%s).\nStore: %s\nLog:   %s\n", serviceName, service.Platform(), storeDir, daemonLogPath(storeDir))
			case statusErr != nil:
				fmt.Fprintf(os.Stdout, "Service %q is installed; status unavailable on this platform (%v).\n", serviceName, statusErr)
			default:
				fmt.Fprintf(os.Stdout, "Service %q is installed but stopped. Run `wacli daemon start`.\n", serviceName)
			}
			return nil
		},
	}
}

func newDaemonUninstallCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall",
		Short: "Stop and remove the sync service from the OS",
		RunE: func(cmd *cobra.Command, args []string) error {
			prg := &program{flags: flags}
			s, _, err := buildService(flags, daemonOpts{}, prg)
			if err != nil {
				return err
			}
			if _, statusErr := s.Status(); errors.Is(statusErr, service.ErrNotInstalled) {
				if flags.asJSON {
					return out.WriteJSON(os.Stdout, map[string]any{"installed": false})
				}
				fmt.Fprintln(os.Stdout, "Service is not installed.")
				return nil
			}
			// Best-effort stop before removal.
			_ = s.Stop()
			if err := s.Uninstall(); err != nil {
				return fmt.Errorf("uninstall service: %w", err)
			}
			if flags.asJSON {
				return out.WriteJSON(os.Stdout, map[string]any{"uninstalled": true, "service": serviceName})
			}
			fmt.Fprintf(os.Stdout, "Service %q stopped and removed.\n", serviceName)
			return nil
		},
	}
}

// newDaemonRunCmd is the hidden entrypoint the OS service manager invokes. It
// runs the follow-sync loop in-process under kardianos control.
func newDaemonRunCmd(flags *rootFlags) *cobra.Command {
	var dopts daemonOpts
	cmd := &cobra.Command{
		Use:    "run",
		Short:  "Internal: run the sync service (invoked by the service manager)",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			storeDir := daemonStoreDir(flags)

			// Redirect process output to the daemon log so `daemon logs` works
			// uniformly across platforms. Must happen before newApp captures
			// os.Stderr for its event writer.
			if err := os.MkdirAll(storeDir, 0700); err == nil {
				if f, ferr := os.OpenFile(daemonLogPath(storeDir), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600); ferr == nil {
					fmt.Fprintf(f, "\n=== wacli daemon run at %s ===\n", time.Now().Format(time.RFC3339))
					os.Stdout = f
					os.Stderr = f
				}
			}

			prg := &program{flags: flags, dopts: dopts}
			s, _, err := buildService(flags, dopts, prg)
			if err != nil {
				return err
			}
			return s.Run()
		},
	}
	addDaemonSyncFlags(cmd, &dopts)
	return cmd
}

func newDaemonLogsCmd(flags *rootFlags) *cobra.Command {
	var follow bool
	var lines int

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Show daemon log output",
		RunE: func(cmd *cobra.Command, args []string) error {
			storeDir := daemonStoreDir(flags)
			logPath := daemonLogPath(storeDir)

			if _, err := os.Stat(logPath); os.IsNotExist(err) {
				return fmt.Errorf("no log file found at %s", logPath)
			}

			tailArgs := []string{"-n", strconv.Itoa(lines)}
			if follow {
				tailArgs = append(tailArgs, "-f")
			}
			tailArgs = append(tailArgs, logPath)

			tailCmd := exec.Command("tail", tailArgs...)
			tailCmd.Stdout = os.Stdout
			tailCmd.Stderr = os.Stderr
			return tailCmd.Run()
		},
	}

	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "follow log output (like tail -f)")
	cmd.Flags().IntVarP(&lines, "lines", "n", 50, "number of lines to show")
	return cmd
}

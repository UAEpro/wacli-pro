package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/steipete/wacli/internal/config"
	"github.com/steipete/wacli/internal/ipc"
	"github.com/steipete/wacli/internal/out"
)

func newDaemonCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Manage background sync daemon",
	}
	cmd.AddCommand(newDaemonStartCmd(flags))
	cmd.AddCommand(newDaemonStopCmd(flags))
	cmd.AddCommand(newDaemonStatusCmd(flags))
	cmd.AddCommand(newDaemonLogsCmd(flags))
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

func daemonPIDPath(storeDir string) string {
	return filepath.Join(storeDir, "daemon.pid")
}

func daemonLogPath(storeDir string) string {
	return filepath.Join(storeDir, "daemon.log")
}

func readDaemonPID(storeDir string) (int, error) {
	data, err := os.ReadFile(daemonPIDPath(storeDir))
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("invalid PID in daemon.pid: %w", err)
	}
	return pid, nil
}

func isDaemonRunning(storeDir string) (int, bool) {
	pid, err := readDaemonPID(storeDir)
	if err != nil || pid <= 0 {
		return 0, false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return pid, false
	}
	err = proc.Signal(syscall.Signal(0))
	return pid, err == nil
}

func newDaemonStartCmd(flags *rootFlags) *cobra.Command {
	var downloadMedia bool
	var refreshContacts bool
	var refreshGroups bool

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start background sync daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			storeDir := daemonStoreDir(flags)

			if pid, running := isDaemonRunning(storeDir); running {
				if flags.asJSON {
					return out.WriteJSON(os.Stdout, map[string]any{
						"running": true,
						"pid":     pid,
						"message": "daemon already running",
					})
				}
				return fmt.Errorf("daemon already running (pid %d)", pid)
			}

			if err := os.MkdirAll(storeDir, 0700); err != nil {
				return fmt.Errorf("create store dir: %w", err)
			}

			logPath := daemonLogPath(storeDir)
			logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
			if err != nil {
				return fmt.Errorf("open log file: %w", err)
			}

			// Build the command to run in the background.
			exe, err := os.Executable()
			if err != nil {
				_ = logFile.Close()
				return fmt.Errorf("resolve executable: %w", err)
			}

			cmdArgs := []string{"sync", "--follow", "--store", storeDir}
			if downloadMedia {
				cmdArgs = append(cmdArgs, "--download-media")
			}
			if refreshContacts {
				cmdArgs = append(cmdArgs, "--refresh-contacts")
			}
			if refreshGroups {
				cmdArgs = append(cmdArgs, "--refresh-groups")
			}

			daemonProc := exec.Command(exe, cmdArgs...)
			daemonProc.Stdout = logFile
			daemonProc.Stderr = logFile
			daemonProc.SysProcAttr = &syscall.SysProcAttr{
				Setsid: true, // Detach from terminal.
			}

			if err := daemonProc.Start(); err != nil {
				_ = logFile.Close()
				return fmt.Errorf("start daemon: %w", err)
			}

			pid := daemonProc.Process.Pid
			if err := os.WriteFile(daemonPIDPath(storeDir), []byte(strconv.Itoa(pid)), 0600); err != nil {
				// Kill the process if we can't write the PID file.
				_ = daemonProc.Process.Kill()
				_ = logFile.Close()
				return fmt.Errorf("write PID file: %w", err)
			}

			// Release the process so it doesn't become a zombie.
			_ = daemonProc.Process.Release()
			_ = logFile.Close()

			// Write startup marker to log.
			if f, err := os.OpenFile(logPath, os.O_WRONLY|os.O_APPEND, 0600); err == nil {
				fmt.Fprintf(f, "\n=== wacli daemon started at %s (pid %d) ===\n", time.Now().Format(time.RFC3339), pid)
				_ = f.Close()
			}

			if flags.asJSON {
				return out.WriteJSON(os.Stdout, map[string]any{
					"started": true,
					"pid":     pid,
					"log":     logPath,
				})
			}
			fmt.Fprintf(os.Stdout, "Daemon started (pid %d)\nLog: %s\n", pid, logPath)
			return nil
		},
	}

	cmd.Flags().BoolVar(&downloadMedia, "download-media", false, "download media during sync")
	cmd.Flags().BoolVar(&refreshContacts, "refresh-contacts", false, "refresh contacts on start")
	cmd.Flags().BoolVar(&refreshGroups, "refresh-groups", false, "refresh groups on start")
	return cmd
}

func newDaemonStopCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the background sync daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			storeDir := daemonStoreDir(flags)

			pid, running := isDaemonRunning(storeDir)
			if !running {
				// Clean up stale PID file.
				_ = os.Remove(daemonPIDPath(storeDir))
				if flags.asJSON {
					return out.WriteJSON(os.Stdout, map[string]any{"running": false})
				}
				fmt.Fprintln(os.Stdout, "Daemon is not running.")
				return nil
			}

			proc, err := os.FindProcess(pid)
			if err != nil {
				return fmt.Errorf("find process %d: %w", pid, err)
			}

			// Send SIGTERM for graceful shutdown.
			if err := proc.Signal(syscall.SIGTERM); err != nil {
				return fmt.Errorf("send SIGTERM to pid %d: %w", pid, err)
			}

			// Wait up to 10 seconds for the process to exit.
			stopped := false
			for i := 0; i < 20; i++ {
				time.Sleep(500 * time.Millisecond)
				if err := proc.Signal(syscall.Signal(0)); err != nil {
					stopped = true
					break
				}
			}

			if !stopped {
				// Force kill.
				_ = proc.Signal(syscall.SIGKILL)
				time.Sleep(500 * time.Millisecond)
			}

			_ = os.Remove(daemonPIDPath(storeDir))
			_ = os.Remove(ipc.SocketPath(storeDir))

			if flags.asJSON {
				return out.WriteJSON(os.Stdout, map[string]any{
					"stopped": true,
					"pid":     pid,
				})
			}
			fmt.Fprintf(os.Stdout, "Daemon stopped (pid %d)\n", pid)
			return nil
		},
	}
}

func newDaemonStatusCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check daemon status",
		RunE: func(cmd *cobra.Command, args []string) error {
			storeDir := daemonStoreDir(flags)
			pid, running := isDaemonRunning(storeDir)

			ipcReady := running && ipc.IsAvailable(storeDir)

			if flags.asJSON {
				return out.WriteJSON(os.Stdout, map[string]any{
					"running": running,
					"pid":     pid,
					"ipc":     ipcReady,
					"log":     daemonLogPath(storeDir),
				})
			}

			if running {
				ipcLabel := "no"
				if ipcReady {
					ipcLabel = "yes"
				}
				fmt.Fprintf(os.Stdout, "Daemon is running (pid %d, ipc %s)\nLog: %s\n", pid, ipcLabel, daemonLogPath(storeDir))
			} else {
				fmt.Fprintln(os.Stdout, "Daemon is not running.")
				// Clean up stale PID file.
				if pid > 0 {
					_ = os.Remove(daemonPIDPath(storeDir))
				}
			}
			return nil
		},
	}
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

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	appPkg "github.com/steipete/wacli/internal/app"
	"github.com/steipete/wacli/internal/ipc"
	"github.com/steipete/wacli/internal/out"
)

func newSyncCmd(flags *rootFlags) *cobra.Command {
	var once bool
	var follow bool
	var idleExit time.Duration
	var maxDuration time.Duration
	var downloadMedia bool
	var refreshContacts bool
	var refreshGroups bool

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync messages (requires prior auth; never shows QR)",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			// A second interrupt forces immediate exit.
			go func() {
				sigCh := make(chan os.Signal, 1)
				signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
				<-sigCh // first is handled by NotifyContext
				<-sigCh // second forces exit
				fmt.Fprintln(os.Stderr, "\nForced exit.")
				os.Exit(1)
			}()

			a, lk, err := newApp(ctx, flags, true, false)
			if err != nil {
				return err
			}
			defer closeApp(a, lk)

			if err := a.EnsureAuthed(); err != nil {
				return err
			}

			mode := appPkg.SyncModeFollow
			if once {
				mode = appPkg.SyncModeOnce
			} else if follow {
				mode = appPkg.SyncModeFollow
			} else {
				mode = appPkg.SyncModeOnce
			}

			// Start IPC server so other wacli commands can delegate to this
			// process while it holds the lock and WhatsApp connection.
			var ipcServer *ipc.Server
			var afterConnect func(context.Context) error
			if mode == appPkg.SyncModeFollow {
				afterConnect = func(_ context.Context) error {
					sockPath := ipc.SocketPath(a.StoreDir())
					s, err := ipc.NewServer(sockPath, a)
					if err != nil {
						fmt.Fprintf(os.Stderr, "warning: IPC server: %v\n", err)
						return nil
					}
					registerIPCHandlers(s)
					go s.Serve(ctx)
					ipcServer = s
					return nil
				}
			}

			res, err := a.Sync(ctx, appPkg.SyncOptions{
				Mode:            mode,
				AllowQR:         false,
				AfterConnect:    afterConnect,
				DownloadMedia:   downloadMedia,
				RefreshContacts: refreshContacts,
				RefreshGroups:   refreshGroups,
				IdleExit:        idleExit,
				MaxDuration:     maxDuration,
			})
			if ipcServer != nil {
				_ = ipcServer.Close()
			}
			if err != nil {
				return err
			}

			if flags.asJSON {
				return out.WriteJSON(os.Stdout, map[string]any{
					"synced":          true,
					"messages_stored": res.MessagesStored,
				})
			}
			fmt.Fprintf(os.Stdout, "Messages stored: %d\n", res.MessagesStored)
			return nil
		},
	}

	cmd.Flags().BoolVar(&once, "once", false, "sync until idle and exit")
	cmd.Flags().BoolVar(&follow, "follow", true, "keep syncing until Ctrl+C")
	cmd.Flags().DurationVar(&idleExit, "idle-exit", 30*time.Second, "exit after being idle (once mode)")
	cmd.Flags().DurationVar(&maxDuration, "max-duration", 0, "hard timeout for once mode (e.g. 5m); 0 = no limit")
	cmd.Flags().BoolVar(&downloadMedia, "download-media", false, "download media in the background during sync")
	cmd.Flags().BoolVar(&refreshContacts, "refresh-contacts", false, "refresh contacts from session store into local DB")
	cmd.Flags().BoolVar(&refreshGroups, "refresh-groups", false, "refresh joined groups (live) into local DB")
	return cmd
}

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/UAEpro/wacli-pro/internal/app"
	"github.com/UAEpro/wacli-pro/internal/out"
	"github.com/spf13/cobra"
)

func newHistoryCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history",
		Short: "History backfill (best-effort; requires prior auth)",
	}
	cmd.AddCommand(newHistoryBackfillCmd(flags))
	return cmd
}

func newHistoryBackfillCmd(flags *rootFlags) *cobra.Command {
	var chat string
	var count int
	var requests int
	var wait time.Duration
	var idleExit time.Duration

	cmd := &cobra.Command{
		Use:   "backfill",
		Short: "Request older messages for a chat from your primary device (on-demand history sync)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if chat == "" {
				return fmt.Errorf("--chat is required")
			}

			// Delegate to the running daemon (reusing its live connection)
			// when available; otherwise run a one-shot backfill directly.
			if data, err := tryDaemonCall(flags, "history.backfill", map[string]any{
				"chat":         chat,
				"count":        count,
				"requests":     requests,
				"wait_ms":      wait.Milliseconds(),
				"idle_exit_ms": idleExit.Milliseconds(),
			}); err != nil {
				return err
			} else if data != nil {
				return outputBackfill(flags, data)
			}

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			a, lk, err := newApp(ctx, flags, true, false)
			if err != nil {
				return err
			}
			defer closeApp(a, lk)

			res, err := a.BackfillHistory(ctx, app.BackfillOptions{
				ChatJID:        chat,
				Count:          count,
				Requests:       requests,
				WaitPerRequest: wait,
				IdleExit:       idleExit,
			})
			if err != nil {
				return err
			}
			return outputBackfill(flags, backfillResultMap(res))
		},
	}

	cmd.Flags().StringVar(&chat, "chat", "", "chat JID")
	cmd.Flags().IntVar(&count, "count", 50, "number of messages to request per on-demand sync (recommended: 50)")
	cmd.Flags().IntVar(&requests, "requests", 1, "number of on-demand requests to attempt")
	cmd.Flags().DurationVar(&wait, "wait", 60*time.Second, "time to wait for an on-demand response per request")
	cmd.Flags().DurationVar(&idleExit, "idle-exit", 5*time.Second, "exit after being idle (after backfill requests)")
	return cmd
}

// outputBackfill renders a backfill result as JSON or a human summary, working
// for both the direct path and the daemon (IPC, round-tripped) path.
func outputBackfill(flags *rootFlags, data map[string]any) error {
	if flags.asJSON {
		return out.WriteJSON(os.Stdout, data)
	}
	fmt.Fprintf(os.Stdout, "Backfill complete for %v. Added %v messages (%v requests).\n",
		data["chat"], data["messages_added"], data["requests_sent"])
	return nil
}

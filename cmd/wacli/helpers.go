package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/steipete/wacli/internal/app"
	"github.com/steipete/wacli/internal/config"
	"github.com/steipete/wacli/internal/ipc"
	"github.com/steipete/wacli/internal/lock"
	"github.com/steipete/wacli/internal/out"
	"github.com/steipete/wacli/internal/wa"
	"go.mau.fi/whatsmeow/types"
	"golang.org/x/term"
)

// warnOnErr prints a warning to stderr when a best-effort operation fails.
// Use this in CLI command paths where the primary action (e.g. sending a message)
// succeeded but a secondary operation (e.g. persisting to local DB) failed.
func warnOnErr(err error, op string) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: %s: %v\n", op, err)
	}
}

// prepareSend handles the common boilerplate for all send commands:
// create timeout context, init app, ensure authed, connect, parse recipient.
func prepareSend(flags *rootFlags, to string) (context.Context, context.CancelFunc, *app.App, *lock.Lock, types.JID, error) {
	ctx, cancel := withTimeout(context.Background(), flags)

	a, lk, err := newApp(ctx, flags, true, false)
	if err != nil {
		cancel()
		return nil, nil, nil, nil, types.JID{}, err
	}

	if err := a.EnsureAuthed(); err != nil {
		closeApp(a, lk)
		cancel()
		return nil, nil, nil, nil, types.JID{}, err
	}
	if err := a.Connect(ctx, false, nil); err != nil {
		closeApp(a, lk)
		cancel()
		return nil, nil, nil, nil, types.JID{}, err
	}

	toJID, err := wa.ParseUserOrJID(to)
	if err != nil {
		closeApp(a, lk)
		cancel()
		return nil, nil, nil, nil, types.JID{}, err
	}

	return ctx, cancel, a, lk, toJID, nil
}

func resolveStoreDir(flags *rootFlags) string {
	dir := flags.storeDir
	if dir == "" {
		dir = config.DefaultStoreDir()
	}
	dir, _ = filepath.Abs(dir)
	return dir
}

// tryDaemonCall attempts to delegate a command to the running daemon via IPC.
// Returns (nil, nil) if no daemon is available (caller should fall back to direct execution).
// Returns (data, nil) on success.
// Returns (nil, err) if the daemon returned an error.
func tryDaemonCall(flags *rootFlags, command string, params map[string]any) (map[string]any, error) {
	storeDir := resolveStoreDir(flags)
	if !ipc.IsAvailable(storeDir) {
		return nil, nil
	}

	ctx, cancel := withTimeout(context.Background(), flags)
	defer cancel()

	resp, err := ipc.Call(ctx, storeDir, ipc.Request{
		Command:   command,
		Params:    params,
		TimeoutMS: flags.timeout.Milliseconds(),
	})
	if err != nil {
		// Connection error — fall back to direct execution.
		return nil, nil
	}
	if !resp.Success {
		return nil, fmt.Errorf("%s", resp.Error)
	}

	data, ok := resp.Data.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected response from daemon")
	}
	return data, nil
}

// outputIPCResult writes the IPC result to stdout, either as JSON or as
// the provided human-readable text.
func outputIPCResult(flags *rootFlags, data map[string]any, humanText string) error {
	if flags.asJSON {
		return out.WriteJSON(os.Stdout, data)
	}
	fmt.Fprint(os.Stdout, humanText)
	return nil
}

// ipcFileName extracts the file name from an IPC result's "file" map.
func ipcFileName(data map[string]any) string {
	if fm, ok := data["file"].(map[string]any); ok {
		if n, ok := fm["name"].(string); ok {
			return n
		}
	}
	return "file"
}

func isTTY() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

func parseTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("time is required")
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("unsupported time format %q (use RFC3339 or YYYY-MM-DD)", s)
}

func sanitize(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
}

func truncate(s string, max int) string {
	s = sanitize(s)
	if max <= 0 || len(s) <= max {
		return s
	}
	if max <= 1 {
		return s[:max]
	}
	return s[:max-1] + "…"
}

// truncateForDisplay truncates strings for tabular output.
// When forceFull is true or stdout is not a TTY (piped), returns the full string.
func truncateForDisplay(s string, max int, forceFull bool) string {
	if forceFull || !isTTY() {
		return sanitize(s)
	}
	return truncate(s, max)
}

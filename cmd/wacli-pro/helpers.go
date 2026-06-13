package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/UAEpro/wacli-pro/internal/app"
	"github.com/UAEpro/wacli-pro/internal/config"
	"github.com/UAEpro/wacli-pro/internal/ipc"
	"github.com/UAEpro/wacli-pro/internal/lock"
	"github.com/UAEpro/wacli-pro/internal/out"
	"github.com/UAEpro/wacli-pro/internal/wa"
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

// runLiveOrDelegate runs a live-connection operation, delegating to the running
// daemon over IPC when one is available (so it works while the daemon holds the
// store lock), otherwise acquiring the lock and connecting directly. The op
// closure runs against a connected app and returns the same serializable result
// the daemon's IPC handler would return, so both paths produce identical output.
func runLiveOrDelegate(flags *rootFlags, command string, params map[string]any,
	op func(ctx context.Context, a *app.App) (map[string]any, error)) (map[string]any, error) {

	if data, err := tryDaemonCall(flags, command, params); err != nil {
		return nil, err
	} else if data != nil {
		return data, nil
	}

	ctx, cancel := withTimeout(context.Background(), flags)
	defer cancel()

	a, lk, err := newApp(ctx, flags, true, false)
	if err != nil {
		return nil, err
	}
	defer closeApp(a, lk)

	if err := a.EnsureAuthed(); err != nil {
		return nil, err
	}
	if err := a.Connect(ctx, false, nil); err != nil {
		return nil, err
	}
	return op(ctx, a)
}

// outputOK renders a simple action result: the JSON map with --json, or "OK".
func outputOK(flags *rootFlags, data map[string]any) error {
	if flags.asJSON {
		return out.WriteJSON(os.Stdout, data)
	}
	fmt.Fprintln(os.Stdout, "OK")
	return nil
}

// paramStringSlice extracts a []string from IPC params, tolerating the []any
// form produced by JSON decoding.
func paramStringSlice(params map[string]any, key string) []string {
	v, ok := params[key]
	if !ok {
		return nil
	}
	switch arr := v.(type) {
	case []string:
		return arr
	case []any:
		res := make([]string, 0, len(arr))
		for _, e := range arr {
			if s, ok := e.(string); ok {
				res = append(res, s)
			}
		}
		return res
	}
	return nil
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

// parseByteSize parses a human-friendly byte size string like "500MB", "1GB", "100KB".
func parseByteSize(s string) (int64, error) {
	s = strings.TrimSpace(strings.ToUpper(s))
	if s == "" || s == "0" {
		return 0, nil
	}
	multipliers := []struct {
		suffix string
		mult   int64
	}{
		{"TB", 1024 * 1024 * 1024 * 1024},
		{"GB", 1024 * 1024 * 1024},
		{"MB", 1024 * 1024},
		{"KB", 1024},
		{"B", 1},
	}
	for _, m := range multipliers {
		if strings.HasSuffix(s, m.suffix) {
			numStr := strings.TrimSuffix(s, m.suffix)
			numStr = strings.TrimSpace(numStr)
			var n int64
			if _, err := fmt.Sscanf(numStr, "%d", &n); err != nil {
				return 0, fmt.Errorf("cannot parse %q as byte size", s)
			}
			return n * m.mult, nil
		}
	}
	var n int64
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
		return 0, fmt.Errorf("cannot parse %q as byte size (use e.g. 500MB, 1GB)", s)
	}
	return n, nil
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

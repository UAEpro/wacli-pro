package ipc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"time"
)

// ErrDaemonUnavailable reports that the request never reached the daemon.
// Only this error makes it safe for callers to fall back to direct execution;
// any failure after the request was sent leaves the daemon possibly still
// executing the command, so retrying could duplicate a non-idempotent
// operation (e.g. a send).
var ErrDaemonUnavailable = errors.New("daemon unavailable")

// IsAvailable checks whether a daemon IPC socket is connectable.
func IsAvailable(storeDir string) bool {
	sockPath := SocketPath(storeDir)
	if _, err := os.Stat(sockPath); err != nil {
		return false
	}
	conn, err := net.DialTimeout("unix", sockPath, 500*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// Call sends a request to the daemon and returns the response.
func Call(ctx context.Context, storeDir string, req Request) (*Response, error) {
	sockPath := SocketPath(storeDir)

	var d net.Dialer
	conn, err := d.DialContext(ctx, "unix", sockPath)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDaemonUnavailable, err)
	}
	defer conn.Close()

	// Honor the caller's deadline/cancellation on the blocking socket I/O
	// below, so a hung daemon can't stall the CLI past its own timeout.
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	}
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}

	// Signal that we're done writing.
	if uc, ok := conn.(*net.UnixConn); ok {
		_ = uc.CloseWrite()
	}

	var resp Response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return &resp, nil
}

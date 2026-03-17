package ipc

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"
)

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
		return nil, fmt.Errorf("connect to daemon: %w", err)
	}
	defer conn.Close()

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

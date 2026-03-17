package ipc

import "path/filepath"

// Request is sent from a CLI command to the daemon over the Unix socket.
type Request struct {
	Command   string         `json:"command"`
	Params    map[string]any `json:"params"`
	TimeoutMS int64          `json:"timeout_ms,omitempty"`
}

// Response is sent from the daemon back to the CLI command.
type Response struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

// SocketPath returns the path to the daemon's Unix socket.
func SocketPath(storeDir string) string {
	return filepath.Join(storeDir, "daemon.sock")
}

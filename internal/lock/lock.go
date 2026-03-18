package lock

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type Lock struct {
	path string
	f    *os.File
}

func Acquire(storeDir string) (*Lock, error) {
	if err := os.MkdirAll(storeDir, 0700); err != nil {
		return nil, fmt.Errorf("create store dir: %w", err)
	}
	path := filepath.Join(storeDir, "LOCK")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_, _ = f.Seek(0, 0)
		b, _ := os.ReadFile(path)
		_ = f.Close()
		info := strings.TrimSpace(string(b))

		// Check if the process holding the lock is still alive (#88 stale lock).
		if pid := parseLockPID(info); pid > 0 && !processAlive(pid) {
			// Stale lock — the process is dead. Clean up and retry.
			_ = os.Remove(path)
			return Acquire(storeDir)
		}

		if info != "" {
			return nil, fmt.Errorf("store is locked (another wacli is running?): %w (%s)", err, info)
		}
		return nil, fmt.Errorf("store is locked (another wacli is running?): %w", err)
	}

	_ = f.Truncate(0)
	_, _ = f.Seek(0, 0)
	_, _ = fmt.Fprintf(f, "pid=%d\nacquired_at=%s\n", os.Getpid(), time.Now().Format(time.RFC3339Nano))
	_ = f.Sync()

	return &Lock{path: path, f: f}, nil
}

// parseLockPID extracts the PID from lock file content like "pid=12345\n...".
func parseLockPID(info string) int {
	for _, line := range strings.Split(info, "\n") {
		if strings.HasPrefix(line, "pid=") {
			if pid, err := strconv.Atoi(strings.TrimPrefix(line, "pid=")); err == nil {
				return pid
			}
		}
	}
	return 0
}

// processAlive checks if a process with the given PID is still running.
func processAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// Signal 0 doesn't send a signal but checks if the process exists.
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

func (l *Lock) Release() error {
	if l == nil || l.f == nil {
		return nil
	}
	_ = syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN)
	err := l.f.Close()
	l.f = nil
	return err
}

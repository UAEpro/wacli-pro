package ipc_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/steipete/wacli/internal/app"
	"github.com/steipete/wacli/internal/ipc"
)

func TestRoundtrip(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "daemon.sock")

	// Server with a nil App is fine here since our handler doesn't use it.
	s, err := ipc.NewServer(sockPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	s.Handle("echo", func(ctx context.Context, _ *app.App, params map[string]any) (any, error) {
		return params, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go s.Serve(ctx)

	// Give server time to start accepting.
	time.Sleep(50 * time.Millisecond)

	resp, err := ipc.Call(ctx, dir, ipc.Request{
		Command: "echo",
		Params:  map[string]any{"hello": "world"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !resp.Success {
		t.Fatalf("expected success, got error: %s", resp.Error)
	}

	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", resp.Data)
	}
	if data["hello"] != "world" {
		t.Fatalf("expected hello=world, got %v", data["hello"])
	}
}

func TestUnknownCommand(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "daemon.sock")

	s, err := ipc.NewServer(sockPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go s.Serve(ctx)
	time.Sleep(50 * time.Millisecond)

	resp, err := ipc.Call(ctx, dir, ipc.Request{
		Command: "nonexistent",
		Params:  map[string]any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Success {
		t.Fatal("expected failure for unknown command")
	}
	if resp.Error == "" {
		t.Fatal("expected error message")
	}
}

func TestIsAvailable(t *testing.T) {
	dir := t.TempDir()

	// No socket yet.
	if ipc.IsAvailable(dir) {
		t.Fatal("expected not available before server starts")
	}

	sockPath := filepath.Join(dir, "daemon.sock")
	s, err := ipc.NewServer(sockPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go s.Serve(ctx)
	time.Sleep(50 * time.Millisecond)

	if !ipc.IsAvailable(dir) {
		t.Fatal("expected available after server starts")
	}
}

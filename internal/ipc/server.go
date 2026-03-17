package ipc

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/steipete/wacli/internal/app"
)

// HandlerFunc processes an IPC request using the daemon's live App.
type HandlerFunc func(ctx context.Context, a *app.App, params map[string]any) (any, error)

// Server listens on a Unix socket and dispatches IPC requests to handlers.
type Server struct {
	listener net.Listener
	app      *app.App
	handlers map[string]HandlerFunc
	mu       sync.RWMutex
}

// NewServer creates a Unix socket server at sockPath.
func NewServer(sockPath string, a *app.App) (*Server, error) {
	_ = os.Remove(sockPath)

	listener, err := net.Listen("unix", sockPath)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", sockPath, err)
	}
	_ = os.Chmod(sockPath, 0600)

	return &Server{
		listener: listener,
		app:      a,
		handlers: make(map[string]HandlerFunc),
	}, nil
}

// Handle registers a handler for the given command name.
func (s *Server) Handle(command string, fn HandlerFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers[command] = fn
}

// Serve accepts connections until ctx is cancelled.
func (s *Server) Serve(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		_ = s.listener.Close()
	}()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			continue
		}
		go s.handleConn(ctx, conn)
	}
}

func (s *Server) handleConn(parentCtx context.Context, conn net.Conn) {
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(30 * time.Second))

	var req Request
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		writeResponse(conn, Response{Error: fmt.Sprintf("decode request: %v", err)})
		return
	}

	s.mu.RLock()
	handler, ok := s.handlers[req.Command]
	s.mu.RUnlock()

	if !ok {
		writeResponse(conn, Response{Error: fmt.Sprintf("unknown command: %s", req.Command)})
		return
	}

	ctx := parentCtx
	cancel := func() {}
	if req.TimeoutMS > 0 {
		ctx, cancel = context.WithTimeout(parentCtx, time.Duration(req.TimeoutMS)*time.Millisecond)
	}
	defer cancel()

	_ = conn.SetReadDeadline(time.Time{})
	_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Minute))

	data, err := handler(ctx, s.app, req.Params)
	if err != nil {
		writeResponse(conn, Response{Error: err.Error()})
		return
	}
	writeResponse(conn, Response{Success: true, Data: data})
}

func writeResponse(conn net.Conn, resp Response) {
	_ = json.NewEncoder(conn).Encode(resp)
}

// Close shuts down the listener and removes the socket file.
func (s *Server) Close() error {
	if s.listener != nil {
		sockPath := s.listener.Addr().String()
		err := s.listener.Close()
		_ = os.Remove(sockPath)
		return err
	}
	return nil
}

package kairos

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
)

// Server is an HTTP handler that exposes POST /kairos/run as a KAIROS worker endpoint.
type Server struct {
	handler   RunHandler
	authToken string
}

// NewServer creates a Server with the given handler and optional Bearer auth token.
// If authToken is empty, all requests are accepted without authentication.
func NewServer(handler RunHandler, authToken string) *Server {
	return &Server{handler: handler, authToken: authToken}
}

// Handler returns an http.Handler that serves /kairos/run and /kairos/healthz.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/kairos/run", s.handleRun)
	mux.HandleFunc("/kairos/healthz", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	})
	return mux
}

func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.authToken != "" {
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") || strings.TrimPrefix(auth, "Bearer ") != s.authToken {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}
	var req RunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if req.SubagentType == "" || req.Prompt == "" {
		http.Error(w, "missing subagent_type or prompt", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	var mu sync.Mutex
	emit := func(delta string) {
		mu.Lock()
		defer mu.Unlock()
		line, _ := json.Marshal(map[string]string{"text": delta})
		fmt.Fprintf(w, "event: text\ndata: %s\n\n", line)
		flusher.Flush()
	}

	finalText, err := s.handler(r.Context(), req, emit)
	if err != nil {
		mu.Lock()
		defer mu.Unlock()
		line, _ := json.Marshal(map[string]string{"error": err.Error()})
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", line)
		flusher.Flush()
		return
	}
	mu.Lock()
	defer mu.Unlock()
	line, _ := json.Marshal(map[string]string{"final": finalText})
	fmt.Fprintf(w, "event: done\ndata: %s\n\n", line)
	flusher.Flush()
}

// Run starts an HTTP server on addr and blocks until ctx is cancelled or the
// server fails. It shuts down gracefully when ctx is done.
func (s *Server) Run(ctx context.Context, addr string) error {
	srv := &http.Server{Addr: addr, Handler: s.Handler()}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	select {
	case <-ctx.Done():
		_ = srv.Shutdown(context.Background())
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

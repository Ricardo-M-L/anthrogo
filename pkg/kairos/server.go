package kairos

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/google/uuid"
)

// runState tracks an in-flight SSE run so tool-result POSTs can be routed to
// the correct waiting goroutine. It is created at the start of handleRun and
// removed when the SSE connection closes (client disconnect or handler return).
type runState struct {
	mu          sync.Mutex
	resultChans map[string]chan ToolResult // keyed by tool_use_id
}

// waitForResult blocks until either a result for toolUseID arrives or ctx is
// done. The channel is created lazily here so out-of-order POSTs (arriving
// before the goroutine calls wait) are handled correctly via a buffered chan.
func (rs *runState) waitForResult(ctx context.Context, toolUseID string) (ToolResult, error) {
	ch := make(chan ToolResult, 1)
	rs.mu.Lock()
	rs.resultChans[toolUseID] = ch
	rs.mu.Unlock()

	defer func() {
		rs.mu.Lock()
		delete(rs.resultChans, toolUseID)
		rs.mu.Unlock()
	}()

	select {
	case <-ctx.Done():
		return ToolResult{}, ctx.Err()
	case res := <-ch:
		return res, nil
	}
}

// deliver routes an incoming ToolResult to the waiting goroutine. Returns false
// if no goroutine is waiting for that tool_use_id (the POST arrived too early
// or the run already finished).
func (rs *runState) deliver(res ToolResult) bool {
	rs.mu.Lock()
	ch, ok := rs.resultChans[res.ToolUseID]
	rs.mu.Unlock()
	if !ok {
		return false
	}
	select {
	case ch <- res:
		return true
	default:
		// Already delivered (duplicate POST); drop.
		return false
	}
}

// Server is an HTTP handler that exposes POST /kairos/run as a KAIROS worker endpoint.
type Server struct {
	handler             RunHandler
	handlerWithForward  RunHandlerWithToolForward
	authToken           string
	mu                  sync.Mutex
	runs                map[string]*runState
}

// NewServer creates a Server with the given handler and optional Bearer auth token.
// If authToken is empty, all requests are accepted without authentication.
func NewServer(handler RunHandler, authToken string) *Server {
	return &Server{
		handler:   handler,
		authToken: authToken,
		runs:      make(map[string]*runState),
	}
}

// NewServerWithToolForward creates a Server that supports both plain RunHandler
// requests and exec-tools-locally requests (when the client sends
// X-Anthrogo-Exec-Tools-Locally: true). When that header is absent the plain
// handler is used.
func NewServerWithToolForward(handler RunHandler, handlerWithForward RunHandlerWithToolForward, authToken string) *Server {
	return &Server{
		handler:            handler,
		handlerWithForward: handlerWithForward,
		authToken:          authToken,
		runs:               make(map[string]*runState),
	}
}

// Handler returns an http.Handler that serves /kairos/run, /kairos/run/{rid}/tool-result, and /kairos/healthz.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/kairos/run", s.handleRun)
	mux.HandleFunc("/kairos/run/", s.handleToolResult) // catches /kairos/run/<rid>/tool-result
	mux.HandleFunc("/kairos/healthz", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ok")
	})
	return mux
}

func (s *Server) checkAuth(r *http.Request) bool {
	if s.authToken == "" {
		return true
	}
	auth := r.Header.Get("Authorization")
	return strings.HasPrefix(auth, "Bearer ") && strings.TrimPrefix(auth, "Bearer ") == s.authToken
}

func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.checkAuth(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
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

	execLocally := r.Header.Get("X-Anthrogo-Exec-Tools-Locally") == "true"

	// Emit helper: all SSE writes go through a mutex to ensure interleaving safety.
	var writeMu sync.Mutex
	writeEvent := func(event, dataJSON string) {
		writeMu.Lock()
		defer writeMu.Unlock()
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, dataJSON)
		flusher.Flush()
	}

	if execLocally && s.handlerWithForward != nil {
		// Mint a request-id and register run state so tool-result POSTs can be
		// routed here.
		rid := uuid.New().String()
		rs := &runState{resultChans: make(map[string]chan ToolResult)}
		s.mu.Lock()
		s.runs[rid] = rs
		s.mu.Unlock()
		defer func() {
			s.mu.Lock()
			delete(s.runs, rid)
			s.mu.Unlock()
		}()

		// Announce the request-id to the client so it knows where to POST results.
		ridJSON, _ := json.Marshal(map[string]string{"run_id": rid})
		writeEvent("run_id", string(ridJSON))

		emitText := func(delta string) {
			line, _ := json.Marshal(map[string]string{"text": delta})
			writeEvent("text", string(line))
		}
		emitToolUse := func(req ToolUseRequest) {
			line, _ := json.Marshal(req)
			writeEvent("tool_use_request", string(line))
		}
		waitForResult := func(toolUseID string) (ToolResult, error) {
			return rs.waitForResult(r.Context(), toolUseID)
		}

		finalText, err := s.handlerWithForward(r.Context(), req, emitText, emitToolUse, waitForResult)
		if err != nil {
			line, _ := json.Marshal(map[string]string{"error": err.Error()})
			writeEvent("error", string(line))
			return
		}
		line, _ := json.Marshal(map[string]string{"final": finalText})
		writeEvent("done", string(line))
		return
	}

	// Plain handler path (no exec-tools-locally).
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

// handleToolResult handles POST /kairos/run/<rid>/tool-result.
// It routes the ToolResult to the waiting goroutine in the matching runState.
func (s *Server) handleToolResult(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.checkAuth(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Extract rid from path: /kairos/run/<rid>/tool-result
	path := strings.TrimPrefix(r.URL.Path, "/kairos/run/")
	path = strings.TrimSuffix(path, "/tool-result")
	rid := path
	if rid == "" || strings.Contains(rid, "/") {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	rs, ok := s.runs[rid]
	s.mu.Unlock()
	if !ok {
		http.Error(w, "run not found", http.StatusNotFound)
		return
	}

	var res ToolResult
	if err := json.NewDecoder(r.Body).Decode(&res); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}

	if !rs.deliver(res) {
		// No waiting goroutine; this can happen if the run finished or the
		// tool_use_id is unknown. Return 202 Accepted rather than 404 to avoid
		// client retry loops.
		w.WriteHeader(http.StatusAccepted)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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

package kairos

import (
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/google/uuid"
	"golang.org/x/crypto/acme/autocert"
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
	handler            RunHandler
	handlerWithForward RunHandlerWithToolForward
	authToken          string
	signingKey         ed25519.PrivateKey // optional; when set, every SSE frame is wrapped in SignedFrame
	mu                 sync.Mutex
	runs               map[string]*runState
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

// NewServerWithSigning creates a Server with a signing key. Every SSE event payload
// is wrapped in a SignedFrame (signed with the ed25519 private key) before transmission.
// Clients must set ClientOptions.TrustKey to the corresponding public key to verify frames.
func NewServerWithSigning(handler RunHandler, authToken string, signingKey ed25519.PrivateKey) *Server {
	s := NewServer(handler, authToken)
	s.signingKey = signingKey
	return s
}

// SetHandlerWithForward sets the exec-tools-locally handler on an existing Server.
// This is useful when a Server was constructed via NewServerWithSigning and also
// needs to support the exec-tools-locally protocol.
func (s *Server) SetHandlerWithForward(h RunHandlerWithToolForward) {
	s.handlerWithForward = h
}

// writeEvent writes a single SSE event to w and flushes. When s.signingKey is
// set the payload is wrapped in a SignedFrame before transmission.
func (s *Server) writeEvent(w http.ResponseWriter, flusher http.Flusher, event string, payload any) {
	raw, _ := json.Marshal(payload)
	if s.signingKey != nil {
		frame := SignFrame(s.signingKey, raw)
		raw, _ = json.Marshal(frame)
	}
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, raw)
	flusher.Flush()
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
	emitEvent := func(event string, payload any) {
		writeMu.Lock()
		defer writeMu.Unlock()
		s.writeEvent(w, flusher, event, payload)
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
		emitEvent("run_id", map[string]string{"run_id": rid})

		emitText := func(delta string) {
			emitEvent("text", map[string]string{"text": delta})
		}
		emitToolUse := func(req ToolUseRequest) {
			emitEvent("tool_use_request", req)
		}
		waitForResult := func(toolUseID string) (ToolResult, error) {
			return rs.waitForResult(r.Context(), toolUseID)
		}

		finalText, err := s.handlerWithForward(r.Context(), req, emitText, emitToolUse, waitForResult)
		if err != nil {
			emitEvent("error", map[string]string{"error": err.Error()})
			return
		}
		emitEvent("done", map[string]string{"final": finalText})
		return
	}

	// Plain handler path (no exec-tools-locally).
	emit := func(delta string) {
		emitEvent("text", map[string]string{"text": delta})
	}

	finalText, err := s.handler(r.Context(), req, emit)
	if err != nil {
		emitEvent("error", map[string]string{"error": err.Error()})
		return
	}
	emitEvent("done", map[string]string{"final": finalText})
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

// RunTLS starts an HTTPS server on addr using the supplied PEM cert and key
// files. It blocks until ctx is cancelled or the server fails, and shuts down
// gracefully when ctx is done.
func (s *Server) RunTLS(ctx context.Context, addr, certFile, keyFile string) error {
	srv := &http.Server{Addr: addr, Handler: s.Handler()}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServeTLS(certFile, keyFile) }()
	select {
	case <-ctx.Done():
		_ = srv.Shutdown(context.Background())
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

// RunAutocert starts an HTTPS server on addr using Let's Encrypt auto-provisioned
// certificates for the given domains. Certificates are cached under cacheDir.
// Port 443 must be reachable from the internet for the ACME HTTP-01 challenge.
// It blocks until ctx is cancelled or the server fails, and shuts down gracefully
// when ctx is done.
func (s *Server) RunAutocert(ctx context.Context, addr string, domains []string, cacheDir string) error {
	m := &autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(domains...),
		Cache:      autocert.DirCache(cacheDir),
	}
	srv := &http.Server{
		Addr:      addr,
		Handler:   s.Handler(),
		TLSConfig: m.TLSConfig(),
	}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServeTLS("", "") }()
	select {
	case <-ctx.Done():
		_ = srv.Shutdown(context.Background())
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

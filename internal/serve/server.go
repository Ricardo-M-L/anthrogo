// Package serve implements the anthrogo HTTP daemon (anthrogo serve).
package serve

import (
	"context"
	"crypto/subtle"
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"time"

	"github.com/ricardo/anthrogo/pkg/permissions"
	"github.com/ricardo/anthrogo/pkg/provider"
	"github.com/ricardo/anthrogo/pkg/query"
	"github.com/ricardo/anthrogo/pkg/tool"
)

// ProviderFactory is a function that constructs a provider and returns the
// effective model name. The indirection exists so tests can inject a fake.
type ProviderFactory func() (provider.Provider, string, error)

// Config holds runtime options for the HTTP server.
type Config struct {
	Addr        string
	Token       string // optional bearer auth token
	CORSOrigin  string // optional Access-Control-Allow-Origin value
	SessionsDir string // override sessions storage root; "" = default

	// ProviderFactory constructs the provider used for all chat requests.
	ProviderFactory ProviderFactory

	// Tools is the shared tool registry.
	Tools *tool.Registry

	// Permissions is the shared permissions context.
	Permissions *permissions.Context

	// Model is the default model name (may be overridden per-request in future).
	Model string

	// SystemPrompt is the system prompt used for every engine.
	SystemPrompt string
}

// Server is the HTTP server for the anthrogo serve daemon.
type Server struct {
	cfg         Config
	cache       *sessionCache
	inFlight    atomic.Int64
	startedAt   time.Time
	mux         *http.ServeMux
	rootHandler http.Handler // optional handler mounted at "/"
}

// New constructs a Server from cfg.
func New(cfg Config) *Server {
	s := &Server{
		cfg:       cfg,
		cache:     newSessionCache(),
		startedAt: time.Now(),
	}
	s.mux = s.buildMux()
	return s
}

// WithRoot sets an additional http.Handler that is mounted at "/" for all paths
// not matched by the /v1/* API routes. This allows embedding a SPA alongside
// the API without path conflicts. It rebuilds the internal mux.
func (s *Server) WithRoot(h http.Handler) *Server {
	s.rootHandler = h
	s.mux = s.buildMux()
	return s
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// ListenAndServe starts the HTTP server on cfg.Addr and blocks until ctx is
// cancelled or the listener fails.
func (s *Server) ListenAndServe(ctx context.Context) error {
	srv := &http.Server{
		Addr:    s.cfg.Addr,
		Handler: s,
	}
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutCtx)
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

// buildMux registers all routes with the middleware chain.
func (s *Server) buildMux() *http.ServeMux {
	mux := http.NewServeMux()

	wrap := func(h http.HandlerFunc) http.Handler {
		return s.withRecovery(s.withLogging(s.withCORS(s.withAuth(h))))
	}

	mux.Handle("POST /v1/chat", wrap(s.handleChat))
	mux.Handle("GET /v1/sessions", wrap(s.handleListSessions))
	mux.Handle("GET /v1/sessions/{id}", wrap(s.handleGetSession))
	mux.Handle("DELETE /v1/sessions/{id}", wrap(s.handleDeleteSession))
	mux.Handle("GET /v1/tools", wrap(s.handleListTools))
	mux.Handle("GET /v1/health", wrap(s.handleHealth))

	// Handle CORS pre-flight for all routes.
	mux.Handle("OPTIONS /", wrap(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	// Mount optional root handler (e.g. embedded SPA) at "/".
	// It is registered after the /v1/* routes so the more-specific patterns win.
	if s.rootHandler != nil {
		root := s.rootHandler
		mux.Handle("/", s.withRecovery(s.withLogging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			root.ServeHTTP(w, r)
		}))))
	}

	return mux
}

// ---- middleware ----

func (s *Server) withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.Token == "" {
			next(w, r)
			return
		}
		auth := r.Header.Get("Authorization")
		expected := "Bearer " + s.cfg.Token
		if subtle.ConstantTimeCompare([]byte(auth), []byte(expected)) != 1 {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func (s *Server) withCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.CORSOrigin != "" {
			w.Header().Set("Access-Control-Allow-Origin", s.cfg.CORSOrigin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		}
		next(w, r)
	}
}

func (s *Server) withLogging(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &statusWriter{ResponseWriter: w, code: http.StatusOK}
		next(rw, r)
		log.Printf("[serve] %s %s %d %s", r.Method, r.URL.Path, rw.code, time.Since(start))
	}
}

func (s *Server) withRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("[serve] panic: %v\n%s", rec, debug.Stack())
				http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// statusWriter captures the written HTTP status code.
type statusWriter struct {
	http.ResponseWriter
	code int
}

func (sw *statusWriter) WriteHeader(code int) {
	sw.code = code
	sw.ResponseWriter.WriteHeader(code)
}

// ---- engine construction ----

// getOrCreateEngine returns a cached engine for sessionID, creating a new one
// (and a new session file) if none exists. Uses GetOrCreate to prevent TOCTOU
// race where two concurrent requests for the same session both create engines.
func (s *Server) getOrCreateEngine(sessionID string) (*query.Engine, error) {
	return s.cache.GetOrCreate(sessionID, func() (*query.Engine, error) {
		prov, model, err := s.cfg.ProviderFactory()
		if err != nil {
			return nil, fmt.Errorf("serve: build provider: %w", err)
		}

		effectiveModel := s.cfg.Model
		if model != "" {
			effectiveModel = model
		}

		// Determine cwd for session — use server's working directory.
		cwd := s.cfg.SessionsDir
		if cwd == "" {
			cwd = "."
		}

		perms := s.cfg.Permissions
		if perms == nil {
			// Safe default: gate stays at ModeDefault so the engine asks
			// before destructive tools. ShouldAvoidPrompts=true because
			// there is no interactive TUI to receive prompts — under that
			// flag, the gate treats Ask as Deny rather than blocking.
			// Callers that want bypass must opt in explicitly via
			// ServerConfig.Permissions.
			perms = &permissions.Context{
				Mode:               permissions.ModeDefault,
				ShouldAvoidPrompts: true,
			}
		}

		// Build the engine with no record hook (sessions not persisted by the
		// serve daemon by default — purely in-memory for now).
		e := query.NewEngine(query.Config{
			Provider:     prov,
			Tools:        s.cfg.Tools,
			Permissions:  perms,
			Model:        effectiveModel,
			SystemPrompt: s.cfg.SystemPrompt,
			Cwd:          cwd,
		})

		// Seed existing messages if the session was previously persisted.
		// (Future: we could load from JSONL here; skipped for initial version.)
		return e, nil
	})
}

// pathSegment extracts the last path segment from r.URL.Path (used for {id}).
func pathSegment(r *http.Request, key string) string {
	// Go 1.22+ ServeMux wildcard: PathValue(key).
	// Fallback: trim the prefix manually.
	if v := r.PathValue(key); v != "" {
		return v
	}
	// Fallback for older Go — shouldn't be needed at go 1.26 but kept safe.
	parts := strings.Split(strings.TrimRight(r.URL.Path, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

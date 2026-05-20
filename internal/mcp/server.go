package mcp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// State is the public lifecycle phase of one MCP server.
type State string

const (
	StateInit   State = "init"
	StateReady  State = "ready"
	StateFailed State = "failed"
	StateClosed State = "closed"
)

// Server wraps one MCP stdio subprocess + its SDK ClientSession.
type Server struct {
	mu sync.RWMutex

	Name string
	cfg  MCPServerConfig

	cmd     *exec.Cmd
	session *sdk.ClientSession
	tools   []*sdk.Tool

	state State
	err   error

	notifyLog func(serverName, msg string) // nil-safe
}

// NewServer constructs (but does not start) a Server.
func NewServer(name string, cfg MCPServerConfig, notifyLog func(string, string)) *Server {
	if cfg.Timeout == 0 {
		cfg.Timeout = DefaultInitTimeout
	}
	return &Server{
		Name:      name,
		cfg:       cfg,
		state:     StateInit,
		notifyLog: notifyLog,
	}
}

// Start spawns or connects the server, performs the initialize handshake, and
// calls tools/list. State is StateReady on success or StateFailed on any
// error. It may be called again after Close to reload.
func (s *Server) Start(parent context.Context) error {
	startCtx, cancel := context.WithTimeout(parent, s.cfg.Timeout)
	defer cancel()

	// Reset state so a reloaded server doesn't inherit Closed/Failed.
	s.mu.Lock()
	s.state = StateInit
	s.err = nil
	s.tools = nil
	s.mu.Unlock()

	// Build the client with a logging handler that fans out to s.notifyLog.
	impl := &sdk.Implementation{Name: "anthrogo", Version: "0.3.0-dev"}
	opts := &sdk.ClientOptions{
		LoggingMessageHandler: func(_ context.Context, p *sdk.LoggingMessageRequest) {
			if s.notifyLog == nil {
				return
			}
			s.notifyLog(s.Name, formatLogData(p.Params.Data))
		},
	}
	client := sdk.NewClient(impl, opts)

	// Pick transport based on Type.
	var transport sdk.Transport
	switch strings.ToLower(s.cfg.Type) {
	case "", "stdio":
		if s.cfg.Command == "" {
			s.fail(fmt.Errorf("stdio MCP server %s requires command", s.Name))
			return s.err
		}
		cmd := exec.CommandContext(parent, s.cfg.Command, s.cfg.Args...)
		cmd.Dir = s.cfg.Cwd
		cmd.Env = mergeEnv(s.cfg.Env)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		s.mu.Lock()
		s.cmd = cmd
		s.mu.Unlock()
		transport = &sdk.CommandTransport{
			Command:           cmd,
			TerminateDuration: 2 * time.Second,
		}
	case "sse":
		if s.cfg.Endpoint == "" {
			s.fail(fmt.Errorf("sse MCP server %s requires endpoint", s.Name))
			return s.err
		}
		transport = &sdk.SSEClientTransport{Endpoint: s.cfg.Endpoint}
	case "streamable":
		if s.cfg.Endpoint == "" {
			s.fail(fmt.Errorf("streamable MCP server %s requires endpoint", s.Name))
			return s.err
		}
		transport = &sdk.StreamableClientTransport{
			Endpoint:   s.cfg.Endpoint,
			MaxRetries: s.cfg.MaxRetries,
		}
	default:
		s.fail(fmt.Errorf("unknown MCP server type %q for %s", s.cfg.Type, s.Name))
		return s.err
	}

	session, err := client.Connect(startCtx, transport, nil)
	if err != nil {
		s.fail(fmt.Errorf("connect: %w", err))
		return s.err
	}
	s.mu.Lock()
	s.session = session
	s.mu.Unlock()

	listRes, err := session.ListTools(startCtx, &sdk.ListToolsParams{})
	if err != nil {
		s.fail(fmt.Errorf("tools/list: %w", err))
		_ = session.Close()
		return s.err
	}

	s.mu.Lock()
	s.tools = listRes.Tools
	s.state = StateReady
	s.mu.Unlock()
	return nil
}

func (s *Server) fail(err error) {
	s.mu.Lock()
	s.state = StateFailed
	s.err = err
	s.mu.Unlock()
}

// State returns the current state. Concurrent-safe.
func (s *Server) State() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// Err returns the last error if state is Failed.
func (s *Server) Err() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.err
}

// Tools returns a copy of the most recent tools/list snapshot.
func (s *Server) Tools() []*sdk.Tool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*sdk.Tool, len(s.tools))
	copy(out, s.tools)
	return out
}

// CallTool invokes a tool on this server by name.
func (s *Server) CallTool(ctx context.Context, name string, args map[string]any) (*sdk.CallToolResult, error) {
	s.mu.RLock()
	sess := s.session
	state := s.state
	s.mu.RUnlock()
	if state != StateReady {
		return nil, fmt.Errorf("mcp server %s is not ready (state=%s)", s.Name, state)
	}
	if sess == nil {
		return nil, errors.New("mcp session not established")
	}
	return sess.CallTool(ctx, &sdk.CallToolParams{Name: name, Arguments: args})
}

// Close gracefully terminates the server. The SDK's CommandTransport owns
// the cmd lifecycle (SIGTERM → wait → SIGKILL) via TerminateDuration, so we
// only need to call sess.Close().
func (s *Server) Close() error {
	s.mu.Lock()
	if s.state == StateClosed {
		s.mu.Unlock()
		return nil
	}
	sess := s.session
	s.state = StateClosed
	s.mu.Unlock()

	if sess != nil {
		_ = sess.Close() // owns cmd shutdown via CommandTransport's TerminateDuration
	}
	return nil
}

// formatLogData coerces a logging-message payload to a string.
func formatLogData(data any) string {
	switch d := data.(type) {
	case string:
		return d
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", d)
	}
}

// mergeEnv merges the process env with overrides.
func mergeEnv(overrides map[string]string) []string {
	out := append([]string(nil), os.Environ()...)
	for k, v := range overrides {
		out = append(out, k+"="+v)
	}
	return out
}

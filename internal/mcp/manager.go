package mcp

import (
	"context"
	"fmt"
	"sort"
	"sync"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/ricardo/anthrogo/pkg/tool"
)

// Manager owns N MCP Servers and exposes their tools through tool.Tool
// adapters. Construct with NewManager; call Start before AllTools.
type Manager struct {
	mu       sync.RWMutex
	servers  map[string]*Server
	logSink  LogSink
	elicitFn ElicitFn // M6.3: set via SetElicitationHandler
}

// NewManager constructs a Manager with an optional LogSink.
func NewManager(logSink LogSink) *Manager {
	return &Manager{
		servers: map[string]*Server{},
		logSink: logSink,
	}
}

// AddServer registers a server config. Server is not started until Start().
func (m *Manager) AddServer(name string, cfg MCPServerConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.servers[name] = NewServer(name, cfg, m.logSink, m.elicitFn)
}

// SetElicitationHandler installs fn as the elicitation callback for all
// current and future servers. Call before Start for best effect; any already-
// started servers update their elicitFn immediately (affects next elicitation).
func (m *Manager) SetElicitationHandler(fn ElicitFn) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.elicitFn = fn
	for _, s := range m.servers {
		s.elicitFn = fn
	}
}

// Start spawns every server in parallel and waits for all of them to settle
// (ready or failed). Returns nil even if some servers failed — inspect with
// State(name).
func (m *Manager) Start(ctx context.Context) error {
	m.mu.RLock()
	servers := make([]*Server, 0, len(m.servers))
	for _, s := range m.servers {
		servers = append(servers, s)
	}
	m.mu.RUnlock()

	var wg sync.WaitGroup
	for _, s := range servers {
		wg.Add(1)
		go func(srv *Server) {
			defer wg.Done()
			_ = srv.Start(ctx)
		}(s)
	}
	wg.Wait()
	return nil
}

// Names returns server names in deterministic (sorted) order.
func (m *Manager) Names() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]string, 0, len(m.servers))
	for n := range m.servers {
		out = append(out, n)
	}
	// sort for stable tool ordering across runs (matters for prompt caching).
	sort.Strings(out)
	return out
}

// State returns the state of one server, or "" if no such name.
func (m *Manager) State(name string) State {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.servers[name]
	if !ok {
		return ""
	}
	return s.State()
}

// Err returns the last error for one server.
func (m *Manager) Err(name string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.servers[name]
	if !ok {
		return nil
	}
	return s.Err()
}

// AllTools returns one MCPAdapter per (ready server, advertised tool).
func (m *Manager) AllTools() []tool.Tool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []tool.Tool
	for _, name := range m.namesLocked() {
		s := m.servers[name]
		if s.State() != StateReady {
			continue
		}
		for _, t := range s.Tools() {
			descriptor := t
			srv := s
			invoker := func(ctx context.Context, args map[string]any) (*sdk.CallToolResult, error) {
				return srv.CallTool(ctx, descriptor.Name, args)
			}
			out = append(out, tool.NewMCPAdapter(name, descriptor, invoker))
		}
	}
	return out
}

// AllResources lists resources from every Ready server. Per-server errors are
// logged via LogSink and omitted from the result map.
func (m *Manager) AllResources(ctx context.Context) map[string][]*sdk.Resource {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := map[string][]*sdk.Resource{}
	for _, name := range m.namesLocked() {
		srv := m.servers[name]
		if srv.State() != StateReady {
			continue
		}
		rs, err := srv.ListResources(ctx)
		if err != nil {
			if m.logSink != nil {
				m.logSink(name, fmt.Sprintf("list resources failed: %v", err))
			}
			continue
		}
		out[name] = rs
	}
	return out
}

// ReadResource reads a resource by URI from a named server.
func (m *Manager) ReadResource(ctx context.Context, server, uri string) (*sdk.ReadResourceResult, error) {
	m.mu.RLock()
	srv, ok := m.servers[server]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown MCP server %s", server)
	}
	return srv.ReadResource(ctx, uri)
}

func (m *Manager) namesLocked() []string {
	out := make([]string, 0, len(m.servers))
	for n := range m.servers {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// ServerConfig returns the MCPServerConfig for the named server, and whether
// the server exists. Used by status display to show (redacted) headers.
func (m *Manager) ServerConfig(name string) (MCPServerConfig, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.servers[name]
	if !ok {
		return MCPServerConfig{}, false
	}
	return s.cfg, true
}

// Close terminates every server.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.servers {
		_ = s.Close()
	}
	return nil
}

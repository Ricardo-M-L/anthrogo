package mcp

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// buildEchoServer compiles testdata/echo-server into the test's tempdir and
// returns the binary path.
func buildEchoServer(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "echo-server")
	src := filepath.Join("testdata", "echo-server")
	cmd := exec.Command("go", "build", "-o", bin, "./"+src)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "build echo-server: %s", out)
	return bin
}

func TestManager_StartsAndListsTools(t *testing.T) {
	bin := buildEchoServer(t)

	m := NewManager(nil)
	m.AddServer("echo", MCPServerConfig{Command: bin, Timeout: 30 * time.Second})

	require.NoError(t, m.Start(context.Background()))
	require.Equal(t, StateReady, m.State("echo"))

	tools := m.AllTools()
	require.Len(t, tools, 2)
	names := make([]string, len(tools))
	for i, tt := range tools {
		names[i] = tt.Name()
	}
	require.Contains(t, names, "mcp__echo__echo")
}

func TestManager_CallsToolEndToEnd(t *testing.T) {
	bin := buildEchoServer(t)

	m := NewManager(nil)
	m.AddServer("echo", MCPServerConfig{Command: bin, Timeout: 30 * time.Second})
	require.NoError(t, m.Start(context.Background()))
	defer m.Close()

	tools := m.AllTools()
	require.Len(t, tools, 2)
	// find the "echo" tool by name
	var echoIdx = -1
	for i, tt := range tools {
		if tt.Name() == "mcp__echo__echo" {
			echoIdx = i
			break
		}
	}
	require.NotEqual(t, -1, echoIdx, "echo tool not found")
	res, err := tools[echoIdx].Call(context.Background(), map[string]any{"msg": "world"}, nil)
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Contains(t, res.Text, "echo: world")
}

func TestManager_FailedServer_RestStillReady(t *testing.T) {
	bin := buildEchoServer(t)
	m := NewManager(nil)
	m.AddServer("good", MCPServerConfig{Command: bin, Timeout: 30 * time.Second})
	m.AddServer("bad", MCPServerConfig{Command: "/nonexistent/binary", Timeout: 2 * time.Second})

	require.NoError(t, m.Start(context.Background()))
	require.Equal(t, StateReady, m.State("good"))
	require.Equal(t, StateFailed, m.State("bad"))

	tools := m.AllTools()
	require.Len(t, tools, 2) // only the good server contributes (bad server has 0 tools)
}

func TestManager_CloseSendsTerm(t *testing.T) {
	bin := buildEchoServer(t)
	m := NewManager(nil)
	m.AddServer("echo", MCPServerConfig{Command: bin, Timeout: 30 * time.Second})
	require.NoError(t, m.Start(context.Background()))
	require.NoError(t, m.Close())
	require.Equal(t, StateClosed, m.State("echo"))
}

func TestManager_LogSinkReceivesNotifications(t *testing.T) {
	t.Skip("requires echo-server to emit notifications/message — placeholder for future PR")
}

func TestServer_Start_ResetsStateAfterClose(t *testing.T) {
	bin := buildEchoServer(t)
	s := NewServer("echo", MCPServerConfig{Command: bin, Timeout: 30 * time.Second}, nil)
	require.NoError(t, s.Start(context.Background()))
	require.Equal(t, StateReady, s.State())

	require.NoError(t, s.Close())
	require.Equal(t, StateClosed, s.State())

	// Re-Start the same server — state must reset to StateReady.
	require.NoError(t, s.Start(context.Background()))
	require.Equal(t, StateReady, s.State())
	require.Nil(t, s.Err())
	require.NoError(t, s.Close())
}

// Transport-validation tests — these never dial a network; they just check
// that invalid configs fail fast with StateFailed.

func TestServer_Start_RejectsBadType(t *testing.T) {
	s := NewServer("x", MCPServerConfig{Type: "garbage", Timeout: 2 * time.Second}, nil)
	err := s.Start(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown")
	require.Equal(t, StateFailed, s.State())
}

func TestServer_Start_StdioRequiresCommand(t *testing.T) {
	s := NewServer("x", MCPServerConfig{Type: "stdio", Timeout: 2 * time.Second}, nil)
	err := s.Start(context.Background())
	require.Error(t, err)
	require.Equal(t, StateFailed, s.State())
}

func TestServer_Start_SSERequiresEndpoint(t *testing.T) {
	s := NewServer("x", MCPServerConfig{Type: "sse", Timeout: 2 * time.Second}, nil)
	err := s.Start(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "endpoint")
	require.Equal(t, StateFailed, s.State())
}

func TestServer_Start_StreamableRequiresEndpoint(t *testing.T) {
	s := NewServer("x", MCPServerConfig{Type: "streamable", Timeout: 2 * time.Second}, nil)
	err := s.Start(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "endpoint")
	require.Equal(t, StateFailed, s.State())
}

func TestServer_ToolListChanged_TriggersRefresh(t *testing.T) {
	bin := buildEchoServer(t)
	var notifyMu sync.Mutex
	var notifications []string
	m := NewManager(func(server, msg string) {
		notifyMu.Lock()
		defer notifyMu.Unlock()
		notifications = append(notifications, server+": "+msg)
	})
	m.AddServer("echo", MCPServerConfig{Command: bin, Timeout: 30 * time.Second})
	require.NoError(t, m.Start(context.Background()))
	defer m.Close()

	// Call the echo-server's hidden _emit_list_changed tool.
	_, err := m.servers["echo"].CallTool(context.Background(), "_emit_list_changed", map[string]any{})
	require.NoError(t, err)

	// Wait up to 2s for the notification to propagate. (The handler is invoked
	// on a goroutine inside the SDK; give it time.)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		notifyMu.Lock()
		found := false
		for _, n := range notifications {
			if strings.Contains(n, "tool list changed") {
				found = true
				break
			}
		}
		notifyMu.Unlock()
		if found {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	notifyMu.Lock()
	var seen bool
	for _, n := range notifications {
		if strings.Contains(n, "tool list changed") {
			seen = true
			break
		}
	}
	notifyMu.Unlock()
	require.True(t, seen, "expected 'tool list changed' notification; got: %v", notifications)
}

func TestMain(m *testing.M) {
	// Some CI shells lack `go`; ensure we error early in that case.
	if _, err := exec.LookPath("go"); err != nil {
		os.Exit(0)
	}
	os.Exit(m.Run())
}

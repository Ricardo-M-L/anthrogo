package telemetry

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestReporter_DisabledIsNoOp(t *testing.T) {
	r := NewReporter(false, "", "")
	r.Event("test", map[string]any{"x": 1})
	require.Equal(t, 0, len(r.events))
}

func TestReporter_EventSanitizesSensitiveKeys(t *testing.T) {
	r := NewReporter(true, "", t.TempDir())
	defer r.Close()
	r.Event("test", map[string]any{
		"path":    "/secret/file",
		"command": "rm -rf /",
		"model":   "claude-sonnet-4-6",
	})
	r.mu.Lock()
	defer r.mu.Unlock()
	require.Len(t, r.events, 1)
	require.NotContains(t, r.events[0], "path")
	require.NotContains(t, r.events[0], "command")
	require.Equal(t, "claude-sonnet-4-6", r.events[0]["model"])
}

func TestReporter_FlushSendsBatch(t *testing.T) {
	gotCh := make(chan []byte, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		fmt.Fprintln(w, "ok")
		// Non-blocking send so a double-call doesn't deadlock the test server.
		select {
		case gotCh <- body:
		default:
		}
	}))
	defer srv.Close()
	r := NewReporter(true, srv.URL, t.TempDir())
	r.Event("test", map[string]any{"x": 1})
	r.Close()
	select {
	case got := <-gotCh:
		require.Contains(t, string(got), "\"events\"")
	case <-time.After(2 * time.Second):
		t.Fatal("reporter did not flush within 2s")
	}
}

func TestSanitizeEvent_RemovesSensitiveKeys(t *testing.T) {
	data := map[string]any{
		"path":     "/home/user/secret",
		"command":  "echo hi",
		"prompt":   "user message",
		"text":     "some text",
		"cwd":      "/home/user",
		"input":    "raw input",
		"args":     []string{"a", "b"},
		"stdout":   "output",
		"stderr":   "error",
		"url":      "https://example.com",
		"endpoint": "https://api.example.com",
		"api_key":  "sk-secret",
		"token":    "bearer-token",
		"secret":   "my-secret",
		"model":    "claude-sonnet-4-6",
		"turns":    42,
	}
	out := sanitizeEvent(data)
	// none of the sensitive keys should be present
	for k := range sensitiveKeys {
		require.NotContains(t, out, k)
	}
	// safe keys should be preserved
	require.Equal(t, "claude-sonnet-4-6", out["model"])
	require.Equal(t, 42, out["turns"])
}

func TestReporter_IsEnabled(t *testing.T) {
	r1 := NewReporter(false, "", "")
	require.False(t, r1.IsEnabled())

	r2 := NewReporter(true, "", t.TempDir())
	defer r2.Close()
	require.True(t, r2.IsEnabled())
}

func TestReporter_Endpoint(t *testing.T) {
	r := NewReporter(true, "https://tel.example.com/events", t.TempDir())
	defer r.Close()
	require.Equal(t, "https://tel.example.com/events", r.Endpoint())
}

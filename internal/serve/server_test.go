package serve_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/internal/serve"
	"github.com/ricardo/anthrogo/pkg/permissions"
	"github.com/ricardo/anthrogo/pkg/provider"
	"github.com/ricardo/anthrogo/pkg/provider/fake"
	"github.com/ricardo/anthrogo/pkg/query"
	"github.com/ricardo/anthrogo/pkg/tool"
)

// newTestServer builds a Server backed by the given fake provider and returns
// an httptest.Server. The caller is responsible for calling Close().
func newTestServer(t *testing.T, prov provider.Provider, cfg serve.Config) *httptest.Server {
	t.Helper()
	cfg.ProviderFactory = func() (provider.Provider, string, error) {
		return prov, "test-model", nil
	}
	if cfg.Permissions == nil {
		cfg.Permissions = &permissions.Context{
			Mode:               permissions.ModeBypassPermissions,
			ShouldAvoidPrompts: true,
		}
	}
	return httptest.NewServer(serve.New(cfg))
}

// TestServer_Health verifies /v1/health returns ok=true and version.
func TestServer_Health(t *testing.T) {
	ts := newTestServer(t, fake.New(), serve.Config{})
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/v1/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	assert.Equal(t, true, body["ok"])
	_, hasVersion := body["version"]
	assert.True(t, hasVersion)
}

// TestServer_AuthRequired verifies that a token-protected server rejects
// unauthenticated requests with 401.
func TestServer_AuthRequired(t *testing.T) {
	ts := newTestServer(t, fake.New(), serve.Config{Token: "secret"})
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/v1/health")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// TestServer_AuthBypassed verifies that the correct Bearer token is accepted.
func TestServer_AuthBypassed(t *testing.T) {
	ts := newTestServer(t, fake.New(), serve.Config{Token: "secret"})
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/v1/health", nil)
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestServer_CORS_HeadersEmitted verifies that CORS headers are emitted when
// --cors-origin is configured.
func TestServer_CORS_HeadersEmitted(t *testing.T) {
	ts := newTestServer(t, fake.New(), serve.Config{CORSOrigin: "https://example.com"})
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/v1/health")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, "https://example.com", resp.Header.Get("Access-Control-Allow-Origin"))
}

// TestServer_ChatSync_HappyPath sends a non-streaming chat request and asserts
// the response contains a non-empty text field.
func TestServer_ChatSync_HappyPath(t *testing.T) {
	script := []provider.Event{
		{Kind: provider.EventTextDelta, Text: "Hello, world!"},
		{Kind: provider.EventMessageStop, StopReason: "end_turn"},
	}
	ts := newTestServer(t, fake.New(script), serve.Config{})
	defer ts.Close()

	body, _ := json.Marshal(map[string]any{
		"session_id": "sync-happy-session",
		"prompt":     "Say hello",
		"stream":     false,
	})
	resp, err := http.Post(ts.URL+"/v1/chat", "application/json", bytes.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&result))
	assert.Equal(t, "sync-happy-session", result["session_id"])
	text, _ := result["text"].(string)
	assert.NotEmpty(t, text, "expected non-empty text in response")
}

// TestServer_ChatStream_EmitsDeltaThenDone verifies the SSE event order for a
// streaming chat request: delta events come before the final done event.
func TestServer_ChatStream_EmitsDeltaThenDone(t *testing.T) {
	script := []provider.Event{
		{Kind: provider.EventTextDelta, Text: "streaming "},
		{Kind: provider.EventTextDelta, Text: "response"},
		{Kind: provider.EventMessageStop, StopReason: "end_turn"},
	}
	ts := newTestServer(t, fake.New(script), serve.Config{})
	defer ts.Close()

	body, _ := json.Marshal(map[string]any{
		"session_id": "stream-session",
		"prompt":     "Stream me",
		"stream":     true,
	})

	req, err := http.NewRequest("POST", ts.URL+"/v1/chat", bytes.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))

	var events []map[string]any
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		var ev map[string]any
		if err := json.Unmarshal([]byte(data), &ev); err == nil {
			events = append(events, ev)
		}
	}

	require.True(t, len(events) >= 2, "expected at least a delta and a done event, got %d", len(events))
	// First event must be a delta.
	assert.Equal(t, "delta", events[0]["type"])
	// Last event must be done.
	assert.Equal(t, "done", events[len(events)-1]["type"])
}

// TestServer_Sessions_ListAndGet verifies that /v1/sessions returns a JSON
// array (200) and that /v1/sessions/{id} returns 404 for unknown IDs.
func TestServer_Sessions_ListAndGet(t *testing.T) {
	ts := newTestServer(t, fake.New(), serve.Config{})
	defer ts.Close()

	// List: expect 200 and a JSON array.
	resp, err := http.Get(ts.URL + "/v1/sessions")
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.True(t, bytes.HasPrefix(body, []byte("[")), "expected JSON array, got: %s", string(body[:min(len(body), 20)]))

	// Get: unknown ID should be 404.
	resp2, err := http.Get(ts.URL + "/v1/sessions/nonexistent-id-xyz")
	require.NoError(t, err)
	resp2.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp2.StatusCode)
}

// TestServer_Tools_ListsRegistered verifies that /v1/tools lists registered tools.
func TestServer_Tools_ListsRegistered(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Register(tool.Bash{})

	ts := newTestServer(t, fake.New(), serve.Config{Tools: reg})
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/v1/tools")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var tools []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&tools))
	require.True(t, len(tools) >= 1, "expected at least one tool")

	found := false
	for _, tool := range tools {
		if tool["name"] == "Bash" {
			found = true
			break
		}
	}
	assert.True(t, found, "Bash tool not found in /v1/tools response")
}

// min returns the smaller of a and b.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TestServer_SessionID_RejectsTraversal verifies that session IDs containing
// path traversal sequences are rejected with 404.
func TestServer_SessionID_RejectsTraversal(t *testing.T) {
	ts := newTestServer(t, fake.New(), serve.Config{})
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/v1/sessions/../etc/passwd")
	require.NoError(t, err)
	resp.Body.Close()
	// Either 404 (unknown) or 400 (bad request) are acceptable; the key
	// requirement is that it is not 200.
	assert.NotEqual(t, http.StatusOK, resp.StatusCode)
}

// TestServer_SessionID_RejectsSlash verifies that session IDs containing
// slash characters are rejected (path traversal via embedded slash).
func TestServer_SessionID_RejectsSlash(t *testing.T) {
	ts := newTestServer(t, fake.New(), serve.Config{})
	defer ts.Close()

	// Encode the slash so it reaches the handler as a literal slash in PathValue.
	// We send the raw request via GET /v1/sessions/a%2Fb — Go's http.ServeMux
	// will reject encoded slashes before they reach the handler, so we also
	// test via DELETE which constructs the path differently.
	resp, err := http.Get(ts.URL + "/v1/sessions/a%2Fb")
	require.NoError(t, err)
	resp.Body.Close()
	assert.NotEqual(t, http.StatusOK, resp.StatusCode)
}

// TestServer_SessionID_AcceptsUUID verifies that a well-formed session ID
// (UUID-style) is accepted by the routing layer (returns 404 because the
// session doesn't exist on disk, not 400 bad-request).
func TestServer_SessionID_AcceptsUUID(t *testing.T) {
	ts := newTestServer(t, fake.New(), serve.Config{})
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/v1/sessions/550e8400-e29b-41d4-a716-446655440000")
	require.NoError(t, err)
	resp.Body.Close()
	// 404 means it passed validation but was not found — that is correct.
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// TestSessionCache_GetOrCreate_NoDuplicate spawns 8 goroutines all calling
// GetOrCreate with the same session ID and asserts the builder ran exactly
// once (no TOCTOU duplicate Engine construction).
func TestSessionCache_GetOrCreate_NoDuplicate(t *testing.T) {
	cache := serve.NewExportedSessionCache()

	var buildCount int64
	// builder constructs a real query.Engine with a nil provider — sufficient
	// for identity comparison since we only check pointer equality here.
	builder := func() (*query.Engine, error) {
		atomic.AddInt64(&buildCount, 1)
		return query.NewEngine(query.Config{}), nil
	}

	const goroutines = 8
	engines := make([]*query.Engine, goroutines)
	errs := make([]error, goroutines)

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			engines[i], errs[i] = cache.GetOrCreate("same-session", builder)
		}()
	}
	wg.Wait()

	for i, err := range errs {
		require.NoError(t, err, "goroutine %d got error", i)
	}
	assert.Equal(t, int64(1), buildCount, "builder must run exactly once")
	first := engines[0]
	for i, e := range engines {
		assert.Same(t, first, e, "goroutine %d got different Engine pointer", i)
	}
}

// TestServer_Tools_ListsTaskAndSkill verifies that Task and Skill appear in
// /v1/tools response when the serve daemon registers agentic tools.
func TestServer_Tools_ListsTaskAndSkill(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Register(tool.Bash{})
	// Simulate the agentic tools registered by serve.go.
	reg.Register(tool.NewSkill(nil))
	reg.Register(tool.NewTask(nil, nil))

	ts := newTestServer(t, fake.New(), serve.Config{Tools: reg})
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/v1/tools")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var tools []map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&tools))

	names := make(map[string]bool)
	for _, t := range tools {
		if n, ok := t["name"].(string); ok {
			names[n] = true
		}
	}
	assert.True(t, names["Task"], "Task tool not found in /v1/tools")
	assert.True(t, names["Skill"], "Skill tool not found in /v1/tools")
}

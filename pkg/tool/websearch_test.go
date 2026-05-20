package tool

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWebSearch_BraveBackend(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "test-key", r.Header.Get("X-Subscription-Token"))
		require.Equal(t, "golang", r.URL.Query().Get("q"))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"web": map[string]any{
				"results": []map[string]any{
					{"title": "Go", "url": "https://go.dev", "description": "The language"},
					{"title": "Effective Go", "url": "https://go.dev/doc/effective_go", "description": "Style guide"},
				},
			},
		})
	}))
	defer srv.Close()

	ws := NewWebSearch(WebSearchConfig{Backend: "brave", APIKey: "test-key", Endpoint: srv.URL})
	res, err := ws.Call(context.Background(), map[string]any{"query": "golang"}, &Context{})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Contains(t, res.Text, "Go")
	require.Contains(t, res.Text, "https://go.dev")
}

func TestWebSearch_NoAPIKey_Errors(t *testing.T) {
	ws := NewWebSearch(WebSearchConfig{Backend: "brave"})
	res, _ := ws.Call(context.Background(), map[string]any{"query": "x"}, &Context{})
	require.True(t, res.IsError)
	require.Contains(t, res.Text, "apiKey")
}

func TestWebSearch_DisabledBackend(t *testing.T) {
	ws := NewWebSearch(WebSearchConfig{Backend: "disabled"})
	res, _ := ws.Call(context.Background(), map[string]any{"query": "x"}, &Context{})
	require.True(t, res.IsError)
}

func TestWebSearch_GoogleBackend(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "test-key", r.URL.Query().Get("key"))
		require.Equal(t, "my-cx", r.URL.Query().Get("cx"))
		require.Equal(t, "openai", r.URL.Query().Get("q"))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{
				{"title": "OpenAI", "link": "https://openai.com", "snippet": "AI company"},
				{"title": "OpenAI Blog", "link": "https://openai.com/blog", "snippet": "Latest news"},
			},
		})
	}))
	defer srv.Close()

	ws := NewWebSearch(WebSearchConfig{
		Backend:  "google",
		APIKey:   "test-key",
		Endpoint: "my-cx",
		URL:      srv.URL,
	})
	res, err := ws.Call(context.Background(), map[string]any{"query": "openai", "count": 5}, &Context{})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Contains(t, res.Text, "OpenAI")
	require.Contains(t, res.Text, "https://openai.com")
	require.Equal(t, 2, res.Data["count"])
}

func TestWebSearch_BingBackend(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "bing-key", r.Header.Get("Ocp-Apim-Subscription-Key"))
		require.Equal(t, "rust lang", r.URL.Query().Get("q"))
		_ = json.NewEncoder(w).Encode(map[string]any{
			"webPages": map[string]any{
				"value": []map[string]any{
					{"name": "Rust", "url": "https://rust-lang.org", "snippet": "Systems lang"},
					{"name": "Rust Book", "url": "https://doc.rust-lang.org/book", "snippet": "Official book"},
				},
			},
		})
	}))
	defer srv.Close()

	ws := NewWebSearch(WebSearchConfig{
		Backend: "bing",
		APIKey:  "bing-key",
		URL:     srv.URL,
	})
	res, err := ws.Call(context.Background(), map[string]any{"query": "rust lang"}, &Context{})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Contains(t, res.Text, "Rust")
	require.Contains(t, res.Text, "https://rust-lang.org")
	require.Equal(t, 2, res.Data["count"])
}

func TestWebSearch_TavilyBackend(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "POST", r.Method)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		require.Equal(t, "tavily-key", body["api_key"])
		require.Equal(t, "kubernetes", body["query"])
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{"title": "Kubernetes", "url": "https://kubernetes.io", "content": "Container orchestration"},
				{"title": "K8s Docs", "url": "https://kubernetes.io/docs", "content": "Official docs"},
			},
		})
	}))
	defer srv.Close()

	ws := NewWebSearch(WebSearchConfig{
		Backend: "tavily",
		APIKey:  "tavily-key",
		URL:     srv.URL,
	})
	res, err := ws.Call(context.Background(), map[string]any{"query": "kubernetes"}, &Context{})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Contains(t, res.Text, "Kubernetes")
	require.Contains(t, res.Text, "https://kubernetes.io")
	require.Equal(t, 2, res.Data["count"])
}

func TestWebSearch_UnknownBackendError(t *testing.T) {
	ws := NewWebSearch(WebSearchConfig{Backend: "duckduckgo", APIKey: "k"})
	res, err := ws.Call(context.Background(), map[string]any{"query": "test"}, &Context{})
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Text, "unknown backend")
	require.Contains(t, res.Text, "duckduckgo")
}

func TestWebSearch_EmptyQuery(t *testing.T) {
	ws := NewWebSearch(WebSearchConfig{Backend: "brave", APIKey: "k"})
	res, err := ws.Call(context.Background(), map[string]any{}, &Context{})
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Text, "query required")
}

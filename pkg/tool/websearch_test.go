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
	require.Contains(t, res.Text, "configure")
}

func TestWebSearch_DisabledBackend(t *testing.T) {
	ws := NewWebSearch(WebSearchConfig{Backend: "disabled"})
	res, _ := ws.Call(context.Background(), map[string]any{"query": "x"}, &Context{})
	require.True(t, res.IsError)
}

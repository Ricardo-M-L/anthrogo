package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// buildEmbedResponse builds an OpenAI-format embeddings response with n vectors of dimension dim.
func buildEmbedResponse(model string, n, dim int) []byte {
	data := make([]map[string]any, n)
	for i := 0; i < n; i++ {
		vec := make([]float64, dim)
		for j := 0; j < dim; j++ {
			vec[j] = float64(i+1) * 0.01 * float64(j+1)
		}
		data[i] = map[string]any{
			"embedding": vec,
			"index":     i,
		}
	}
	resp := map[string]any{
		"data":  data,
		"model": model,
		"usage": map[string]any{"prompt_tokens": 10, "total_tokens": 10},
	}
	b, _ := json.Marshal(resp)
	return b
}

func TestEmbed_SingleInput_Summary(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/embeddings", r.URL.Path)
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildEmbedResponse("text-embedding-3-small", 1, 16))
	}))
	defer srv.Close()

	tool := Embed{httpClient: srv.Client()}
	res, err := tool.Call(context.Background(), map[string]any{
		"input":    "hello world",
		"base_url": srv.URL,
		"api_key":  "test-key",
	}, nil)
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Contains(t, res.Text, "embedded 1 text(s)")
	require.Contains(t, res.Text, "text-embedding-3-small")
	require.Contains(t, res.Text, "dim=16")
	require.Contains(t, res.Text, "[0] preview:")
}

func TestEmbed_BatchInputList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		inputs, _ := body["input"].([]any)
		require.Len(t, inputs, 3)
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildEmbedResponse("text-embedding-ada-002", 3, 8))
	}))
	defer srv.Close()

	tool := Embed{httpClient: srv.Client()}
	res, err := tool.Call(context.Background(), map[string]any{
		"input_list": []any{"a", "b", "c"},
		"model":      "text-embedding-ada-002",
		"base_url":   srv.URL,
		"api_key":    "test-key",
	}, nil)
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Contains(t, res.Text, "embedded 3 text(s)")
	require.Contains(t, res.Text, "[2] preview:")
}

func TestEmbed_JSONOutput(t *testing.T) {
	const dim = 4
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(buildEmbedResponse("my-model", 1, dim))
	}))
	defer srv.Close()

	tool := Embed{httpClient: srv.Client()}
	res, err := tool.Call(context.Background(), map[string]any{
		"input":      "test",
		"base_url":   srv.URL,
		"api_key":    "key",
		"out_format": "json",
	}, nil)
	require.NoError(t, err)
	require.False(t, res.IsError)

	var out struct {
		Vectors [][]float64 `json:"vectors"`
		Model   string      `json:"model"`
		Dim     int         `json:"dim"`
	}
	require.NoError(t, json.Unmarshal([]byte(res.Text), &out))
	require.Len(t, out.Vectors, 1)
	require.Equal(t, dim, out.Dim)
	require.Equal(t, "my-model", out.Model)
}

func TestEmbed_MissingKey_IsError(t *testing.T) {
	// Clear all possible key env vars so the tool truly has no key.
	t.Setenv("ANTHROGO_EMBED_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	tool := Embed{}
	res, err := tool.Call(context.Background(), map[string]any{
		"input":    "hello",
		"base_url": "http://localhost:1",
	}, nil)
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Text, "api_key")
}

func TestEmbed_NoInput_IsError(t *testing.T) {
	tool := Embed{}
	res, err := tool.Call(context.Background(), map[string]any{
		"base_url": "http://localhost:1",
		"api_key":  "k",
	}, nil)
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Text, "input")
}

func TestEmbed_ServerError_IsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":"internal"}`)
	}))
	defer srv.Close()

	tool := Embed{httpClient: srv.Client()}
	res, err := tool.Call(context.Background(), map[string]any{
		"input":    "test",
		"base_url": srv.URL,
		"api_key":  "k",
	}, nil)
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Text, "500")
}

func TestEmbed_ErrorRedactsAPIKey(t *testing.T) {
	const fakeKey = "sk-abcdef0123456789"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		// Simulate an API that echoes the key back in the error body.
		fmt.Fprintf(w, `{"error":"invalid key: %s"}`, fakeKey)
	}))
	defer srv.Close()

	tool := Embed{httpClient: srv.Client()}
	res, err := tool.Call(context.Background(), map[string]any{
		"input":    "test",
		"base_url": srv.URL,
		"api_key":  fakeKey,
	}, nil)
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Text, "sk-***REDACTED***", "API key must be redacted in error message")
	require.NotContains(t, res.Text, fakeKey, "raw API key must not appear in error message")
}

package tool

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// minimalPNG is the 8-byte PNG signature used as test image data.
var minimalPNG = []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}

func imageGenServer(t *testing.T, statusCode int, b64data string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/images/generations", r.URL.Path)
		if statusCode != http.StatusOK {
			w.WriteHeader(statusCode)
			fmt.Fprintf(w, `{"error":"server error"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"data": []map[string]any{
				{"b64_json": b64data},
			},
		}
		b, _ := json.Marshal(resp)
		w.Write(b)
	}))
}

func TestImageGen_HappyPath(t *testing.T) {
	b64 := base64.StdEncoding.EncodeToString(minimalPNG)
	srv := imageGenServer(t, http.StatusOK, b64)
	defer srv.Close()

	outPath := filepath.Join(t.TempDir(), "out.png")
	tool := ImageGen{httpClient: srv.Client()}
	res, err := tool.Call(context.Background(), map[string]any{
		"prompt":   "a cat",
		"base_url": srv.URL,
		"api_key":  "test-key",
		"out_path": outPath,
	}, nil)
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Contains(t, res.Text, "saved")
	require.Contains(t, res.Text, outPath)
	require.Contains(t, res.Text, "dall-e-3")

	data, readErr := os.ReadFile(outPath)
	require.NoError(t, readErr)
	require.Equal(t, minimalPNG, data)
}

func TestImageGen_MissingPrompt_IsError(t *testing.T) {
	tool := ImageGen{}
	res, err := tool.Call(context.Background(), map[string]any{
		"api_key":  "k",
		"base_url": "http://localhost:1",
	}, nil)
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Text, "prompt")
}

func TestImageGen_MissingKey_IsError(t *testing.T) {
	// Clear all possible key env vars so the tool truly has no key.
	t.Setenv("ANTHROGO_IMAGE_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	tool := ImageGen{}
	res, err := tool.Call(context.Background(), map[string]any{
		"prompt":   "a dog",
		"base_url": "http://localhost:1",
	}, nil)
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Text, "api_key")
}

func TestImageGen_ServerError_IsError(t *testing.T) {
	srv := imageGenServer(t, http.StatusInternalServerError, "")
	defer srv.Close()

	tool := ImageGen{httpClient: srv.Client()}
	res, err := tool.Call(context.Background(), map[string]any{
		"prompt":   "fire",
		"base_url": srv.URL,
		"api_key":  "k",
	}, nil)
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Text, "500")
}

func TestImageGen_ErrorRedactsAPIKey(t *testing.T) {
	const fakeKey = "sk-abcdef0123456789"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprintf(w, `{"error":"invalid key: %s"}`, fakeKey)
	}))
	defer srv.Close()

	tool := ImageGen{httpClient: srv.Client()}
	res, err := tool.Call(context.Background(), map[string]any{
		"prompt":   "a cat",
		"base_url": srv.URL,
		"api_key":  fakeKey,
	}, nil)
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Text, "sk-***REDACTED***", "API key must be redacted in error message")
	require.NotContains(t, res.Text, fakeKey, "raw API key must not appear in error message")
}

func TestImageGen_CustomOutPath(t *testing.T) {
	b64 := base64.StdEncoding.EncodeToString(minimalPNG)
	srv := imageGenServer(t, http.StatusOK, b64)
	defer srv.Close()

	dir := t.TempDir()
	outPath := filepath.Join(dir, "custom-output.png")

	tool := ImageGen{httpClient: srv.Client()}
	res, err := tool.Call(context.Background(), map[string]any{
		"prompt":   "ocean",
		"base_url": srv.URL,
		"api_key":  "k",
		"out_path": outPath,
		"model":    "dall-e-2",
		"size":     "512x512",
	}, nil)
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Contains(t, res.Text, outPath)
	require.Contains(t, res.Text, "dall-e-2")
	require.Contains(t, res.Text, "512x512")

	require.Equal(t, outPath, res.Data["out_path"])
	_, statErr := os.Stat(outPath)
	require.NoError(t, statErr)
}

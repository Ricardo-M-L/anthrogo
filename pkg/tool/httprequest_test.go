package tool

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHTTPRequest_GetSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "GET", r.Method)
		fmt.Fprintln(w, "hello")
	}))
	defer srv.Close()

	res, err := (&HTTPRequest{}).Call(context.Background(), map[string]any{"url": srv.URL}, nil)
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Contains(t, res.Text, "hello")
	require.Equal(t, 200, res.Data["status"])
}

func TestHTTPRequest_POSTWithBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		require.Equal(t, "POST", r.Method)
		require.Equal(t, `{"x":1}`, string(body))
		w.WriteHeader(201)
		w.Write([]byte("created"))
	}))
	defer srv.Close()

	res, _ := (&HTTPRequest{}).Call(context.Background(), map[string]any{
		"method": "POST",
		"url":    srv.URL,
		"body":   `{"x":1}`,
		"headers": map[string]any{
			"Content-Type": "application/json",
		},
	}, nil)
	require.Contains(t, res.Text, "201")
	require.Contains(t, res.Text, "created")
}

func TestHTTPRequest_RejectsFileScheme(t *testing.T) {
	res, _ := (&HTTPRequest{}).Call(context.Background(), map[string]any{
		"url": "file:///etc/passwd",
	}, nil)
	require.True(t, res.IsError)
	require.Contains(t, res.Text, "http(s)")
}

func TestHTTPRequest_RejectsBadMethod(t *testing.T) {
	res, _ := (&HTTPRequest{}).Call(context.Background(), map[string]any{
		"method": "TRACE",
		"url":    "http://example.com",
	}, nil)
	require.True(t, res.IsError)
}

func TestHTTPRequest_SaveTo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("disk content"))
	}))
	defer srv.Close()

	dst := filepath.Join(t.TempDir(), "out.txt")
	res, _ := (&HTTPRequest{}).Call(context.Background(), map[string]any{
		"url":     srv.URL,
		"save_to": dst,
	}, nil)
	require.Contains(t, res.Text, "saved")
	data, _ := os.ReadFile(dst)
	require.Equal(t, "disk content", string(data))
}

func TestHTTPRequest_404IsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer srv.Close()

	res, _ := (&HTTPRequest{}).Call(context.Background(), map[string]any{"url": srv.URL}, nil)
	require.True(t, res.IsError)
	require.Equal(t, 404, res.Data["status"])
}

func TestHTTPRequest_MaxResponseSizeTruncates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(bytes.Repeat([]byte("x"), 1000))
	}))
	defer srv.Close()

	res, _ := (&HTTPRequest{}).Call(context.Background(), map[string]any{
		"url":               srv.URL,
		"max_response_size": 100,
	}, nil)
	require.Equal(t, true, res.Data["truncated"])
}

package kairos

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDispatchRemote_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/kairos/run", r.URL.Path)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		fmt.Fprintf(w, "event: text\ndata: {\"text\":\"hello \"}\n\n")
		flusher.Flush()
		fmt.Fprintf(w, "event: text\ndata: {\"text\":\"world\"}\n\n")
		flusher.Flush()
		fmt.Fprintf(w, "event: done\ndata: {\"final\":\"hello world\"}\n\n")
		flusher.Flush()
	}))
	defer srv.Close()

	result, err := DispatchRemote(context.Background(), srv.URL, "", "general-purpose", "test", "do something")
	require.NoError(t, err)
	require.Equal(t, "hello world", result)
}

func TestDispatchRemote_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()

	_, err := DispatchRemote(context.Background(), srv.URL, "wrongtoken", "general-purpose", "", "prompt")
	require.Error(t, err)
	require.Contains(t, err.Error(), "401")
}

func TestDispatchRemote_ErrorEvent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		fmt.Fprintf(w, "event: error\ndata: {\"error\":\"worker exploded\"}\n\n")
		flusher.Flush()
	}))
	defer srv.Close()

	_, err := DispatchRemote(context.Background(), srv.URL, "", "general-purpose", "", "prompt")
	require.Error(t, err)
	require.Contains(t, err.Error(), "worker exploded")
}

func TestDispatchRemote_AccumulatesDeltas_WhenNoDone(t *testing.T) {
	// When server only sends text deltas without a done event, client falls back
	// to the accumulated string.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		fmt.Fprintf(w, "event: text\ndata: {\"text\":\"part1\"}\n\n")
		flusher.Flush()
		fmt.Fprintf(w, "event: text\ndata: {\"text\":\" part2\"}\n\n")
		flusher.Flush()
	}))
	defer srv.Close()

	result, err := DispatchRemote(context.Background(), srv.URL, "", "general-purpose", "", "prompt")
	require.NoError(t, err)
	require.Equal(t, "part1 part2", result)
}

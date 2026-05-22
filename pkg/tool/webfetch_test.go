package tool

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWebFetch_FetchesHTML(t *testing.T) {
	t.Setenv("ANTHROGO_NETGUARD_ALLOW_LOOPBACK", "1")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><h1>Hi</h1><p>world</p></body></html>`))
	}))
	defer srv.Close()

	wf := NewWebFetch()
	res, err := wf.Call(context.Background(), map[string]any{"url": srv.URL}, &Context{})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Contains(t, res.Text, "Hi")
	require.Contains(t, res.Text, "world")
}

func TestWebFetch_NonHTMLReturnedAsIs(t *testing.T) {
	t.Setenv("ANTHROGO_NETGUARD_ALLOW_LOOPBACK", "1")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("plain text"))
	}))
	defer srv.Close()

	wf := NewWebFetch()
	res, err := wf.Call(context.Background(), map[string]any{"url": srv.URL}, &Context{})
	require.NoError(t, err)
	require.Contains(t, res.Text, "plain text")
}

func TestWebFetch_CachesWithinTTL(t *testing.T) {
	t.Setenv("ANTHROGO_NETGUARD_ALLOW_LOOPBACK", "1")
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits++
		_, _ = w.Write([]byte("hello"))
	}))
	defer srv.Close()

	wf := NewWebFetch()
	for i := 0; i < 3; i++ {
		_, err := wf.Call(context.Background(), map[string]any{"url": srv.URL}, &Context{})
		require.NoError(t, err)
	}
	require.Equal(t, 1, hits)
}

func TestWebFetch_SizeLimit(t *testing.T) {
	t.Setenv("ANTHROGO_NETGUARD_ALLOW_LOOPBACK", "1")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "10485760")
		_, _ = w.Write([]byte(strings.Repeat("x", 6*1024*1024)))
	}))
	defer srv.Close()

	wf := NewWebFetch()
	res, _ := wf.Call(context.Background(), map[string]any{"url": srv.URL}, &Context{})
	require.True(t, res.IsError)
	require.Contains(t, res.Text, "too large")
}

// TestWebFetch_GetCache_DropsOrderEntryOnTTLExpiry verifies that an expired
// cache entry is removed from w.order at expiry time, so a subsequent re-fetch
// doesn't push a duplicate URL into w.order and corrupt the LRU.
func TestWebFetch_GetCache_DropsOrderEntryOnTTLExpiry(t *testing.T) {
	w := NewWebFetch()
	// Manually populate cache with a stale entry (storedAt = past).
	w.cache["http://x/a"] = cacheEntry{body: "a", storedAt: time.Now().Add(-time.Hour)}
	w.order = []string{"http://x/a"}

	got := w.getCache("http://x/a")
	require.Equal(t, "", got, "stale entry should not be returned")
	require.Empty(t, w.order, "w.order should have the stale entry removed")
	require.NotContains(t, w.cache, "http://x/a", "w.cache should have the stale entry removed")

	// Re-put the same URL; w.order should hold exactly one entry, not two.
	w.putCache("http://x/a", "fresh")
	require.Len(t, w.order, 1, "w.order should have a single entry after re-put")
}

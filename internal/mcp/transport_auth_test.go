package mcp

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHeaderInjector_AppendsConfiguredHeaders(t *testing.T) {
	var got http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		fmt.Fprintln(w, "ok")
	}))
	defer srv.Close()

	client := &http.Client{Transport: headerInjector{
		base:    http.DefaultTransport,
		headers: map[string]string{"X-Custom": "abc", "X-Auth": "def"},
	}}
	_, err := client.Get(srv.URL)
	require.NoError(t, err)
	require.Equal(t, "abc", got.Get("X-Custom"))
	require.Equal(t, "def", got.Get("X-Auth"))
}

func TestHeaderInjector_LayersWithBearer(t *testing.T) {
	var got http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		fmt.Fprintln(w, "ok")
	}))
	defer srv.Close()

	// Compose: bearerInjector wraps headerInjector so both sets of headers land.
	var base http.RoundTripper = http.DefaultTransport
	base = headerInjector{base: base, headers: map[string]string{"X-Custom": "xyz"}}
	base = bearerInjector{base: base, token: "mytoken"}

	client := &http.Client{Transport: base}
	_, err := client.Get(srv.URL)
	require.NoError(t, err)
	require.Equal(t, "Bearer mytoken", got.Get("Authorization"))
	require.Equal(t, "xyz", got.Get("X-Custom"))
}

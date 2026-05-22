package tool

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// ── NetGuard unit tests ──────────────────────────────────────────────────────

func TestNetGuard_BlocksLoopback(t *testing.T) {
	g := DefaultNetGuard()
	err := g.CheckURL("http://127.0.0.1/x")
	require.Error(t, err)
	require.Contains(t, err.Error(), "loopback")
}

func TestNetGuard_BlocksLoopbackIPv6(t *testing.T) {
	g := DefaultNetGuard()
	err := g.CheckURL("http://[::1]/x")
	require.Error(t, err)
	require.Contains(t, err.Error(), "loopback")
}

func TestNetGuard_BlocksMetadata(t *testing.T) {
	g := DefaultNetGuard()
	err := g.CheckURL("http://169.254.169.254/latest/meta-data/iam/security-credentials/")
	require.Error(t, err)
	require.Contains(t, err.Error(), "link-local")
}

func TestNetGuard_BlocksPrivate_10(t *testing.T) {
	g := DefaultNetGuard()
	require.Error(t, g.CheckURL("http://10.0.0.1"))
}

func TestNetGuard_BlocksPrivate_172(t *testing.T) {
	g := DefaultNetGuard()
	require.Error(t, g.CheckURL("http://172.16.0.1"))
}

func TestNetGuard_BlocksPrivate_192(t *testing.T) {
	g := DefaultNetGuard()
	require.Error(t, g.CheckURL("http://192.168.1.1"))
}

func TestNetGuard_BlocksUnspecified(t *testing.T) {
	g := DefaultNetGuard()
	require.Error(t, g.CheckURL("http://0.0.0.0/"))
}

func TestNetGuard_BlocksMulticast(t *testing.T) {
	g := DefaultNetGuard()
	require.Error(t, g.CheckURL("http://224.0.0.1/"))
}

func TestNetGuard_RejectsBadScheme_File(t *testing.T) {
	g := DefaultNetGuard()
	err := g.CheckURL("file:///etc/passwd")
	require.Error(t, err)
	require.Contains(t, err.Error(), "scheme")
}

func TestNetGuard_RejectsBadScheme_Gopher(t *testing.T) {
	g := DefaultNetGuard()
	err := g.CheckURL("gopher://example.com/")
	require.Error(t, err)
	require.Contains(t, err.Error(), "scheme")
}

func TestNetGuard_AllowsPublic(t *testing.T) {
	// example.com resolves to public IPs; no error expected.
	// This test makes a real DNS query; skip if network is unavailable.
	g := DefaultNetGuard()
	err := g.CheckURL("https://example.com/path")
	// We only skip — never fail — if DNS is genuinely unavailable.
	if err != nil && isDNSUnavailable(err) {
		t.Skip("DNS unavailable, skipping public IP test")
	}
	require.NoError(t, err)
}

func TestNetGuard_AllowsLoopback_WhenEnabled(t *testing.T) {
	t.Setenv("ANTHROGO_NETGUARD_ALLOW_LOOPBACK", "1")
	g := DefaultNetGuard()
	require.NoError(t, g.CheckURL("http://127.0.0.1/x"))
}

func TestNetGuard_AllowsLinkLocal_WhenEnabled(t *testing.T) {
	t.Setenv("ANTHROGO_NETGUARD_ALLOW_LINKLOCAL", "1")
	g := DefaultNetGuard()
	require.NoError(t, g.CheckURL("http://169.254.169.254/"))
}

func TestNetGuard_AllowsPrivate_WhenEnabled(t *testing.T) {
	t.Setenv("ANTHROGO_NETGUARD_ALLOW_PRIVATE", "1")
	g := DefaultNetGuard()
	require.NoError(t, g.CheckURL("http://10.0.0.1/"))
}

// TestNetGuard_DialerControl_BlocksRebinding verifies that even when
// CheckURL is bypassed (simulating a DNS rebind), the Dialer Control hook
// in HTTPClient still blocks the connection to loopback.
func TestNetGuard_DialerControl_BlocksRebinding(t *testing.T) {
	// Spin up a real loopback server.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	// Default guard — loopback NOT allowed.
	g := DefaultNetGuard()
	client := g.HTTPClient(nil)

	// Try to reach the loopback server directly.
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	require.NoError(t, err)
	_, err = client.Do(req)
	require.Error(t, err, "loopback connection should have been blocked by dialer control")
}

// TestNetGuard_DialerControl_AllowsWhenEnabled verifies the inverse:
// with the env override set, the dialer-wrapped client can reach a loopback server.
func TestNetGuard_DialerControl_AllowsWhenEnabled(t *testing.T) {
	t.Setenv("ANTHROGO_NETGUARD_ALLOW_LOOPBACK", "1")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	g := DefaultNetGuard()
	client := g.HTTPClient(nil)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL, nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, 200, resp.StatusCode)
}

// ── Per-tool SSRF guard tests ────────────────────────────────────────────────

func TestHTTPRequest_SSRFGuard_BlocksLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200) // should never be reached
	}))
	defer srv.Close()

	res, err := (&HTTPRequest{}).Call(context.Background(), map[string]any{"url": srv.URL}, nil)
	require.NoError(t, err)
	require.True(t, res.IsError, "expected SSRF block but got success")
	require.Contains(t, res.Text, "loopback")
}

func TestHTTPRequest_SSRFGuard_BlocksMetadata(t *testing.T) {
	res, err := (&HTTPRequest{}).Call(context.Background(), map[string]any{
		"url": "http://169.254.169.254/latest/meta-data/",
	}, nil)
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Text, "link-local")
}

func TestWebFetch_SSRFGuard_BlocksLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	wf := NewWebFetch()
	res, err := wf.Call(context.Background(), map[string]any{"url": srv.URL}, nil)
	require.NoError(t, err)
	require.True(t, res.IsError, "expected SSRF block but got success")
	require.Contains(t, res.Text, "loopback")
}

func TestEmbed_SSRFGuard_BlocksLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	tool := Embed{}
	res, err := tool.Call(context.Background(), map[string]any{
		"input":    "test",
		"base_url": srv.URL,
		"api_key":  "test-key",
	}, nil)
	require.NoError(t, err)
	require.True(t, res.IsError, "expected SSRF block but got success")
	require.Contains(t, res.Text, "loopback")
}

func TestImageGen_SSRFGuard_BlocksLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	tool := ImageGen{}
	res, err := tool.Call(context.Background(), map[string]any{
		"prompt":   "a cat",
		"base_url": srv.URL,
		"api_key":  "test-key",
	}, nil)
	require.NoError(t, err)
	require.True(t, res.IsError, "expected SSRF block but got success")
	require.Contains(t, res.Text, "loopback")
}

// isDNSUnavailable is a heuristic to detect network-less environments.
func isDNSUnavailable(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	for _, kw := range []string{"no such host", "lookup", "dial", "network", "connection refused", "i/o timeout"} {
		for i := 0; i+len(kw) <= len(s); i++ {
			if s[i:i+len(kw)] == kw {
				return true
			}
		}
	}
	return false
}

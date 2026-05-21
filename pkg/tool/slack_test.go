package tool

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSlackPost_BasicTextSucceeds(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	old := slackURLAllowed
	slackURLAllowed = func(string) bool { return true }
	defer func() { slackURLAllowed = old }()

	tool := SlackPost{httpClient: srv.Client()}
	res, err := tool.Call(context.Background(), map[string]any{
		"webhook_url": srv.URL,
		"text":        "hello world",
	}, nil)
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Contains(t, res.Text, "ok")
}

func TestSlackPost_BlocksJSONMerged(t *testing.T) {
	var capturedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 4096)
		n, _ := r.Body.Read(buf)
		capturedBody = buf[:n]
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	old := slackURLAllowed
	slackURLAllowed = func(string) bool { return true }
	defer func() { slackURLAllowed = old }()

	blocksJSON := `[{"type":"section","text":{"type":"mrkdwn","text":"*hello*"}}]`
	tool := SlackPost{httpClient: srv.Client()}
	res, err := tool.Call(context.Background(), map[string]any{
		"webhook_url": srv.URL,
		"text":        "fallback",
		"blocks":      blocksJSON,
	}, nil)
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Contains(t, string(capturedBody), `"blocks"`)
	require.Contains(t, string(capturedBody), `"text"`)
}

func TestSlackPost_RejectsBadURL(t *testing.T) {
	// Use real URL check (don't override slackURLAllowed)
	tool := SlackPost{}
	res, err := tool.Call(context.Background(), map[string]any{
		"webhook_url": "https://evil.com/services/XXX",
		"text":        "hello",
	}, nil)
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Text, "url must start with")
}

func TestSlackPost_ServerNon200_IsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("invalid_token"))
	}))
	defer srv.Close()

	old := slackURLAllowed
	slackURLAllowed = func(string) bool { return true }
	defer func() { slackURLAllowed = old }()

	tool := SlackPost{httpClient: srv.Client()}
	res, err := tool.Call(context.Background(), map[string]any{
		"webhook_url": srv.URL,
		"text":        "hello",
	}, nil)
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Text, "403")
}

func TestSlackPost_MissingText(t *testing.T) {
	tool := SlackPost{}
	res, err := tool.Call(context.Background(), map[string]any{
		"webhook_url": "https://hooks.slack.com/services/X/Y/Z",
	}, nil)
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Text, "text is required")
}

func TestSlackPost_NoURL_UsesEnv(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	old := slackURLAllowed
	slackURLAllowed = func(string) bool { return true }
	defer func() { slackURLAllowed = old }()

	t.Setenv("SLACK_WEBHOOK_URL", srv.URL)

	tool := SlackPost{httpClient: srv.Client()}
	res, err := tool.Call(context.Background(), map[string]any{
		"text": "env fallback test",
	}, nil)
	require.NoError(t, err)
	require.False(t, res.IsError)

	// Also test: no URL at all → error
	_ = os.Unsetenv("SLACK_WEBHOOK_URL")
	tool2 := SlackPost{}
	res2, err2 := tool2.Call(context.Background(), map[string]any{
		"text": "no url",
	}, nil)
	require.NoError(t, err2)
	require.True(t, res2.IsError)
	require.Contains(t, res2.Text, "no webhook URL")
}

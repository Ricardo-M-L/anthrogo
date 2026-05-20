package kairos

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func makeTestHandler(finalText string) RunHandler {
	return func(ctx context.Context, req RunRequest, emit func(string)) (string, error) {
		emit("delta1 ")
		emit("delta2")
		return finalText, nil
	}
}

func TestServer_Run_Authorized(t *testing.T) {
	srv := NewServer(makeTestHandler("FINAL"), "secret")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	body := `{"subagent_type":"general-purpose","prompt":"hello"}`
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/kairos/run", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer secret")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
}

func TestServer_Run_MissingToken(t *testing.T) {
	srv := NewServer(makeTestHandler("X"), "required-token")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	body := `{"subagent_type":"general-purpose","prompt":"hi"}`
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/kairos/run", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// No Authorization header.

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestServer_Run_BadJSON(t *testing.T) {
	srv := NewServer(makeTestHandler("X"), "")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/kairos/run", strings.NewReader("not-json{{{"))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestServer_Healthz(t *testing.T) {
	srv := NewServer(makeTestHandler("X"), "")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/kairos/healthz")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestServer_Run_MissingPromptOrType(t *testing.T) {
	srv := NewServer(makeTestHandler("X"), "")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Missing prompt.
	body, _ := json.Marshal(RunRequest{SubagentType: "general-purpose"})
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/kairos/run", strings.NewReader(string(body)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

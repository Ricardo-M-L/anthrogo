package kairos

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestKAIROS_RoundTrip starts an in-process server, dispatches a request via
// the client, and asserts the final text matches the handler's return value.
func TestKAIROS_RoundTrip(t *testing.T) {
	handler := func(ctx context.Context, req RunRequest, emit func(string)) (string, error) {
		emit("chunk1 ")
		emit("chunk2 ")
		emit("chunk3")
		return "FINAL", nil
	}

	srv := NewServer(handler, "")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	result, err := DispatchRemote(context.Background(), ts.URL, "", "general-purpose", "round-trip test", "do the thing")
	require.NoError(t, err)
	require.Equal(t, "FINAL", result)
}

// TestKAIROS_RoundTrip_WithAuth verifies Bearer auth works end-to-end.
func TestKAIROS_RoundTrip_WithAuth(t *testing.T) {
	handler := func(ctx context.Context, req RunRequest, emit func(string)) (string, error) {
		return "AUTH_OK", nil
	}

	srv := NewServer(handler, "mytoken")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Correct token succeeds.
	result, err := DispatchRemote(context.Background(), ts.URL, "mytoken", "general-purpose", "", "prompt")
	require.NoError(t, err)
	require.Equal(t, "AUTH_OK", result)

	// Wrong token fails.
	_, err = DispatchRemote(context.Background(), ts.URL, "wrongtoken", "general-purpose", "", "prompt")
	require.Error(t, err)
	require.Contains(t, err.Error(), "401")
}

// TestKAIROS_RoundTrip_HandlerError verifies that handler errors propagate as event:error.
func TestKAIROS_RoundTrip_HandlerError(t *testing.T) {
	handler := func(ctx context.Context, req RunRequest, emit func(string)) (string, error) {
		emit("partial")
		return "", context.DeadlineExceeded
	}

	srv := NewServer(handler, "")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	_, err := DispatchRemote(context.Background(), ts.URL, "", "general-purpose", "", "prompt")
	require.Error(t, err)
	require.Contains(t, err.Error(), "kairos remote error")
}

package mcp

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMCPServerConfig_Expand_HeadersEnv(t *testing.T) {
	t.Setenv("TEST_API_TOKEN", "supersecret")
	t.Setenv("TEST_PLAIN_VAL", "hello")

	cfg := MCPServerConfig{
		Headers: map[string]string{
			"Authorization": "env:TEST_API_TOKEN",
			"X-Plain":       "env:TEST_PLAIN_VAL",
			"X-Static":      "no-env-prefix",
			"X-Empty":       "env:DEFINITELY_NOT_SET_XYZ123",
		},
	}
	cfg.Expand()

	require.Equal(t, "supersecret", cfg.Headers["Authorization"])
	require.Equal(t, "hello", cfg.Headers["X-Plain"])
	require.Equal(t, "no-env-prefix", cfg.Headers["X-Static"], "non-env: values must be untouched")
	require.Equal(t, "", cfg.Headers["X-Empty"], "unset env var should resolve to empty string")
}

func TestMCPServerConfig_Expand_NilHeaders(t *testing.T) {
	cfg := MCPServerConfig{} // Headers is nil
	// Must not panic.
	cfg.Expand()
	require.Nil(t, cfg.Headers)
}

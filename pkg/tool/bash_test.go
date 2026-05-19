//go:build !windows

package tool

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBash_BasicEcho(t *testing.T) {
	res, err := (&Bash{}).Call(context.Background(), map[string]any{"command": "echo hello"}, &Context{})
	require.NoError(t, err)
	require.Contains(t, res.Text, "hello")
	require.False(t, res.IsError)
}

func TestBash_ExitNonZero_IsError(t *testing.T) {
	res, _ := (&Bash{}).Call(context.Background(), map[string]any{"command": "exit 7"}, &Context{})
	require.True(t, res.IsError)
	require.Contains(t, res.Text, "exit status 7")
}

func TestBash_Timeout(t *testing.T) {
	start := time.Now()
	res, _ := (&Bash{}).Call(context.Background(), map[string]any{"command": "sleep 5", "timeout_ms": 200.0}, &Context{})
	elapsed := time.Since(start)
	require.True(t, res.IsError)
	require.True(t, elapsed < 2*time.Second, "expected timeout to fire quickly, got %s", elapsed)
	require.Contains(t, strings.ToLower(res.Text), "timeout")
}

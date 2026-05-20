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

// --- M10.2 sandbox tests ---

func TestBash_SandboxAllowsBasicEcho(t *testing.T) {
	res, err := (&Bash{}).Call(context.Background(), map[string]any{
		"command": "echo sandbox-ok",
		"sandbox": true,
	}, &Context{})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Contains(t, res.Text, "sandbox-ok")
}

func TestBash_SandboxRejectsParentDirNav(t *testing.T) {
	res, err := (&Bash{}).Call(context.Background(), map[string]any{
		"command": "ls ../../etc",
		"sandbox": true,
	}, &Context{})
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Text, "sandbox")
	require.Contains(t, res.Text, "../")
}

func TestBash_SandboxRejectsSshAccess(t *testing.T) {
	res, err := (&Bash{}).Call(context.Background(), map[string]any{
		"command": "cat ~/.ssh/id_rsa",
		"sandbox": true,
	}, &Context{})
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Text, "sandbox")
}

func TestBash_SandboxStripsSensitiveEnv(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sentinel-value-should-not-appear")
	res, err := (&Bash{}).Call(context.Background(), map[string]any{
		"command": "printenv ANTHROPIC_API_KEY || true",
		"sandbox": true,
	}, &Context{})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.NotContains(t, res.Text, "sentinel-value-should-not-appear")
}

func TestBash_NonSandboxKeepsEnv(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sentinel-passthrough")
	res, err := (&Bash{}).Call(context.Background(), map[string]any{
		"command": "printenv ANTHROPIC_API_KEY",
	}, &Context{})
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Contains(t, res.Text, "sentinel-passthrough")
}

// --- M10.10 AST scan sandbox tests ---

func TestBash_SandboxRejectsForbiddenBinary(t *testing.T) {
	res, _ := (&Bash{}).Call(context.Background(), map[string]any{
		"command": "rm /tmp/foo", "sandbox": true,
	}, nil)
	require.True(t, res.IsError)
	require.Contains(t, res.Text, "forbidden binary")
}

func TestBash_SandboxRejectsSudo(t *testing.T) {
	res, _ := (&Bash{}).Call(context.Background(), map[string]any{
		"command": "sudo ls", "sandbox": true,
	}, nil)
	require.True(t, res.IsError)
}

func TestBash_SandboxParseFailureRejected(t *testing.T) {
	res, _ := (&Bash{}).Call(context.Background(), map[string]any{
		"command": "if then else", "sandbox": true,
	}, nil)
	require.True(t, res.IsError)
}

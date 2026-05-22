package hooks

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func script(name string) string {
	abs, _ := filepath.Abs(filepath.Join("testdata", name))
	return abs
}

func TestRunner_AllowExit0(t *testing.T) {
	r, err := RunHook(context.Background(), Spec{Command: script("allow.sh"), Timeout: 3 * time.Second}, map[string]any{
		"hook_event_name": "PreToolUse",
	})
	require.NoError(t, err)
	require.Equal(t, 0, r.ExitCode)
	require.Empty(t, r.Stderr)
	require.Nil(t, r.Output)
}

func TestRunner_PassesStdinJSON(t *testing.T) {
	r, err := RunHook(context.Background(), Spec{Command: script("passthrough.sh"), Timeout: 3 * time.Second}, map[string]any{
		"hook_event_name": "PreToolUse",
		"tool_name":       "Bash",
	})
	require.NoError(t, err)
	require.Equal(t, 0, r.ExitCode)
	require.Contains(t, string(r.Stderr), `"tool_name":"Bash"`)
}

func TestRunner_Exit2Block(t *testing.T) {
	r, err := RunHook(context.Background(), Spec{Command: script("deny.sh"), Timeout: 3 * time.Second}, map[string]any{})
	require.NoError(t, err)
	require.Equal(t, 2, r.ExitCode)
	require.Contains(t, string(r.Stderr), "denied by policy")
}

func TestRunner_ExitOtherNonZero(t *testing.T) {
	r, err := RunHook(context.Background(), Spec{Command: script("crash.sh"), Timeout: 3 * time.Second}, map[string]any{})
	require.NoError(t, err)
	require.Equal(t, 7, r.ExitCode)
	require.Contains(t, string(r.Stderr), "boom")
}

func TestRunner_Timeout(t *testing.T) {
	r, err := RunHook(context.Background(), Spec{Command: script("slow.sh"), Timeout: 200 * time.Millisecond}, map[string]any{})
	require.NoError(t, err)
	require.True(t, r.TimedOut)
	require.Equal(t, -1, r.ExitCode)
}

func TestRunner_ParsesJSONOutput(t *testing.T) {
	r, err := RunHook(context.Background(), Spec{Command: script("mutate-input.sh"), Timeout: 3 * time.Second}, map[string]any{
		"tool_name": "Bash",
	})
	require.NoError(t, err)
	require.Equal(t, 0, r.ExitCode)
	require.NotNil(t, r.Output)
	require.NotNil(t, r.Output.HookSpecificOutput)
	require.Equal(t, "ls -al", r.Output.HookSpecificOutput.ModifiedInput["command"])
}

func TestRunner_CommandNotFound(t *testing.T) {
	_, err := RunHook(context.Background(), Spec{Command: "/nonexistent/binary", Timeout: 1 * time.Second}, map[string]any{})
	// `sh -c` with non-existent binary exits 127, doesn't error in Start.
	// So we expect no setup error but exit code 127.
	require.NoError(t, err)
}

func TestSanitizedEnv_StripsAPIKeyPrefixes(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-secret")
	t.Setenv("OPENAI_API_KEY", "sk-openai")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "aws-secret")
	t.Setenv("GITHUB_TOKEN", "gh-secret")
	t.Setenv("USER", "alice") // should pass through
	t.Setenv("ANTHROGO_HOOKS_FULL_ENV", "")

	env := sanitizedEnv()

	require.NotContains(t, env, "ANTHROPIC_API_KEY=sk-secret")
	require.NotContains(t, env, "OPENAI_API_KEY=sk-openai")
	require.NotContains(t, env, "AWS_SECRET_ACCESS_KEY=aws-secret")
	require.NotContains(t, env, "GITHUB_TOKEN=gh-secret")

	found := false
	for _, e := range env {
		if e == "USER=alice" {
			found = true
		}
	}
	require.True(t, found, "USER should be passed through")
}

func TestSanitizedEnv_FullEnvEscapeHatch(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-secret")
	t.Setenv("ANTHROGO_HOOKS_FULL_ENV", "1")

	env := sanitizedEnv()

	found := false
	for _, e := range env {
		if e == "ANTHROPIC_API_KEY=sk-secret" {
			found = true
		}
	}
	require.True(t, found, "with FULL_ENV=1 the API key should pass through")
}

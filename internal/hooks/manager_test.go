package hooks

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// mgr validates cfg (asserts no warnings) and builds a Manager with fixed test options.
func mgr(t *testing.T, cfg Config) *Manager {
	t.Helper()
	warnings := cfg.Validate()
	require.Empty(t, warnings, "cfg.Validate() must produce no warnings")
	return NewManager(cfg, ManagerOptions{
		SessionID: "test-session",
		Cwd:       "/test",
		Version:   "0.4.0-dev",
	})
}

// TestManager_FirePreToolUse_MatchAndAllow: Bash hook (allow.sh) fires; Write hook
// (deny.sh) does NOT fire because its matcher doesn't match "Bash".
// allow.sh exits 0 with no JSON → no opinion → DecisionPass.
func TestManager_FirePreToolUse_MatchAndAllow(t *testing.T) {
	cfg := Config{
		PreToolUse: []Spec{
			{Matcher: "Bash", Command: filepath.Join("testdata", "allow.sh"), Timeout: 3 * time.Second},
			{Matcher: "Write", Command: filepath.Join("testdata", "deny.sh"), Timeout: 3 * time.Second},
		},
	}
	m := mgr(t, cfg)
	dec := m.FirePreToolUse(context.Background(), "Bash", map[string]any{"command": "ls"})
	// allow.sh exits 0 with no JSON output → no permissionDecision → Pass
	require.Equal(t, DecisionPass, dec.Behavior)
	require.Empty(t, dec.Reason)
	require.Nil(t, dec.ModifiedInput)
}

// TestManager_FirePreToolUse_DenyShortCircuits: deny.sh fires first and exits 2,
// so we get DecisionDeny immediately and allow.sh never runs.
func TestManager_FirePreToolUse_DenyShortCircuits(t *testing.T) {
	cfg := Config{
		PreToolUse: []Spec{
			{Command: filepath.Join("testdata", "deny.sh"), Timeout: 3 * time.Second},
			{Command: filepath.Join("testdata", "allow.sh"), Timeout: 3 * time.Second},
		},
	}
	m := mgr(t, cfg)
	dec := m.FirePreToolUse(context.Background(), "Bash", map[string]any{})
	require.Equal(t, DecisionDeny, dec.Behavior)
	require.Contains(t, dec.Reason, "denied by policy")
}

// TestManager_FirePreToolUse_ModifiedInputApplied: mutate-input.sh outputs
// modifiedInput with command "ls -al".
func TestManager_FirePreToolUse_ModifiedInputApplied(t *testing.T) {
	cfg := Config{
		PreToolUse: []Spec{
			{Command: filepath.Join("testdata", "mutate-input.sh"), Timeout: 3 * time.Second},
		},
	}
	m := mgr(t, cfg)
	dec := m.FirePreToolUse(context.Background(), "Bash", map[string]any{"command": "ls"})
	require.NotNil(t, dec.ModifiedInput)
	require.Equal(t, "ls -al", dec.ModifiedInput["command"])
}

// TestManager_FirePreToolUse_NoMatchersNoHooks: empty config → DecisionPass.
func TestManager_FirePreToolUse_NoMatchersNoHooks(t *testing.T) {
	m := mgr(t, Config{})
	dec := m.FirePreToolUse(context.Background(), "Bash", map[string]any{})
	require.Equal(t, DecisionPass, dec.Behavior)
	require.Empty(t, dec.Reason)
	require.Nil(t, dec.ModifiedInput)
}

// TestManager_FireUserPromptSubmit_AdditionalContextConcat: two inject-context.sh hooks
// each append "injected ctx"; final prompt is "hello\n\ninjected ctx\n\ninjected ctx".
func TestManager_FireUserPromptSubmit_AdditionalContextConcat(t *testing.T) {
	cfg := Config{
		UserPromptSubmit: []Spec{
			{Command: filepath.Join("testdata", "inject-context.sh"), Timeout: 3 * time.Second},
			{Command: filepath.Join("testdata", "inject-context.sh"), Timeout: 3 * time.Second},
		},
	}
	m := mgr(t, cfg)
	_, finalPrompt, abort, _ := m.FireUserPromptSubmit(context.Background(), "hello")
	require.False(t, abort)
	require.Equal(t, "hello\n\ninjected ctx\n\ninjected ctx", finalPrompt)
}

// TestManager_FireUserPromptSubmit_DenyAborts: deny.sh exits 2 → abort=true.
func TestManager_FireUserPromptSubmit_DenyAborts(t *testing.T) {
	cfg := Config{
		UserPromptSubmit: []Spec{
			{Command: filepath.Join("testdata", "deny.sh"), Timeout: 3 * time.Second},
		},
	}
	m := mgr(t, cfg)
	_, _, abort, reason := m.FireUserPromptSubmit(context.Background(), "hello")
	require.True(t, abort)
	require.Contains(t, reason, "denied by policy")
}

// TestManager_FireStopAsync_NonBlocking: FireStop with slow.sh must return before
// the hook finishes (the goroutine runs the hook, not the caller).
func TestManager_FireStopAsync_NonBlocking(t *testing.T) {
	cfg := Config{
		Stop: []Spec{
			{Command: filepath.Join("testdata", "slow.sh"), Timeout: 100 * time.Millisecond},
		},
	}
	m := mgr(t, cfg)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		m.FireStop(context.Background(), "end_turn")
	}()

	// FireStop should return nearly instantly (goroutine fires asynchronously).
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// good — returned quickly
	case <-time.After(500 * time.Millisecond):
		t.Fatal("FireStop blocked longer than expected")
	}
}

// TestManager_FirePreToolUse_Timeout_ReturnsDeny: slow.sh with 100ms timeout
// should cause DecisionDeny with "timeout" in the reason.
func TestManager_FirePreToolUse_Timeout_ReturnsDeny(t *testing.T) {
	cfg := Config{
		PreToolUse: []Spec{
			{Matcher: "Bash", Command: filepath.Join("testdata", "slow.sh"), Timeout: 100 * time.Millisecond},
		},
	}
	m := mgr(t, cfg)
	dec := m.FirePreToolUse(context.Background(), "Bash", map[string]any{})
	require.Equal(t, DecisionDeny, dec.Behavior)
	require.Contains(t, dec.Reason, "timeout")
}

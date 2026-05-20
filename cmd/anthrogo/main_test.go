package main

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/internal/hooks"
)

func TestSkipRelativeHookPaths_RemovesRelative(t *testing.T) {
	var warnings []string
	logSink := func(event, msg string) {
		warnings = append(warnings, event+": "+msg)
	}

	cfg := hooks.Config{
		PreToolUse: []hooks.Spec{
			{Command: "./local/script.sh", Matcher: "Bash"},
			{Command: "/usr/local/bin/abs.sh", Matcher: "Read"},
		},
		PostToolUse: []hooks.Spec{
			{Command: "relative/hook.sh"},
			{Command: "/abs/hook.sh"},
		},
	}

	skipRelativeHookPaths(&cfg, logSink)

	// Only absolute paths remain.
	require.Len(t, cfg.PreToolUse, 1)
	require.Equal(t, "/usr/local/bin/abs.sh", cfg.PreToolUse[0].Command)

	require.Len(t, cfg.PostToolUse, 1)
	require.Equal(t, "/abs/hook.sh", cfg.PostToolUse[0].Command)

	// Warnings emitted for each skipped spec.
	require.Len(t, warnings, 2)
	require.Contains(t, warnings[0], "relative path")
	require.Contains(t, warnings[0], "./local/script.sh")
}

func TestSkipRelativeHookPaths_AllAbsolutePassThrough(t *testing.T) {
	var warnings []string
	logSink := func(event, msg string) {
		warnings = append(warnings, msg)
	}

	cfg := hooks.Config{
		PreToolUse: []hooks.Spec{
			{Command: "/bin/hook.sh"},
		},
	}

	skipRelativeHookPaths(&cfg, logSink)

	require.Len(t, cfg.PreToolUse, 1, "absolute paths must pass through unchanged")
	require.Empty(t, warnings, "no warnings expected for absolute paths")
}

func TestSkipRelativeHookPaths_NilLogSinkNocrash(t *testing.T) {
	cfg := hooks.Config{
		PreToolUse: []hooks.Spec{
			{Command: "relative/cmd"},
		},
	}
	// Must not panic with nil logSink.
	require.NotPanics(t, func() {
		skipRelativeHookPaths(&cfg, nil)
	})
	require.Empty(t, cfg.PreToolUse)
}

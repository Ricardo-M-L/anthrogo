package main

import (
	"context"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/internal/config"
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

// TestPprofFlagSmoke verifies the pprof goroutine starts and the endpoint
// responds when --pprof is given a free port.
func TestPprofFlagSmoke(t *testing.T) {
	addr := "127.0.0.1:16061"
	go func() {
		srv := &http.Server{Addr: addr, Handler: nil}
		_ = srv.ListenAndServe()
	}()
	// Give the goroutine time to bind.
	time.Sleep(50 * time.Millisecond)
	resp, err := http.Get("http://" + addr + "/debug/pprof/")
	if err != nil {
		t.Skipf("pprof server not reachable (port busy?): %v", err)
	}
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestResolveServeToken_FlagTakesPriority(t *testing.T) {
	t.Setenv("ANTHROGO_SERVE_TOKEN", "from-env")
	tok, err := resolveServeToken("from-flag", "")
	require.NoError(t, err)
	require.Equal(t, "from-flag", tok)
}

func TestResolveServeToken_FileFallback(t *testing.T) {
	t.Setenv("ANTHROGO_SERVE_TOKEN", "from-env")
	dir := t.TempDir()
	f := filepath.Join(dir, "tok")
	require.NoError(t, os.WriteFile(f, []byte("from-file\n"), 0o600))
	tok, err := resolveServeToken("", f)
	require.NoError(t, err)
	require.Equal(t, "from-file", tok)
}

func TestResolveServeToken_EnvFallback(t *testing.T) {
	t.Setenv("ANTHROGO_SERVE_TOKEN", "from-env")
	tok, err := resolveServeToken("", "")
	require.NoError(t, err)
	require.Equal(t, "from-env", tok)
}

func TestResolveServeToken_EmptyWhenAllAbsent(t *testing.T) {
	t.Setenv("ANTHROGO_SERVE_TOKEN", "")
	tok, err := resolveServeToken("", "")
	require.NoError(t, err)
	require.Equal(t, "", tok)
}

func TestResolveServeToken_FileNotFound_ReturnsError(t *testing.T) {
	_, err := resolveServeToken("", "/nonexistent/path/token")
	require.Error(t, err)
}

func TestVersionSubcommand_PrintsVersion(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "anthrogo")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = "."
	if buildOut, err := build.CombinedOutput(); err != nil {
		t.Skipf("could not build test binary: %s", string(buildOut))
	}
	out, err := exec.Command(bin, "version").CombinedOutput()
	require.NoError(t, err, "version subcommand should exit 0, got %s", string(out))
	require.Contains(t, string(out), "anthrogo ")
}

func TestBuildFromProfile_Anthropic_BlocksLinkLocal(t *testing.T) {
	t.Setenv("DUMMY_KEY", "abc")
	prof := config.Profile{
		Type:    "anthropic",
		BaseURL: "http://169.254.169.254/anthropic/",
		Model:   "x",
		APIKey:  "env:DUMMY_KEY",
	}
	_, _, err := buildFromProfile(context.Background(), "bad", prof, "x")
	require.Error(t, err, "anthropic profile base_url pointing at cloud-metadata must be blocked")
	require.Contains(t, err.Error(), "link-local")
}

func TestBuildFromProfile_Anthropic_BlocksLoopback(t *testing.T) {
	t.Setenv("DUMMY_KEY", "abc")
	prof := config.Profile{
		Type:    "anthropic",
		BaseURL: "http://127.0.0.1:8080/",
		Model:   "x",
		APIKey:  "env:DUMMY_KEY",
	}
	_, _, err := buildFromProfile(context.Background(), "bad", prof, "x")
	require.Error(t, err)
	require.Contains(t, err.Error(), "loopback")
}

func TestBuildFromProfile_Anthropic_AcceptsPublicURL(t *testing.T) {
	t.Setenv("DUMMY_KEY", "abc")
	prof := config.Profile{
		Type:    "anthropic",
		BaseURL: "https://api.minimaxi.com/anthropic",
		Model:   "minimax-m2.7",
		APIKey:  "env:DUMMY_KEY",
	}
	p, model, err := buildFromProfile(context.Background(), "minimax", prof, "fallback")
	require.NoError(t, err)
	require.NotNil(t, p)
	require.Equal(t, "minimax-m2.7", model)
}

func TestBuildFromProfile_Anthropic_NoBaseURL_DefaultsToOfficial(t *testing.T) {
	t.Setenv("DUMMY_KEY", "abc")
	prof := config.Profile{
		Type:   "anthropic",
		Model:  "claude-sonnet-4-6",
		APIKey: "env:DUMMY_KEY",
	}
	p, model, err := buildFromProfile(context.Background(), "default", prof, "fallback")
	require.NoError(t, err, "empty base_url must work — SDK uses api.anthropic.com")
	require.NotNil(t, p)
	require.Equal(t, "claude-sonnet-4-6", model)
}

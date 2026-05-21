package doctor

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/internal/config"
)

func TestCheckGoRuntime(t *testing.T) {
	c := checkGoRuntime()
	require.Equal(t, SeverityPass, c.Severity)
	require.Contains(t, c.Message, runtime.Version())
}

func TestCheckAPIKey_NoneSet(t *testing.T) {
	// Clear all known API key env vars
	for _, k := range []string{"ANTHROPIC_API_KEY", "DEEPSEEK_API_KEY", "KIMI_API_KEY", "MINIMAX_API_KEY", "GLM_API_KEY", "OPENAI_API_KEY", "GITHUB_TOKEN"} {
		t.Setenv(k, "")
	}
	checks := checkAPIKey(config.Config{})
	require.Len(t, checks, 1)
	require.Equal(t, SeverityFail, checks[0].Severity)
}

func TestCheckAPIKey_AnthropicSet(t *testing.T) {
	for _, k := range []string{"ANTHROPIC_API_KEY", "DEEPSEEK_API_KEY", "KIMI_API_KEY", "MINIMAX_API_KEY", "GLM_API_KEY", "OPENAI_API_KEY", "GITHUB_TOKEN"} {
		t.Setenv(k, "")
	}
	t.Setenv("ANTHROPIC_API_KEY", "test-key")
	checks := checkAPIKey(config.Config{})
	require.Equal(t, SeverityPass, checks[0].Severity)
}

func TestCheckBinaries_ReturnsResultsForAll(t *testing.T) {
	checks := checkBinaries()
	require.GreaterOrEqual(t, len(checks), 5)
}

func TestFormat_RendersSummary(t *testing.T) {
	checks := []Check{
		{Name: "A", Severity: SeverityPass, Message: "ok"},
		{Name: "B", Severity: SeverityWarn, Message: "warn"},
		{Name: "C", Severity: SeverityFail, Message: "fail", Remediation: "fix it"},
	}
	out := Format(checks)
	require.Contains(t, out, "1 PASS")
	require.Contains(t, out, "1 WARN")
	require.Contains(t, out, "1 FAIL")
	require.Contains(t, out, "→ fix it")
}

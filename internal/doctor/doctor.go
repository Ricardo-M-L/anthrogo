package doctor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/ricardo/anthrogo/internal/config"
	"github.com/ricardo/anthrogo/internal/version"
)

// Severity describes the outcome level of a health check.
type Severity string

const (
	SeverityPass Severity = "PASS"
	SeverityWarn Severity = "WARN"
	SeverityFail Severity = "FAIL"
)

// Check holds the result of a single health check.
type Check struct {
	Name        string
	Severity    Severity
	Message     string
	Remediation string
}

// RunAll executes all health checks and returns their results.
func RunAll(ctx context.Context, cfg config.Config) []Check {
	var out []Check
	out = append(out, checkGoRuntime())
	out = append(out, checkConfigFile())
	out = append(out, checkAPIKey(cfg)...)
	out = append(out, checkHomeDir())
	out = append(out, checkBinaries()...)
	out = append(out, checkNetworkConnectivity(ctx)...)
	out = append(out, checkVersion(ctx))
	return out
}

func checkGoRuntime() Check {
	return Check{
		Name:     "Go runtime",
		Severity: SeverityPass,
		Message:  fmt.Sprintf("running on %s (%s/%s)", runtime.Version(), runtime.GOOS, runtime.GOARCH),
	}
}

func checkConfigFile() Check {
	path, err := config.SettingsPath()
	if err != nil {
		return Check{Name: "Settings file", Severity: SeverityWarn, Message: "could not resolve settings path: " + err.Error()}
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return Check{Name: "Settings file", Severity: SeverityWarn, Message: "no settings.yaml at " + path, Remediation: "Create one with `anthrogo init-config` or copy the example from README."}
		}
		return Check{Name: "Settings file", Severity: SeverityFail, Message: "stat error: " + err.Error()}
	}
	return Check{Name: "Settings file", Severity: SeverityPass, Message: "found at " + path}
}

func checkAPIKey(cfg config.Config) []Check {
	var out []Check
	keys := map[string]string{
		"ANTHROPIC_API_KEY": "anthropic",
		"DEEPSEEK_API_KEY":  "deepseek",
		"KIMI_API_KEY":      "kimi",
		"MINIMAX_API_KEY":   "minimax",
		"GLM_API_KEY":       "glm",
		"OPENAI_API_KEY":    "openai",
		"GITHUB_TOKEN":      "github (optional, for update check rate limit)",
	}
	for env, label := range keys {
		if os.Getenv(env) != "" {
			out = append(out, Check{Name: "API key: " + env, Severity: SeverityPass, Message: "set (" + label + ")"})
		}
	}
	if len(out) == 0 {
		out = append(out, Check{
			Name:        "API keys",
			Severity:    SeverityFail,
			Message:     "no provider API keys set",
			Remediation: "Set at least ANTHROPIC_API_KEY (or another provider's key) in your shell environment.",
		})
	}
	return out
}

func checkHomeDir() Check {
	home, err := os.UserHomeDir()
	if err != nil {
		return Check{Name: "Anthrogo home", Severity: SeverityFail, Message: err.Error()}
	}
	dir := home + "/.anthrogo"
	if info, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return Check{Name: "Anthrogo home", Severity: SeverityWarn, Message: "directory missing: " + dir, Remediation: "Created automatically on first run."}
		}
		return Check{Name: "Anthrogo home", Severity: SeverityFail, Message: err.Error()}
	} else if !info.IsDir() {
		return Check{Name: "Anthrogo home", Severity: SeverityFail, Message: dir + " exists but is not a directory"}
	}
	return Check{Name: "Anthrogo home", Severity: SeverityPass, Message: dir + " ok"}
}

func checkBinaries() []Check {
	binaries := []struct {
		Name     string
		Required bool
		Purpose  string
	}{
		{"git", false, "Diff/Git tools, plugin install"},
		{"sh", true, "Bash tool (POSIX shell)"},
		{"docker", false, "ContainerExec tool"},
		{"podman", false, "ContainerExec fallback"},
		{"whisper", false, "SpeechToText tool"},
		{"say", false, "TextToSpeech on macOS"},
		{"espeak", false, "TextToSpeech on Linux"},
		{"gofmt", false, "Format tool (.go files)"},
		{"prettier", false, "Format tool (.js/.ts/.css/.json/.md)"},
		{"black", false, "Format tool (.py)"},
		{"rustfmt", false, "Format tool (.rs)"},
	}
	var out []Check
	for _, b := range binaries {
		path, err := exec.LookPath(b.Name)
		if err != nil {
			sev := SeverityWarn
			if b.Required {
				sev = SeverityFail
			}
			out = append(out, Check{
				Name:     "Binary: " + b.Name,
				Severity: sev,
				Message:  "not on PATH (" + b.Purpose + ")",
			})
			continue
		}
		out = append(out, Check{
			Name:     "Binary: " + b.Name,
			Severity: SeverityPass,
			Message:  path,
		})
	}
	return out
}

func checkNetworkConnectivity(ctx context.Context) []Check {
	var out []Check
	endpoints := map[string]string{
		"Anthropic API": "https://api.anthropic.com",
		"GitHub API":    "https://api.github.com",
	}
	for name, url := range endpoints {
		ctxT, cancel := context.WithTimeout(ctx, 5*time.Second)
		cmd := exec.CommandContext(ctxT, "curl", "-sf", "-o", "/dev/null", "-w", "%{http_code}", url)
		outBytes, err := cmd.Output()
		cancel()
		if err != nil {
			out = append(out, Check{
				Name:     name,
				Severity: SeverityWarn,
				Message:  "unreachable (or curl not on PATH): " + err.Error(),
			})
			continue
		}
		code := strings.TrimSpace(string(outBytes))
		if code == "" || code[0] == '5' {
			out = append(out, Check{Name: name, Severity: SeverityWarn, Message: "got HTTP " + code})
			continue
		}
		out = append(out, Check{Name: name, Severity: SeverityPass, Message: "HTTP " + code})
	}
	return out
}

func checkVersion(_ context.Context) Check {
	return Check{
		Name:     "anthrogo version",
		Severity: SeverityPass,
		Message:  version.Version,
	}
}

// Format renders checks as a human-readable report.
func Format(checks []Check) string {
	var pass, warn, fail int
	var b strings.Builder
	for _, c := range checks {
		symbol := "✓"
		switch c.Severity {
		case SeverityWarn:
			symbol = "!"
			warn++
		case SeverityFail:
			symbol = "✗"
			fail++
		case SeverityPass:
			pass++
		}
		fmt.Fprintf(&b, "[%s] %-30s  %s\n", symbol, c.Name, c.Message)
		if c.Remediation != "" {
			fmt.Fprintf(&b, "        → %s\n", c.Remediation)
		}
	}
	fmt.Fprintf(&b, "\nSummary: %d PASS, %d WARN, %d FAIL\n", pass, warn, fail)
	return b.String()
}

package tool

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/ricardo/anthrogo/pkg/bashscan"
)

// Bash executes a shell command. M1 has no sandbox, no AST security, no
// background tasks — those are deferred to M5 alongside the upstream
// bash{Permissions,Security,Sandbox} modules.
//
// M10.2 adds opt-in lightweight sandboxing via sandbox:true (see Schema).
type Bash struct{ DefaultPermission }

func (Bash) Name() string                       { return "Bash" }
func (Bash) Description(context.Context) string { return bashDescription }
func (Bash) UserFacingName(input map[string]any) string {
	if c, ok := input["command"].(string); ok {
		if len(c) > 60 {
			return "Bash " + c[:60] + "…"
		}
		return "Bash " + c
	}
	return "Bash"
}
func (Bash) IsReadOnly() bool        { return false }
func (Bash) IsConcurrencySafe() bool { return false }

func (Bash) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command":    map[string]any{"type": "string"},
			"timeout_ms": map[string]any{"type": "integer", "default": 120000},
			"sandbox": map[string]any{
				"type":        "boolean",
				"description": "Restrict cwd, PATH, and strip sensitive env vars.",
			},
		},
		"required": []string{"command"},
	}
}

func (Bash) Call(parent context.Context, input map[string]any, tcx *Context) (Result, error) {
	cmd, _ := input["command"].(string)
	if cmd == "" {
		return errResult("command is required"), nil
	}
	timeoutMS := intField(input, "timeout_ms", 120_000)
	ctx, cancel := context.WithTimeout(parent, time.Duration(timeoutMS)*time.Millisecond)
	defer cancel()

	sandbox, _ := input["sandbox"].(bool)
	if sandbox {
		if err := validateSandboxedCommand(cmd); err != nil {
			msg := err.Error()
			return Result{Type: ResultText, Text: msg, ForLLM: msg, IsError: true}, nil
		}
	}

	shell, flag := defaultShell()
	c := exec.CommandContext(ctx, shell, flag, cmd)
	if tcx != nil && tcx.Cwd != "" {
		c.Dir = tcx.Cwd
	}
	if sandbox {
		c.Env = sandboxedEnv()
	}
	var out, errb bytes.Buffer
	c.Stdout = &out
	c.Stderr = &errb
	err := c.Run()
	combined := out.String() + errb.String()

	if ctx.Err() == context.DeadlineExceeded {
		msg := fmt.Sprintf("timeout after %dms\n%s", timeoutMS, combined)
		return Result{Type: ResultText, Text: msg, ForLLM: msg, IsError: true}, nil
	}
	if err != nil {
		msg := fmt.Sprintf("%s\n%s", err.Error(), combined)
		return Result{Type: ResultText, Text: msg, ForLLM: msg, IsError: true}, nil
	}
	return Result{Type: ResultText, Text: combined, ForLLM: combined}, nil
}

func defaultShell() (string, string) {
	if runtime.GOOS == "windows" {
		return "powershell.exe", "-Command"
	}
	return "/bin/bash", "-c"
}

// sandboxPATH is the restricted PATH used when sandbox=true.
const sandboxPATH = "/usr/bin:/bin:/usr/sbin:/sbin"

// sandboxEnvDenylist contains exact env var names to strip in sandbox mode.
var sandboxEnvDenylist = []string{
	"HOME",
	"SSH_AUTH_SOCK", "SSH_CONNECTION",
}

// sandboxEnvDenyPrefixes contains prefixes: any env var whose name starts with
// one of these is stripped in sandbox mode.
var sandboxEnvDenyPrefixes = []string{
	"SSH_", "AWS_", "GCP_", "AZURE_",
	"GOOGLE_", "GITHUB_", "GITLAB_",
	"ANTHROPIC_API_KEY", "OPENAI_API_KEY",
	"DEEPSEEK_API_KEY", "KIMI_API_KEY", "MINIMAX_API_KEY", "GLM_API_KEY",
	"KAIROS_AUTH_TOKEN",
}

// forbiddenBinaries is the denylist checked by the AST scan in sandbox mode.
var forbiddenBinaries = []string{
	"sudo", "doas", "rm", "dd", "mkfs", "mount", "umount",
	"chmod", "chown", "chroot", "setuid", "setgid",
}

// sandboxForbiddenSubstrings are path/pattern heuristics kept as a second
// layer of defence (M10.2 originals).
var sandboxForbiddenSubstrings = []string{
	"../", "..\\",
	"~/.ssh", "~/.aws", "~/.gnupg", "~/.kube",
	"/etc/passwd", "/etc/shadow", "/private/etc/",
	"/var/log", "/proc/", "/sys/",
}

// validateSandboxedCommand uses AST-based binary detection (M10.10) layered
// on top of the M10.2 substring denylist.
func validateSandboxedCommand(cmdString string) error {
	scan := bashscan.Scan(cmdString)
	if scan.ParseError != "" {
		// Conservative: parse failure → reject when in sandbox mode.
		return fmt.Errorf("sandbox: shell parse failed: %s", scan.ParseError)
	}
	// Block dangerous binaries.
	if scan.HasBinary(forbiddenBinaries...) {
		return fmt.Errorf("sandbox: command invokes forbidden binary in %v", scan.Binaries)
	}
	if scan.UsesSudo {
		return fmt.Errorf("sandbox: sudo not allowed")
	}
	// Still apply M10.2 substring path heuristics as a second layer.
	return validateSandboxedSubstrings(cmdString)
}

// validateSandboxedSubstrings keeps the M10.2 substring checks as a fallback.
func validateSandboxedSubstrings(cmdString string) error {
	lower := strings.ToLower(cmdString)
	for _, sub := range sandboxForbiddenSubstrings {
		if strings.Contains(lower, sub) {
			return fmt.Errorf("sandbox: command contains forbidden path/substring %q", sub)
		}
	}
	return nil
}

// sandboxedEnv builds an env slice from the current process env, dropping
// vars that match sandboxEnvDenylist or sandboxEnvDenyPrefixes, then forces
// PATH to sandboxPATH.
func sandboxedEnv() []string {
	base := os.Environ()
	out := make([]string, 0, len(base))
	for _, kv := range base {
		idx := strings.IndexByte(kv, '=')
		if idx < 0 {
			continue
		}
		key := kv[:idx]
		if isDeniedEnv(key) {
			continue
		}
		out = append(out, kv)
	}
	out = append(out, "PATH="+sandboxPATH)
	return out
}

// isDeniedEnv reports whether the given env var name should be stripped in
// sandbox mode.
func isDeniedEnv(name string) bool {
	for _, exact := range sandboxEnvDenylist {
		if name == exact {
			return true
		}
	}
	for _, prefix := range sandboxEnvDenyPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	// Always strip PATH — we will re-add sandboxPATH unconditionally.
	return name == "PATH"
}

const bashDescription = `Run a shell command. Use timeout_ms to cap execution time (default 120000). Set sandbox:true for opt-in lightweight sandboxing (restricted PATH, env stripping, heuristic path denylist). Returns combined stdout+stderr. Non-zero exit codes and timeouts surface as is_error tool_result blocks.`

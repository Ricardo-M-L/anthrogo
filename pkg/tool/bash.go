package tool

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"time"
)

// Bash executes a shell command. M1 has no sandbox, no AST security, no
// background tasks — those are deferred to M5 alongside the upstream
// bash{Permissions,Security,Sandbox} modules.
type Bash struct{ DefaultPermission }

func (Bash) Name() string                      { return "Bash" }
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

	shell, flag := defaultShell()
	c := exec.CommandContext(ctx, shell, flag, cmd)
	if tcx != nil && tcx.Cwd != "" {
		c.Dir = tcx.Cwd
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

const bashDescription = `Run a shell command. Use timeout_ms to cap execution time (default 120000). Returns combined stdout+stderr. Non-zero exit codes and timeouts surface as is_error tool_result blocks.`

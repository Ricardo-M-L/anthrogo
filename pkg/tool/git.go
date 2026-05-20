package tool

import (
	"bytes"
	"context"
	"strings"

	"os/exec"
)

// gitAllowedCommands is the allowlist of read-only git subcommands.
var gitAllowedCommands = map[string]bool{
	"status": true,
	"log":    true,
	"branch": true,
	"show":   true,
	"blame":  true,
	"remote": true,
}

// gitMetachars are the shell metacharacters we reject in args.
var gitMetachars = []string{";", "&&", "||", "|", ">", "<", "`"}

// Git exposes a read-only subset of git subcommands.
type Git struct{ DefaultPermission }

func (Git) Name() string                      { return "Git" }
func (Git) Description(context.Context) string { return gitDescription }
func (Git) IsReadOnly() bool                  { return true }
func (Git) IsConcurrencySafe() bool           { return true }

func (Git) UserFacingName(input map[string]any) string {
	cmd, _ := input["command"].(string)
	args, _ := input["args"].(string)
	if args != "" {
		return "Git " + cmd + " " + args
	}
	if cmd != "" {
		return "Git " + cmd
	}
	return "Git"
}

func (Git) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type": "string",
				"enum": []string{"status", "log", "branch", "show", "blame", "remote"},
				"description": "Which git subcommand. Only read-only ones allowed.",
			},
			"args": map[string]any{
				"type":        "string",
				"description": "Extra args (e.g. '-10' for log, 'HEAD~3' for show).",
			},
		},
		"required": []string{"command"},
	}
}

func (Git) Call(_ context.Context, input map[string]any, tcx *Context) (Result, error) {
	subcmd, ok := input["command"].(string)
	if !ok || subcmd == "" {
		return errResult("command is required"), nil
	}
	if !gitAllowedCommands[subcmd] {
		return errResult("git subcommand not allowed: " + subcmd + ". Allowed: status, log, branch, show, blame, remote"), nil
	}

	rawArgs, _ := input["args"].(string)
	rawArgs = strings.TrimSpace(rawArgs)

	// Reject shell metacharacters in args.
	for _, meta := range gitMetachars {
		if strings.Contains(rawArgs, meta) {
			return errResult("args contains disallowed shell metacharacter: " + meta), nil
		}
	}

	// Build argv: git <subcmd> [args...]
	argv := []string{subcmd}
	if rawArgs != "" {
		argv = append(argv, strings.Fields(rawArgs)...)
	}

	cmd := exec.Command("git", argv...)
	if tcx != nil && tcx.Cwd != "" {
		cmd.Dir = tcx.Cwd
	}
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb

	err := cmd.Run()
	combined := out.String()
	if err != nil {
		msg := strings.TrimSpace(errb.String())
		if msg == "" {
			msg = err.Error()
		}
		return errResult(msg), nil
	}

	return Result{Type: ResultText, Text: combined, ForLLM: combined}, nil
}

const gitDescription = `Run a read-only git subcommand (status, log, branch, show, blame, remote). Args are sanitized against shell metacharacters. Use Bash for write operations (commit, push, reset, etc.).`

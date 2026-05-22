package tool

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ricardo/anthrogo/pkg/bgtasks"
)

// BackgroundLaunch launches a shell command as a background task.
type BackgroundLaunch struct {
	DefaultPermission
	Manager *bgtasks.Manager
}

func (*BackgroundLaunch) Name() string { return "BackgroundLaunch" }
func (*BackgroundLaunch) Description(context.Context) string {
	return "Launch a shell command as a background task; returns a task ID. Use BackgroundStatus / BackgroundOutput / BackgroundCancel to interact."
}
func (*BackgroundLaunch) UserFacingName(input map[string]any) string {
	if c, _ := input["command"].(string); c != "" {
		return "BackgroundLaunch: " + truncStr(c, 60)
	}
	return "BackgroundLaunch"
}
func (*BackgroundLaunch) IsReadOnly() bool        { return false }
func (*BackgroundLaunch) IsConcurrencySafe() bool { return true }
func (*BackgroundLaunch) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "Shell command to run in the background via sh -c.",
			},
		},
		"required": []string{"command"},
	}
}
func (b *BackgroundLaunch) Call(_ context.Context, input map[string]any, _ *Context) (Result, error) {
	cmd, _ := input["command"].(string)
	if cmd == "" {
		return errResult("command is required"), nil
	}
	if b.Manager == nil {
		return errResult("background manager not configured"), nil
	}
	id := b.Manager.Launch(cmd)
	msg := fmt.Sprintf("launched: task_id=%s", id)
	return Result{Type: ResultText, Text: msg, ForLLM: msg, Data: map[string]any{"task_id": id}}, nil
}

// BackgroundStatus returns status of one task or lists all tasks.
type BackgroundStatus struct {
	DefaultPermission
	Manager *bgtasks.Manager
}

func (*BackgroundStatus) Name() string { return "BackgroundStatus" }
func (*BackgroundStatus) Description(context.Context) string {
	return "Get current status of one background task or all tasks."
}
func (*BackgroundStatus) UserFacingName(_ map[string]any) string { return "BackgroundStatus" }
func (*BackgroundStatus) IsReadOnly() bool                       { return true }
func (*BackgroundStatus) IsConcurrencySafe() bool               { return true }
func (*BackgroundStatus) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"task_id": map[string]any{
				"type":        "string",
				"description": "Omit to list all tasks.",
			},
		},
	}
}
func (b *BackgroundStatus) Call(_ context.Context, input map[string]any, _ *Context) (Result, error) {
	if b.Manager == nil {
		return errResult("manager not configured"), nil
	}
	id, _ := input["task_id"].(string)
	if id == "" {
		ids := b.Manager.List()
		var lines []string
		for _, tid := range ids {
			if t, ok := b.Manager.Get(tid); ok {
				lines = append(lines, fmt.Sprintf("%s  %-10s  %s", tid, t.Status, truncStr(t.Command, 50)))
			}
		}
		out := strings.Join(lines, "\n")
		if out == "" {
			out = "(no background tasks)"
		}
		return Result{Type: ResultText, Text: out, ForLLM: out}, nil
	}
	t, ok := b.Manager.Get(id)
	if !ok {
		return errResult("no such task: " + id), nil
	}
	out := fmt.Sprintf("task=%s\nstatus=%s\nexit_code=%d\ncommand=%s\nstarted=%s\nfinished=%s\n",
		t.ID, t.Status, t.ExitCode, t.Command,
		t.StartedAt.Format(time.RFC3339),
		t.FinishedAt.Format(time.RFC3339),
	)
	return Result{Type: ResultText, Text: out, ForLLM: out}, nil
}

// BackgroundOutput fetches captured stdout and stderr for a task.
type BackgroundOutput struct {
	DefaultPermission
	Manager *bgtasks.Manager
}

func (*BackgroundOutput) Name() string { return "BackgroundOutput" }
func (*BackgroundOutput) Description(context.Context) string {
	return "Fetch captured stdout and stderr for a background task."
}
func (*BackgroundOutput) UserFacingName(_ map[string]any) string { return "BackgroundOutput" }
func (*BackgroundOutput) IsReadOnly() bool                       { return true }
func (*BackgroundOutput) IsConcurrencySafe() bool               { return true }
func (*BackgroundOutput) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"task_id": map[string]any{"type": "string"},
		},
		"required": []string{"task_id"},
	}
}
func (b *BackgroundOutput) Call(_ context.Context, input map[string]any, _ *Context) (Result, error) {
	if b.Manager == nil {
		return errResult("manager not configured"), nil
	}
	id, _ := input["task_id"].(string)
	if id == "" {
		return errResult("task_id is required"), nil
	}
	t, ok := b.Manager.Get(id)
	if !ok {
		return errResult("no such task: " + id), nil
	}
	var sb strings.Builder
	if stdout := t.Stdout.String(); stdout != "" {
		sb.WriteString("--- stdout ---\n")
		sb.WriteString(stdout)
	}
	if stderr := t.Stderr.String(); stderr != "" {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("--- stderr ---\n")
		sb.WriteString(stderr)
	}
	out := sb.String()
	if out == "" {
		out = "(no output yet)"
	}
	return Result{Type: ResultText, Text: out, ForLLM: out}, nil
}

// BackgroundCancel cancels a running background task.
type BackgroundCancel struct {
	DefaultPermission
	Manager *bgtasks.Manager
}

func (*BackgroundCancel) Name() string { return "BackgroundCancel" }
func (*BackgroundCancel) Description(context.Context) string {
	return "Cancel a running background task by task ID."
}
func (*BackgroundCancel) UserFacingName(_ map[string]any) string { return "BackgroundCancel" }
func (*BackgroundCancel) IsReadOnly() bool                       { return false }
func (*BackgroundCancel) IsConcurrencySafe() bool               { return true }
func (*BackgroundCancel) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"task_id": map[string]any{"type": "string"},
		},
		"required": []string{"task_id"},
	}
}
func (b *BackgroundCancel) Call(_ context.Context, input map[string]any, _ *Context) (Result, error) {
	if b.Manager == nil {
		return errResult("manager not configured"), nil
	}
	id, _ := input["task_id"].(string)
	if id == "" {
		return errResult("task_id is required"), nil
	}
	if err := b.Manager.Cancel(id); err != nil {
		return errResult(err.Error()), nil
	}
	msg := fmt.Sprintf("task %s canceled", id)
	return Result{Type: ResultText, Text: msg, ForLLM: msg}, nil
}

// truncStr truncates s to at most n bytes, replacing newlines, and appends
// an ellipsis when truncated. Defined here; used by BackgroundStatus as well.
func truncStr(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

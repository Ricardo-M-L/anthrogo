package tool

import (
	"context"
	"fmt"
	"strings"

	"github.com/ricardo/anthrogo/pkg/subagent"
)

// Task is a built-in tool that spawns a subagent engine to perform a
// self-contained multi-step task and returns its final assistant text.
type Task struct {
	DefaultPermission
	runner func(ctx context.Context, opts TaskOptions) (string, error)
	reg    *subagent.Registry
}

// TaskOptions carries the parsed input for a Task tool call.
type TaskOptions struct {
	Description  string
	Prompt       string
	SubagentType string
}

// NewTask creates a Task tool wired with the given subagent registry and runner.
// The runner is typically engine.RunSubagent, injected to avoid an import cycle.
func NewTask(reg *subagent.Registry, runner func(context.Context, TaskOptions) (string, error)) *Task {
	return &Task{reg: reg, runner: runner}
}

// Name implements Tool.
func (*Task) Name() string { return "Task" }

// Description lists available subagent types so the model knows what to pass.
func (t *Task) Description(_ context.Context) string {
	var b strings.Builder
	b.WriteString("Launch a subagent to run a self-contained multi-step task. " +
		"The subagent has no memory of the current conversation — brief it fully in prompt.")
	if t.reg != nil {
		specs := t.reg.List()
		if len(specs) > 0 {
			b.WriteString(" Available subagent types:\n")
			for _, s := range specs {
				fmt.Fprintf(&b, "- %s: %s\n", s.Name, s.Description)
			}
		}
	}
	return b.String()
}

// UserFacingName returns "Task: <description>" for display in the TUI.
func (*Task) UserFacingName(input map[string]any) string {
	if d, _ := input["description"].(string); d != "" {
		return "Task: " + d
	}
	return "Task"
}

// IsReadOnly returns false — a subagent can call write tools.
func (*Task) IsReadOnly() bool { return false }

// IsConcurrencySafe returns true — multiple Task tool_use blocks in one
// assistant turn run concurrently via the engine's parallel dispatch path.
func (*Task) IsConcurrencySafe() bool { return true }

// Schema returns the JSON schema for Task inputs.
func (*Task) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"description": map[string]any{
				"type":        "string",
				"description": "Short 5-10 word summary of the task (shown in UI).",
			},
			"prompt": map[string]any{
				"type":        "string",
				"description": "Self-contained instructions to the subagent. Brief the subagent like a colleague — it has no memory of this conversation.",
			},
			"subagent_type": map[string]any{
				"type":        "string",
				"description": `Subagent type name (e.g., "general-purpose").`,
			},
		},
		"required": []string{"description", "prompt", "subagent_type"},
	}
}

// Call invokes the runner with the parsed inputs.
func (t *Task) Call(ctx context.Context, input map[string]any, _ *Context) (Result, error) {
	desc, _ := input["description"].(string)
	prompt, _ := input["prompt"].(string)
	sub, _ := input["subagent_type"].(string)

	if desc == "" || prompt == "" || sub == "" {
		msg := "task: description, prompt, and subagent_type are required"
		return Result{Type: ResultText, Text: msg, ForLLM: msg, IsError: true}, nil
	}
	if t.runner == nil {
		msg := "task: no runner wired (anthrogo internal error)"
		return Result{Type: ResultText, Text: msg, ForLLM: msg, IsError: true}, nil
	}

	out, err := t.runner(ctx, TaskOptions{
		Description:  desc,
		Prompt:       prompt,
		SubagentType: sub,
	})
	if err != nil {
		msg := fmt.Sprintf("task failed: %v", err)
		return Result{Type: ResultText, Text: msg, ForLLM: msg, IsError: true}, nil
	}
	return Result{
		Type:   ResultText,
		Text:   out,
		ForLLM: out,
		Data:   map[string]any{"subagent_type": sub, "description": desc},
	}, nil
}

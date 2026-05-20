package tool

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/pkg/subagent"
)

func TestTask_Schema_Sanity(t *testing.T) {
	tk := NewTask(nil, nil)
	schema := tk.Schema()
	require.Equal(t, "object", schema["type"])
	props, ok := schema["properties"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, props, "description")
	require.Contains(t, props, "prompt")
	require.Contains(t, props, "subagent_type")
	req, ok := schema["required"].([]string)
	require.True(t, ok)
	require.ElementsMatch(t, []string{"description", "prompt", "subagent_type"}, req)
}

func TestTask_MissingFields_IsError(t *testing.T) {
	tk := NewTask(nil, nil)
	// Missing subagent_type
	res, err := tk.Call(context.Background(), map[string]any{
		"description": "do something",
		"prompt":      "do it",
	}, nil)
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Text, "required")
}

func TestTask_MissingPrompt_IsError(t *testing.T) {
	tk := NewTask(nil, nil)
	res, err := tk.Call(context.Background(), map[string]any{
		"description":   "do something",
		"subagent_type": "general-purpose",
	}, nil)
	require.NoError(t, err)
	require.True(t, res.IsError)
}

func TestTask_NilRunner_IsError(t *testing.T) {
	tk := NewTask(subagent.DefaultRegistry(), nil)
	res, err := tk.Call(context.Background(), map[string]any{
		"description":   "test task",
		"prompt":        "do something",
		"subagent_type": "general-purpose",
	}, nil)
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Text, "no runner")
}

func TestTask_RunnerSuccess_ReturnsText(t *testing.T) {
	runner := func(ctx context.Context, opts TaskOptions) (string, error) {
		require.Equal(t, "general-purpose", opts.SubagentType)
		require.Equal(t, "find the answer", opts.Description)
		require.Equal(t, "what is 2+2?", opts.Prompt)
		return "4", nil
	}
	tk := NewTask(subagent.DefaultRegistry(), runner)
	res, err := tk.Call(context.Background(), map[string]any{
		"description":   "find the answer",
		"prompt":        "what is 2+2?",
		"subagent_type": "general-purpose",
	}, nil)
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Equal(t, "4", res.Text)
	require.Equal(t, "4", res.ForLLM)
}

func TestTask_RunnerError_IsError(t *testing.T) {
	runner := func(ctx context.Context, opts TaskOptions) (string, error) {
		return "", context.DeadlineExceeded
	}
	tk := NewTask(subagent.DefaultRegistry(), runner)
	res, err := tk.Call(context.Background(), map[string]any{
		"description":   "test",
		"prompt":        "go",
		"subagent_type": "general-purpose",
	}, nil)
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Text, "task failed")
}

func TestTask_Name(t *testing.T) {
	require.Equal(t, "Task", NewTask(nil, nil).Name())
}

func TestTask_UserFacingName(t *testing.T) {
	tk := NewTask(nil, nil)
	require.Equal(t, "Task: search files", tk.UserFacingName(map[string]any{"description": "search files"}))
	require.Equal(t, "Task", tk.UserFacingName(map[string]any{}))
}

func TestTask_Description_ListsTypes(t *testing.T) {
	reg := subagent.DefaultRegistry()
	tk := NewTask(reg, nil)
	desc := tk.Description(context.Background())
	require.Contains(t, desc, "general-purpose")
	require.Contains(t, desc, "subagent")
}

// TestTask_Call_BufferEmitsOnNewlineBoundaries verifies that AppendUIMessage is
// called with properly prefixed lines at newline boundaries, and the remaining
// buffer is flushed after the runner returns.
func TestTask_Call_BufferEmitsOnNewlineBoundaries(t *testing.T) {
	var mu sync.Mutex
	var emitted []string
	appendUI := func(msg string) {
		mu.Lock()
		defer mu.Unlock()
		emitted = append(emitted, msg)
	}

	tcx := &Context{
		AppendUIMessage: appendUI,
	}

	// Runner emits "foo\nbar\n" as two delta chunks plus "baz" without a trailing newline.
	runner := func(ctx context.Context, opts TaskOptions) (string, error) {
		opts.OnDelta("foo\n")
		opts.OnDelta("bar\n")
		opts.OnDelta("baz") // no trailing newline — flushed after runner returns
		return "done", nil
	}

	tk := NewTask(subagent.DefaultRegistry(), runner)
	res, err := tk.Call(context.Background(), map[string]any{
		"description":   "test-desc",
		"prompt":        "do it",
		"subagent_type": "general-purpose",
	}, tcx)

	require.NoError(t, err)
	require.False(t, res.IsError)

	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, []string{
		"[Task: test-desc] foo",
		"[Task: test-desc] bar",
		"[Task: test-desc] baz",
	}, emitted)
}

// TestTask_Call_NilContext_NoEmit verifies that Call works without a TUI context
// (nil tcx) without panicking.
func TestTask_Call_NilContext_NoEmit(t *testing.T) {
	runner := func(ctx context.Context, opts TaskOptions) (string, error) {
		if opts.OnDelta != nil {
			opts.OnDelta("should not panic")
		}
		return "ok", nil
	}
	tk := NewTask(subagent.DefaultRegistry(), runner)
	res, err := tk.Call(context.Background(), map[string]any{
		"description":   "safe",
		"prompt":        "go",
		"subagent_type": "general-purpose",
	}, nil)
	require.NoError(t, err)
	require.False(t, res.IsError)
}

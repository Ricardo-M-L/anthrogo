package tool

import (
	"context"
	"encoding/json"
	"sync"
)

// Todo mirrors the upstream TodoWriteTool item shape.
type Todo struct {
	Content    string `json:"content"`
	Status     string `json:"status"` // pending | in_progress | completed
	ActiveForm string `json:"activeForm,omitempty"`
}

// TodoWrite replaces the tool's internal list on each call. M1 keeps a single
// session-scoped list; M2 will scope per-session-id when sessions persist.
type TodoWrite struct {
	DefaultPermission
	mu    sync.Mutex
	items []Todo
}

func (*TodoWrite) Name() string                         { return "TodoWrite" }
func (*TodoWrite) Description(context.Context) string   { return todoDescription }
func (*TodoWrite) UserFacingName(map[string]any) string { return "TodoWrite" }
func (*TodoWrite) IsReadOnly() bool                     { return false }
func (*TodoWrite) IsConcurrencySafe() bool              { return true }

func (*TodoWrite) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"todos": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"content":    map[string]any{"type": "string"},
						"status":     map[string]any{"type": "string", "enum": []string{"pending", "in_progress", "completed"}},
						"activeForm": map[string]any{"type": "string"},
					},
					"required": []string{"content", "status"},
				},
			},
		},
		"required": []string{"todos"},
	}
}

func (t *TodoWrite) Call(_ context.Context, input map[string]any, _ *Context) (Result, error) {
	raw, ok := input["todos"]
	if !ok {
		return errResult("todos is required"), nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return errResult(err.Error()), nil
	}
	var items []Todo
	if err := json.Unmarshal(data, &items); err != nil {
		return errResult("invalid todos shape: " + err.Error()), nil
	}
	t.mu.Lock()
	t.items = items
	t.mu.Unlock()
	msg := "todo list updated"
	return Result{Type: ResultText, Text: msg, ForLLM: msg, Data: map[string]any{"todos": items}}, nil
}

func (t *TodoWrite) List() []Todo {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]Todo, len(t.items))
	copy(out, t.items)
	return out
}

const todoDescription = `Maintain a structured todo list for the current session. Pass the COMPLETE list every time; the tool replaces the prior list. Use exactly one item with status="in_progress" while you're working; mark items "completed" as soon as they're done.`

package query

// PermissionDenial captures one rejected tool invocation.
type PermissionDenial struct {
	ToolName  string         `json:"tool_name"`
	ToolUseID string         `json:"tool_use_id"`
	ToolInput map[string]any `json:"tool_input"`
}

// Denials returns a copy of all denials recorded during this engine's lifetime.
func (e *Engine) Denials() []PermissionDenial {
	out := make([]PermissionDenial, len(e.denials))
	copy(out, e.denials)
	return out
}

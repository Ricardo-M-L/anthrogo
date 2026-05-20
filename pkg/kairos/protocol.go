package kairos

import "context"

// RunRequest is the JSON body for POST /kairos/run.
type RunRequest struct {
	SubagentType string `json:"subagent_type"`
	Prompt       string `json:"prompt"`
	Description  string `json:"description,omitempty"`
}

// RunHandler is the function signature the server invokes for each request.
// emit is called for each text delta; the returned finalText is sent as the
// event: done payload. On error, event: error is sent instead.
type RunHandler func(ctx context.Context, req RunRequest, emit func(textDelta string)) (finalText string, err error)

// ToolUseRequest is the payload sent by the worker over SSE when it needs the
// client to execute a tool call. The worker blocks waiting for the matching
// ToolResult to arrive via POST /kairos/run/<rid>/tool-result.
type ToolUseRequest struct {
	ToolUseID string         `json:"tool_use_id"`
	ToolName  string         `json:"tool_name"`
	ToolInput map[string]any `json:"tool_input"`
}

// ToolResult is the payload the client POSTs back to the worker after executing
// a tool locally. IsError mirrors tool.Result.IsError.
type ToolResult struct {
	ToolUseID string `json:"tool_use_id"`
	Text      string `json:"text"`
	IsError   bool   `json:"is_error"`
}

// RunHandlerWithToolForward extends RunHandler for the exec-tools-locally mode.
// The worker invokes emitToolUse to push a tool_use_request SSE event to the
// client, then calls waitForResult which blocks until the client POSTs the
// matching result to /kairos/run/<rid>/tool-result.
//
// Note: the permission gate is NOT applied on the worker side — the client
// runs its own gate before POSTing the result back. A denied tool call arrives
// as an IsError result.
type RunHandlerWithToolForward func(
	ctx context.Context,
	req RunRequest,
	emitText func(string),
	emitToolUse func(req ToolUseRequest),
	waitForResult func(toolUseID string) (ToolResult, error),
) (string, error)

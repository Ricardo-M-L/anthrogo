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

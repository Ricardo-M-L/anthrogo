package kairos

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/ricardo/anthrogo/pkg/permissions"
	"github.com/ricardo/anthrogo/pkg/tool"
)

// ClientOptions configures DispatchRemoteWithOptions.
type ClientOptions struct {
	AuthToken        string
	ExecToolsLocally bool
	// ToolRegistry is required when ExecToolsLocally is true. Tool calls from
	// the remote subagent are dispatched through this registry on the client.
	ToolRegistry *tool.Registry
	// Permissions is required when ExecToolsLocally is true. The client-side
	// permission gate is applied before each tool call.
	Permissions *permissions.Context
}

// DispatchRemote sends a RunRequest to a KAIROS worker at endpoint, streams
// the SSE response, and returns the accumulated final text.
//
// The client accumulates text deltas and returns the "final" field from the
// event: done frame (preferred) or the accumulated deltas as a fallback.
// On event: error the returned error contains the remote error message.
//
// DispatchRemote is a thin wrapper around DispatchRemoteWithOptions with no
// exec-tools-locally behaviour.
func DispatchRemote(ctx context.Context, endpoint, authToken, subagentType, description, prompt string) (string, error) {
	return DispatchRemoteWithOptions(ctx, endpoint, RunRequest{
		SubagentType: subagentType,
		Prompt:       prompt,
		Description:  description,
	}, ClientOptions{AuthToken: authToken})
}

// DispatchRemoteWithOptions is the full client entry-point. When
// opts.ExecToolsLocally is true it:
//  1. Sends X-Anthrogo-Exec-Tools-Locally: true on the request.
//  2. Captures the run_id from event: run_id.
//  3. On each event: tool_use_request, dispatches via opts.ToolRegistry, gates
//     via opts.Permissions, and POSTs the ToolResult back to
//     /kairos/run/<rid>/tool-result.
//  4. Continues consuming text deltas until event: done.
//
// The client-side permission gate (opts.Permissions) applies independently of
// any gate on the worker. A denied tool call produces an IsError ToolResult
// that the worker's engine sees and may handle as it sees fit.
func DispatchRemoteWithOptions(ctx context.Context, endpoint string, req RunRequest, opts ClientOptions) (string, error) {
	body, _ := json.Marshal(req)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(endpoint, "/")+"/kairos/run", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	if opts.AuthToken != "" {
		httpReq.Header.Set("Authorization", "Bearer "+opts.AuthToken)
	}
	if opts.ExecToolsLocally {
		httpReq.Header.Set("X-Anthrogo-Exec-Tools-Locally", "true")
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := bufio.NewReader(resp.Body).ReadString('\n')
		return "", fmt.Errorf("kairos: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(msg))
	}

	// Parse SSE stream.
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 1024), 1024*1024)

	var event, data string
	var accumulated strings.Builder
	var finalText string
	var streamErr error
	var rid string // run-id for POSTing tool results

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			// Blank line: dispatch accumulated event.
			if event != "" {
				switch event {
				case "run_id":
					var p struct {
						RunID string `json:"run_id"`
					}
					_ = json.Unmarshal([]byte(data), &p)
					rid = p.RunID

				case "text":
					var p struct {
						Text string `json:"text"`
					}
					_ = json.Unmarshal([]byte(data), &p)
					accumulated.WriteString(p.Text)

				case "tool_use_request":
					if opts.ExecToolsLocally && opts.ToolRegistry != nil && rid != "" {
						var tur ToolUseRequest
						if jsonErr := json.Unmarshal([]byte(data), &tur); jsonErr == nil {
							result := dispatchToolLocally(ctx, tur, opts, endpoint, opts.AuthToken, rid)
							if postErr := postToolResult(ctx, endpoint, opts.AuthToken, rid, result); postErr != nil {
								// Non-fatal; the worker will time out waiting and return an error.
								_ = postErr
							}
						}
					}

				case "done":
					var p struct {
						Final string `json:"final"`
					}
					_ = json.Unmarshal([]byte(data), &p)
					if p.Final != "" {
						finalText = p.Final
					}

				case "error":
					var p struct {
						Error string `json:"error"`
					}
					_ = json.Unmarshal([]byte(data), &p)
					streamErr = fmt.Errorf("kairos remote error: %s", p.Error)
				}
			}
			event = ""
			data = ""
			continue
		}
		if strings.HasPrefix(line, "event: ") {
			event = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			data = strings.TrimPrefix(line, "data: ")
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	if streamErr != nil {
		return "", streamErr
	}
	if finalText != "" {
		return finalText, nil
	}
	return accumulated.String(), nil
}

// dispatchToolLocally executes a tool call on the client using the provided
// registry and permission gate. It mirrors the logic in query/loop.go's
// executeTool but without the engine's session recording or hook callbacks
// (those live on the worker side).
func dispatchToolLocally(ctx context.Context, tur ToolUseRequest, opts ClientOptions, _ string, _ string, _ string) ToolResult {
	reg := opts.ToolRegistry

	t, ok := reg.Get(tur.ToolName)
	if !ok {
		return ToolResult{
			ToolUseID: tur.ToolUseID,
			Text:      "unknown tool: " + tur.ToolName,
			IsError:   true,
		}
	}

	// Apply client-side permission gate.
	decision := t.Permission(ctx, tur.ToolInput)
	if decision.Behavior == permissions.BehaviorAsk {
		decision = permissions.Decide(opts.Permissions, tur.ToolName, tur.ToolInput)
	}
	if decision.ModifiedInput != nil {
		tur.ToolInput = decision.ModifiedInput
	}
	// If still Ask (no prompt surface), deny.
	if decision.Behavior == permissions.BehaviorAsk {
		decision = permissions.Decision{Behavior: permissions.BehaviorDeny, Reason: "no prompt surface; ask denied"}
	}
	if decision.Behavior == permissions.BehaviorDeny {
		return ToolResult{
			ToolUseID: tur.ToolUseID,
			Text:      "permission denied: " + decision.Reason,
			IsError:   true,
		}
	}

	tcx := &tool.Context{
		Permissions:  opts.Permissions,
		AbortContext: ctx,
	}
	res, err := t.Call(ctx, tur.ToolInput, tcx)
	if err != nil {
		return ToolResult{ToolUseID: tur.ToolUseID, Text: err.Error(), IsError: true}
	}
	return ToolResult{
		ToolUseID: tur.ToolUseID,
		Text:      res.ModelText(),
		IsError:   res.IsError,
	}
}

// postToolResult POSTs a ToolResult back to the worker's tool-result endpoint.
func postToolResult(ctx context.Context, endpoint, authToken, rid string, result ToolResult) error {
	body, _ := json.Marshal(result)
	url := strings.TrimRight(endpoint, "/") + "/kairos/run/" + rid + "/tool-result"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if authToken != "" {
		req.Header.Set("Authorization", "Bearer "+authToken)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("kairos tool-result POST: HTTP %d", resp.StatusCode)
	}
	return nil
}

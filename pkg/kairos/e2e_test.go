package kairos

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/pkg/permissions"
	"github.com/ricardo/anthrogo/pkg/tool"
)

// TestKAIROS_RoundTrip starts an in-process server, dispatches a request via
// the client, and asserts the final text matches the handler's return value.
func TestKAIROS_RoundTrip(t *testing.T) {
	handler := func(ctx context.Context, req RunRequest, emit func(string)) (string, error) {
		emit("chunk1 ")
		emit("chunk2 ")
		emit("chunk3")
		return "FINAL", nil
	}

	srv := NewServer(handler, "")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	result, err := DispatchRemote(context.Background(), ts.URL, "", "general-purpose", "round-trip test", "do the thing")
	require.NoError(t, err)
	require.Equal(t, "FINAL", result)
}

// TestKAIROS_RoundTrip_WithAuth verifies Bearer auth works end-to-end.
func TestKAIROS_RoundTrip_WithAuth(t *testing.T) {
	handler := func(ctx context.Context, req RunRequest, emit func(string)) (string, error) {
		return "AUTH_OK", nil
	}

	srv := NewServer(handler, "mytoken")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Correct token succeeds.
	result, err := DispatchRemote(context.Background(), ts.URL, "mytoken", "general-purpose", "", "prompt")
	require.NoError(t, err)
	require.Equal(t, "AUTH_OK", result)

	// Wrong token fails.
	_, err = DispatchRemote(context.Background(), ts.URL, "wrongtoken", "general-purpose", "", "prompt")
	require.Error(t, err)
	require.Contains(t, err.Error(), "401")
}

// TestKAIROS_RoundTrip_HandlerError verifies that handler errors propagate as event:error.
func TestKAIROS_RoundTrip_HandlerError(t *testing.T) {
	handler := func(ctx context.Context, req RunRequest, emit func(string)) (string, error) {
		emit("partial")
		return "", context.DeadlineExceeded
	}

	srv := NewServer(handler, "")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	_, err := DispatchRemote(context.Background(), ts.URL, "", "general-purpose", "", "prompt")
	require.Error(t, err)
	require.Contains(t, err.Error(), "kairos remote error")
}

// echoTool is a minimal tool that returns its "msg" input field as text.
// Used in TestKAIROS_RoundTrip_WithRemoteToolExec.
type echoTool struct{ tool.DefaultPermission }

func (echoTool) Name() string                         { return "Echo" }
func (echoTool) Description(context.Context) string   { return "echoes the msg input" }
func (echoTool) UserFacingName(map[string]any) string { return "Echo" }
func (echoTool) IsReadOnly() bool                     { return true }
func (echoTool) IsConcurrencySafe() bool              { return true }
func (echoTool) Schema() map[string]any               { return map[string]any{"type": "object"} }
func (echoTool) Call(_ context.Context, input map[string]any, _ *tool.Context) (tool.Result, error) {
	msg, _ := input["msg"].(string)
	return tool.Result{Text: "echoed: " + msg}, nil
}

// TestKAIROS_RoundTrip_WithRemoteToolExec verifies the exec-tools-locally round-trip:
//  1. Client opens SSE stream with ExecToolsLocally=true.
//  2. Server handler emits one tool_use_request (simulating a subagent needing a tool).
//  3. Client executes the Echo tool locally and POSTs the result back.
//  4. Server handler incorporates the result into its final text.
//  5. Client receives the final text containing the echoed value.
func TestKAIROS_RoundTrip_WithRemoteToolExec(t *testing.T) {
	// plain handler (unused when exec-tools-locally header present but server must have one)
	plainHandler := func(ctx context.Context, req RunRequest, emit func(string)) (string, error) {
		return "plain", nil
	}

	// forwardHandler simulates a subagent that needs one tool call before answering.
	forwardHandler := func(
		ctx context.Context,
		req RunRequest,
		emitText func(string),
		emitToolUse func(ToolUseRequest),
		waitForResult func(string) (ToolResult, error),
	) (string, error) {
		// Emit some text before the tool call.
		emitText("thinking... ")

		// Ask the client to run the Echo tool with msg="world".
		emitToolUse(ToolUseRequest{
			ToolUseID: "t1",
			ToolName:  "Echo",
			ToolInput: map[string]any{"msg": "world"},
		})

		// Block for the client's result.
		res, err := waitForResult("t1")
		if err != nil {
			return "", err
		}
		if res.IsError {
			return "", nil
		}

		// Build final text that incorporates the tool result.
		finalText := "tool said: " + res.Text
		return finalText, nil
	}

	srv := NewServerWithToolForward(plainHandler, forwardHandler, "")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Build client tool registry with the Echo tool.
	reg := tool.NewRegistry()
	reg.Register(echoTool{})
	perms := permissions.Empty()
	perms.AlwaysAllowRules[permissions.SourceCLI] = []permissions.Rule{{Tool: "Echo"}}

	result, err := DispatchRemoteWithOptions(context.Background(), ts.URL, RunRequest{
		SubagentType: "general-purpose",
		Prompt:       "echo world",
	}, ClientOptions{
		ExecToolsLocally: true,
		ToolRegistry:     reg,
		Permissions:      perms,
	})
	require.NoError(t, err)

	// Final text must incorporate the echoed value returned by the local Echo tool.
	require.Equal(t, "tool said: echoed: world", result)
}

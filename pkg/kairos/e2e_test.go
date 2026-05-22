package kairos

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/internal/hooks"
	"github.com/ricardo/anthrogo/pkg/permissions"
	"github.com/ricardo/anthrogo/pkg/subagent"
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

// TestKAIROS_RoundTrip_RemoteContext_Propagated verifies that a RunRequest with
// a non-nil RemoteContext round-trips correctly over JSON: the server handler
// can read the HopDepth and Permissions snapshot that the client sent.
func TestKAIROS_RoundTrip_RemoteContext_Propagated(t *testing.T) {
	var receivedRC *RemoteContext

	handler := func(ctx context.Context, req RunRequest, emit func(string)) (string, error) {
		receivedRC = req.RemoteContext
		return "ok", nil
	}

	srv := NewServer(handler, "")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	rc := &RemoteContext{
		HopDepth: 1,
		Permissions: &PermSnapshot{
			Mode: "default",
			AlwaysDenyRules: map[permissions.Source][]permissions.Rule{
				permissions.SourceUser: {{Tool: "Bash"}},
			},
		},
	}
	_, err := DispatchRemoteWithOptions(context.Background(), ts.URL, RunRequest{
		SubagentType:  "general-purpose",
		Prompt:        "test",
		RemoteContext: rc,
	}, ClientOptions{})
	require.NoError(t, err)
	require.NotNil(t, receivedRC)
	require.Equal(t, 1, receivedRC.HopDepth)
	require.NotNil(t, receivedRC.Permissions)
	require.Equal(t, "default", receivedRC.Permissions.Mode)
	bashRules := receivedRC.Permissions.AlwaysDenyRules[permissions.SourceUser]
	require.Len(t, bashRules, 1)
	require.Equal(t, "Bash", bashRules[0].Tool)
}

// TestKAIROS_RoundTrip_RemoteContext_HooksConfig verifies that hook specs
// inside RemoteContext.Hooks are received intact by the server handler.
func TestKAIROS_RoundTrip_RemoteContext_HooksConfig(t *testing.T) {
	var receivedRC *RemoteContext

	handler := func(ctx context.Context, req RunRequest, emit func(string)) (string, error) {
		receivedRC = req.RemoteContext
		return "ok", nil
	}

	srv := NewServer(handler, "")
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	hookCfg := &hooks.Config{
		PreToolUse: []hooks.Spec{
			{Command: "/usr/local/bin/audit-hook", Matcher: "Bash"},
		},
	}
	rc := &RemoteContext{
		HopDepth: 0,
		Hooks:    hookCfg,
	}
	_, err := DispatchRemoteWithOptions(context.Background(), ts.URL, RunRequest{
		SubagentType:  "general-purpose",
		Prompt:        "test",
		RemoteContext: rc,
	}, ClientOptions{})
	require.NoError(t, err)
	require.NotNil(t, receivedRC)
	require.NotNil(t, receivedRC.Hooks)
	require.Len(t, receivedRC.Hooks.PreToolUse, 1)
	require.Equal(t, "/usr/local/bin/audit-hook", receivedRC.Hooks.PreToolUse[0].Command)
}

// TestKAIROS_HopDepth_BuildWorkerSubReg verifies that the hop-depth logic in
// buildWorkerSubReg (mirrored here as a standalone function under test) correctly
// excludes Remote-typed subagents when HopDepth >= MaxHops and includes them otherwise.
func TestKAIROS_HopDepth_BuildWorkerSubReg(t *testing.T) {
	// Build a test subagent registry with one remote and one local spec.
	remoteSpec := subagent.Spec{
		Name: "remote-worker",
		Remote: &subagent.RemoteSpec{
			Endpoint: "http://remote.example.com:9001",
		},
	}
	localSpec := subagent.Spec{
		Name:               "local-helper",
		SystemPromptSuffix: "you are helpful",
	}

	buildReg := func(rc *RemoteContext) *subagent.Registry {
		hopDepth := 0
		if rc != nil {
			hopDepth = rc.HopDepth
		}
		reg := subagent.NewRegistry()
		for _, s := range []subagent.Spec{remoteSpec, localSpec} {
			if s.Remote != nil && hopDepth >= MaxHops {
				continue
			}
			reg.Register(s)
		}
		return reg
	}

	// At hop depth 0: both should be registered.
	reg0 := buildReg(&RemoteContext{HopDepth: 0})
	_, hasRemote := reg0.Get("remote-worker")
	_, hasLocal := reg0.Get("local-helper")
	require.True(t, hasRemote, "remote spec should be registered at hop depth 0")
	require.True(t, hasLocal, "local spec should be registered at hop depth 0")

	// At hop depth 1: still below MaxHops (2), remote still registered.
	reg1 := buildReg(&RemoteContext{HopDepth: 1})
	_, hasRemote1 := reg1.Get("remote-worker")
	require.True(t, hasRemote1, "remote spec should be registered at hop depth 1")

	// At hop depth 2 (= MaxHops): remote should be excluded.
	reg2 := buildReg(&RemoteContext{HopDepth: 2})
	_, hasRemote2 := reg2.Get("remote-worker")
	_, hasLocal2 := reg2.Get("local-helper")
	require.False(t, hasRemote2, "remote spec must be excluded at hop depth == MaxHops")
	require.True(t, hasLocal2, "local spec should still be registered at hop depth == MaxHops")

	// At hop depth 3 (> MaxHops): remote should also be excluded.
	reg3 := buildReg(&RemoteContext{HopDepth: 3})
	_, hasRemote3 := reg3.Get("remote-worker")
	require.False(t, hasRemote3, "remote spec must be excluded at hop depth > MaxHops")

	// nil RemoteContext (no context): defaults to hop depth 0 → remote registered.
	regNil := buildReg(nil)
	_, hasRemoteNil := regNil.Get("remote-worker")
	require.True(t, hasRemoteNil, "nil RemoteContext should default to hop depth 0")
}

// TestKAIROS_SignedRoundTrip starts a signed server and verifies that a client
// with the matching public key can consume the SSE stream without error.
func TestKAIROS_SignedRoundTrip(t *testing.T) {
	priv, pub, err := GenerateKeyPair()
	require.NoError(t, err)

	handler := func(ctx context.Context, req RunRequest, emit func(string)) (string, error) {
		emit("hello")
		return "world", nil
	}

	srv := httptest.NewServer(NewServerWithSigning(handler, "", priv).Handler())
	defer srv.Close()

	result, err := DispatchRemoteWithOptions(context.Background(), srv.URL, RunRequest{
		SubagentType: "general-purpose",
		Prompt:       "go",
	}, ClientOptions{TrustKey: pub})
	require.NoError(t, err)
	require.Equal(t, "world", result)
}

// TestKAIROS_SignedRoundTrip_TamperedKeyFails verifies that a client with the
// wrong (mismatched) public key gets an error.
func TestKAIROS_SignedRoundTrip_TamperedKeyFails(t *testing.T) {
	priv, _, err := GenerateKeyPair()
	require.NoError(t, err)
	_, badPub, err := GenerateKeyPair()
	require.NoError(t, err)

	handler := func(ctx context.Context, req RunRequest, emit func(string)) (string, error) {
		emit("hello")
		return "world", nil
	}

	srv := httptest.NewServer(NewServerWithSigning(handler, "", priv).Handler())
	defer srv.Close()

	_, err = DispatchRemoteWithOptions(context.Background(), srv.URL, RunRequest{
		SubagentType: "general-purpose",
		Prompt:       "go",
	}, ClientOptions{TrustKey: badPub})
	require.Error(t, err)
	require.Contains(t, err.Error(), "signature verification failed")
}

// TestKAIROS_UnsignedServer_TrustKeyClient verifies that a client with TrustKey
// set receives an error when the server sends unsigned (plain JSON) frames, because
// the frame won't decode as a SignedFrame with a valid sig field.
func TestKAIROS_UnsignedServer_TrustKeyClient(t *testing.T) {
	_, pub, err := GenerateKeyPair()
	require.NoError(t, err)

	handler := func(ctx context.Context, req RunRequest, emit func(string)) (string, error) {
		return "world", nil
	}

	// Plain server (no signing).
	srv := httptest.NewServer(NewServer(handler, "").Handler())
	defer srv.Close()

	_, err = DispatchRemoteWithOptions(context.Background(), srv.URL, RunRequest{
		SubagentType: "general-purpose",
		Prompt:       "go",
	}, ClientOptions{TrustKey: pub})
	require.Error(t, err)
}

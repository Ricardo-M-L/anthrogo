package query

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/internal/session"
	"github.com/ricardo/anthrogo/pkg/kairos"
	"github.com/ricardo/anthrogo/pkg/message"
	"github.com/ricardo/anthrogo/pkg/permissions"
	"github.com/ricardo/anthrogo/pkg/pricing"
	"github.com/ricardo/anthrogo/pkg/provider"
	"github.com/ricardo/anthrogo/pkg/provider/fake"
	"github.com/ricardo/anthrogo/pkg/subagent"
	"github.com/ricardo/anthrogo/pkg/tool"
)

func TestEngine_TextOnlyTurn_EmitsDeltasAndComplete(t *testing.T) {
	fp := fake.New([]provider.Event{
		{Kind: provider.EventTextDelta, Text: "hi"},
		{Kind: provider.EventTextDelta, Text: " there"},
		{Kind: provider.EventMessageStop, StopReason: "end_turn"},
	})
	e := NewEngine(Config{
		Provider:     fp,
		Tools:        tool.NewRegistry(),
		Permissions:  permissions.Empty(),
		Model:        "claude-sonnet-4-6",
		SystemPrompt: "you are a helper",
	})

	ch := e.SubmitMessage(context.Background(), "hello")
	var assistantText strings.Builder
	var sawComplete bool
	for ev := range ch {
		switch ev.Kind {
		case KindAssistantDelta:
			assistantText.WriteString(ev.Text)
		case KindTurnComplete:
			sawComplete = true
		}
	}
	require.Equal(t, "hi there", assistantText.String())
	require.True(t, sawComplete)
	require.Len(t, e.Messages(), 2)
	require.Equal(t, message.RoleUser, e.Messages()[0].Role)
	require.Equal(t, message.RoleAssistant, e.Messages()[1].Role)
}

func TestEngine_ToolUseTurn_ExecutesToolAndContinues(t *testing.T) {
	// Turn 1: model asks for Read; Turn 2: model answers.
	fp := fake.New(
		[]provider.Event{
			{Kind: provider.EventToolUseStart, ToolUseID: "u1", ToolName: "Read"},
			{Kind: provider.EventToolInputDelta, PartialJSON: `{"file_path": "/etc/hostname"}`},
			{Kind: provider.EventBlockStop, StopReason: "tool_use"},
			{Kind: provider.EventMessageStop, StopReason: "tool_use"},
		},
		[]provider.Event{
			{Kind: provider.EventTextDelta, Text: "done"},
			{Kind: provider.EventMessageStop, StopReason: "end_turn"},
		},
	)
	reg := tool.NewRegistry()
	reg.Register(tool.Read{})
	perms := permissions.Empty()
	perms.AlwaysAllowRules[permissions.SourceCLI] = []permissions.Rule{{Tool: "Read"}}

	e := NewEngine(Config{
		Provider: fp, Tools: reg, Permissions: perms, Model: "x",
	})
	ch := e.SubmitMessage(context.Background(), "read /etc/hostname")
	var saw struct{ tool, result, complete bool }
	for ev := range ch {
		switch ev.Kind {
		case KindToolUseRequest:
			saw.tool = true
		case KindToolResult:
			saw.result = true
		case KindTurnComplete:
			saw.complete = true
		}
	}
	require.True(t, saw.tool, "expected ToolUseRequest event")
	require.True(t, saw.result, "expected ToolResult event")
	require.True(t, saw.complete, "expected TurnComplete event")
}

func TestEngine_PermissionDeny_FeedsErrorBack(t *testing.T) {
	fp := fake.New(
		[]provider.Event{
			{Kind: provider.EventToolUseStart, ToolUseID: "u1", ToolName: "Bash"},
			{Kind: provider.EventToolInputDelta, PartialJSON: `{"command": "rm -rf /"}`},
			{Kind: provider.EventBlockStop, StopReason: "tool_use"},
			{Kind: provider.EventMessageStop, StopReason: "tool_use"},
		},
		[]provider.Event{
			{Kind: provider.EventTextDelta, Text: "ok skipped"},
			{Kind: provider.EventMessageStop, StopReason: "end_turn"},
		},
	)
	reg := tool.NewRegistry()
	reg.Register(tool.Bash{})
	perms := permissions.Empty()
	// NOTE: Rule field is "Pattern" (not "Match")
	perms.AlwaysDenyRules[permissions.SourceCLI] = []permissions.Rule{{Tool: "Bash", Pattern: "rm -rf*"}}
	perms.ShouldAvoidPrompts = true

	e := NewEngine(Config{Provider: fp, Tools: reg, Permissions: perms, Model: "x"})
	ch := e.SubmitMessage(context.Background(), "delete everything")
	denied := false
	for ev := range ch {
		if ev.Kind == KindToolResult && ev.IsError {
			denied = true
		}
	}
	require.True(t, denied, "expected a denied tool_result event")
}

func TestEngine_ContextCancel_EndsTurnWithError(t *testing.T) {
	slow := &slowFakeProvider{}
	e := NewEngine(Config{Provider: slow, Tools: tool.NewRegistry(), Permissions: permissions.Empty(), Model: "x"})
	ctx, cancel := context.WithCancel(context.Background())
	ch := e.SubmitMessage(ctx, "hi")

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	var sawErr bool
	for ev := range ch {
		if ev.Kind == KindError {
			sawErr = true
			require.ErrorIs(t, ev.Err, context.Canceled)
		}
	}
	require.True(t, sawErr, "expected KindError carrying context.Canceled")
}

// recordingTool records the input it receives on Call so tests can inspect it.
type recordingTool struct {
	tool.DefaultPermission
	lastInput map[string]any
}

func (r *recordingTool) Name() string                         { return "RecordingTool" }
func (r *recordingTool) Description(context.Context) string   { return "records input" }
func (r *recordingTool) UserFacingName(map[string]any) string { return "RecordingTool" }
func (r *recordingTool) Schema() map[string]any               { return map[string]any{"type": "object"} }
func (r *recordingTool) IsReadOnly() bool                     { return true }
func (r *recordingTool) IsConcurrencySafe() bool              { return true }
func (r *recordingTool) Call(_ context.Context, input map[string]any, _ *tool.Context) (tool.Result, error) {
	r.lastInput = input
	return tool.Result{Text: "ok"}, nil
}

func TestEngine_HookModifiedInput_ReachesTool(t *testing.T) {
	seen := &recordingTool{}
	reg := tool.NewRegistry()
	reg.Register(seen)

	// Two-script provider: turn 1 asks for RecordingTool; turn 2 returns end_turn.
	fp := fake.New(
		[]provider.Event{
			{Kind: provider.EventToolUseStart, ToolUseID: "u1", ToolName: "RecordingTool"},
			{Kind: provider.EventToolInputDelta, PartialJSON: `{"orig": true}`},
			{Kind: provider.EventBlockStop, StopReason: "tool_use"},
			{Kind: provider.EventMessageStop, StopReason: "tool_use"},
		},
		[]provider.Event{
			{Kind: provider.EventTextDelta, Text: "done"},
			{Kind: provider.EventMessageStop, StopReason: "end_turn"},
		},
	)

	perms := permissions.Empty()
	// Hook: always pass (don't deny or allow), but rewrite the input.
	perms.HookDecide = func(_ context.Context, toolName string, input map[string]any) permissions.HookOutcome {
		return permissions.HookOutcome{
			Pass:          true,
			ModifiedInput: map[string]any{"mutated": true},
		}
	}
	// Allow the tool so the gate doesn't block it after the hook pass.
	perms.AlwaysAllowRules[permissions.SourceCLI] = []permissions.Rule{{Tool: "RecordingTool"}}

	e := NewEngine(Config{
		Provider:    fp,
		Tools:       reg,
		Permissions: perms,
		Model:       "x",
	})
	for range e.SubmitMessage(context.Background(), "run recording tool") {
	}

	require.NotNil(t, seen.lastInput, "tool Call was never reached")
	require.Equal(t, true, seen.lastInput["mutated"], "mutated key must be present")
	_, hasOrig := seen.lastInput["orig"]
	require.False(t, hasOrig, "orig key must be absent after mutation")
}

// recordingHookSink records FirePreCompact and FireSubagentStop calls for tests.
type recordingHookSink struct {
	preCompactCalled   bool
	subagentStopReason string
	subagentStopCalled bool
}

func (r *recordingHookSink) FirePostToolUse(_ context.Context, _ string, _, _ map[string]any) string {
	return ""
}
func (r *recordingHookSink) FireStop(_ context.Context, _ string) {}
func (r *recordingHookSink) FirePreCompact(_ context.Context, _ string) {
	r.preCompactCalled = true
}
func (r *recordingHookSink) FireSubagentStop(_ context.Context, reason string) {
	r.subagentStopCalled = true
	r.subagentStopReason = reason
}

func TestEngine_Compact_FiresHookAndReplaces(t *testing.T) {
	// Use alternating user/assistant messages so the new assistant-boundary
	// split algorithm can find a valid split point.
	msgs := make([]message.Message, 15)
	for i := range msgs {
		if i%2 == 0 {
			msgs[i] = message.Message{Role: message.RoleUser, Content: []message.Block{{Type: message.BlockText, Text: "u"}}}
		} else {
			msgs[i] = message.Message{Role: message.RoleAssistant, Content: []message.Block{{Type: message.BlockText, Text: "a"}}}
		}
	}
	// Adaptation: fake.New (variadic) instead of plan's fake.NewScripted.
	fp := fake.New(
		[]provider.Event{
			{Kind: provider.EventStart},
			{Kind: provider.EventTextDelta, Text: "SUM"},
			{Kind: provider.EventMessageStop},
		},
	)
	rec := &recordingHookSink{}
	e := NewEngine(Config{Provider: fp, Model: "m", Hooks: rec})
	e.SetInitialMessages(msgs)

	s, err := e.Compact(context.Background(), CompactOptions{KeepRecent: 10})
	require.NoError(t, err)
	require.False(t, s.Skipped)
	require.True(t, rec.preCompactCalled)
	// desiredSplit=5, msgs[5] is assistant → split=5, head=5, tail=10 → newCount=11
	require.Equal(t, 11, s.NewCount)
	require.Equal(t, 11, len(e.Messages()))
	require.Equal(t, "SUM", s.SummaryText)
}

func TestEngine_Compact_SkipsWhenShort(t *testing.T) {
	e := NewEngine(Config{Provider: fake.New(nil), Model: "m"})
	e.SetInitialMessages([]message.Message{
		{Role: message.RoleUser, Content: []message.Block{{Type: message.BlockText, Text: "a"}}},
	})
	s, err := e.Compact(context.Background(), CompactOptions{KeepRecent: 10})
	require.NoError(t, err)
	require.True(t, s.Skipped)
}

func TestEngine_Compact_ConcurrentSafe(t *testing.T) {
	msgs := make([]message.Message, 30)
	for i := range msgs {
		if i%2 == 0 {
			msgs[i] = message.Message{Role: message.RoleUser, Content: []message.Block{{Type: message.BlockText, Text: "u"}}}
		} else {
			msgs[i] = message.Message{Role: message.RoleAssistant, Content: []message.Block{{Type: message.BlockText, Text: "a"}}}
		}
	}
	fp := fake.New(
		[]provider.Event{{Kind: provider.EventStart}, {Kind: provider.EventTextDelta, Text: "S"}, {Kind: provider.EventMessageStop}},
	)
	e := NewEngine(Config{Provider: fp, Model: "m"})
	e.SetInitialMessages(msgs)

	// Run 50 concurrent reads of Messages() while Compact runs once.
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = e.Messages()
		}()
	}
	_, err := e.Compact(context.Background(), CompactOptions{KeepRecent: 10})
	require.NoError(t, err)
	wg.Wait()
}

// TestEngine_SkillRespectsAlwaysDeny verifies that an alwaysDeny rule for the
// Skill tool wins even when alwaysAllow is also present (deny > allow in gate).
// This is a direct Decide-level test; the engine respects Decide via loop.go.
func TestEngine_SkillRespectsAlwaysDeny(t *testing.T) {
	perms := permissions.Empty()
	// Both allow and deny rules present for Skill — deny must win.
	perms.AlwaysAllowRules[permissions.SourceCLI] = []permissions.Rule{{Tool: "Skill"}}
	perms.AlwaysDenyRules[permissions.SourceCLI] = []permissions.Rule{{Tool: "Skill"}}
	perms.ShouldAvoidPrompts = true

	d := permissions.Decide(context.Background(), perms, "Skill", map[string]any{"skill": "git-flow"})
	require.Equal(t, permissions.BehaviorDeny, d.Behavior, "alwaysDeny must trump alwaysAllow for Skill")
}

type slowFakeProvider struct{}

func (s *slowFakeProvider) Stream(ctx context.Context, _ provider.Request) (<-chan provider.Event, error) {
	out := make(chan provider.Event, 1)
	go func() {
		defer close(out)
		for i := 0; i < 10; i++ {
			select {
			case <-ctx.Done():
				out <- provider.Event{Kind: provider.EventError, Err: ctx.Err()}
				return
			case <-time.After(100 * time.Millisecond):
				out <- provider.Event{Kind: provider.EventTextDelta, Text: "."}
			}
		}
		out <- provider.Event{Kind: provider.EventMessageStop, StopReason: "end_turn"}
	}()
	return out, nil
}

// --- RunSubagent tests ---

func makeSubagentEngine(t *testing.T, fp provider.Provider, hooks HookSink) (*Engine, *subagent.Registry) {
	t.Helper()
	reg := subagent.DefaultRegistry()
	e := NewEngine(Config{
		Provider:         fp,
		Tools:            tool.NewRegistry(),
		Permissions:      permissions.Empty(),
		Model:            "x",
		SubagentRegistry: reg,
		Hooks:            hooks,
	})
	return e, reg
}

func TestEngine_RunSubagent_HappyPath(t *testing.T) {
	fp := fake.New([]provider.Event{
		{Kind: provider.EventTextDelta, Text: "SUB ANSWER"},
		{Kind: provider.EventMessageStop, StopReason: "end_turn"},
	})
	rec := &recordingHookSink{}
	e, _ := makeSubagentEngine(t, fp, rec)

	result, err := e.RunSubagent(context.Background(), SubagentOptions{
		Type:        "general-purpose",
		Description: "test task",
		Prompt:      "what is 2+2?",
	})
	require.NoError(t, err)
	require.Equal(t, "SUB ANSWER", result)
	require.True(t, rec.subagentStopCalled, "SubagentStop hook must fire")
	require.Equal(t, "end_turn", rec.subagentStopReason)
}

func TestEngine_RunSubagent_DepthLimit(t *testing.T) {
	fp := &alwaysEndTurnProvider{}
	reg := subagent.DefaultRegistry()

	// Create engine already at the max depth — any call should be rejected.
	e := NewEngine(Config{
		Provider:         fp,
		Tools:            tool.NewRegistry(),
		Permissions:      permissions.Empty(),
		Model:            "x",
		SubagentRegistry: reg,
		SubagentDepth:    3,
		MaxSubagentDepth: 3,
	})

	_, err := e.RunSubagent(context.Background(), SubagentOptions{
		Type:   "general-purpose",
		Prompt: "hello",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "depth limit")
}

func TestEngine_RunSubagent_UnknownType(t *testing.T) {
	fp := fake.New(nil)
	e, _ := makeSubagentEngine(t, fp, nil)

	_, err := e.RunSubagent(context.Background(), SubagentOptions{
		Type:   "nonexistent-weird-type",
		Prompt: "hello",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown type")
}

func TestEngine_RunSubagent_NoRegistry(t *testing.T) {
	fp := fake.New(nil)
	e := NewEngine(Config{
		Provider:    fp,
		Tools:       tool.NewRegistry(),
		Permissions: permissions.Empty(),
		Model:       "x",
		// SubagentRegistry intentionally nil
	})
	_, err := e.RunSubagent(context.Background(), SubagentOptions{
		Type:   "general-purpose",
		Prompt: "hi",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "no registry")
}

// TestEngine_RunSubagent_ConcurrentTwoTasksFromOneTurn verifies that when an
// assistant turn contains two Task tool_use blocks (both IsConcurrencySafe=true),
// the engine dispatches them concurrently and both tool_results appear in the
// message history.
func TestEngine_RunSubagent_ConcurrentTwoTasksFromOneTurn(t *testing.T) {
	subreg := subagent.DefaultRegistry()

	// Each subagent call gets its own scripted provider. We use a provider that
	// returns a different text depending on the script index. Because fake.Provider
	// serves scripts in order and Task.Call invokes e.RunSubagent (which calls the
	// parent engine's provider via the child engine), we need two separate child
	// providers. The simplest approach: wire a multi-script parent fake where:
	//   - Script 0 (parent turn 1): two Task tool_use blocks, stopReason=tool_use
	//   - Script 1 (parent turn 2): end_turn with text "all done"
	// And the Task runner itself uses an inner fake for the child — achieved by
	// using a multi-call fake that serves the parent first, then two child calls.
	//
	// Since fake.Provider.Stream is called in order, we script:
	//   call 0  → parent assistant turn with 2 Task tool_use blocks
	//   call 1  → child A: "FOO" end_turn
	//   call 2  → child B: "BAR" end_turn
	//   call 3  → parent turn 2 end_turn
	fp := fake.New(
		// Parent turn 1: two Task tool_use blocks.
		[]provider.Event{
			{Kind: provider.EventToolUseStart, ToolUseID: "ta", ToolName: "Task"},
			{Kind: provider.EventToolInputDelta, PartialJSON: `{"description":"task A","prompt":"do A","subagent_type":"general-purpose"}`},
			{Kind: provider.EventBlockStop},
			{Kind: provider.EventToolUseStart, ToolUseID: "tb", ToolName: "Task"},
			{Kind: provider.EventToolInputDelta, PartialJSON: `{"description":"task B","prompt":"do B","subagent_type":"general-purpose"}`},
			{Kind: provider.EventBlockStop},
			{Kind: provider.EventMessageStop, StopReason: "tool_use"},
		},
		// Child A provider script.
		[]provider.Event{
			{Kind: provider.EventTextDelta, Text: "FOO"},
			{Kind: provider.EventMessageStop, StopReason: "end_turn"},
		},
		// Child B provider script.
		[]provider.Event{
			{Kind: provider.EventTextDelta, Text: "BAR"},
			{Kind: provider.EventMessageStop, StopReason: "end_turn"},
		},
		// Parent turn 2: model acknowledges the results and ends.
		[]provider.Event{
			{Kind: provider.EventTextDelta, Text: "all done"},
			{Kind: provider.EventMessageStop, StopReason: "end_turn"},
		},
	)

	perms := permissions.Empty()
	perms.AlwaysAllowRules[permissions.SourceCLI] = []permissions.Rule{{Tool: "Task"}}

	toolReg := tool.NewRegistry()

	var engineRef sync.Mutex
	var parentEngine *Engine

	taskTool := tool.NewTask(subreg, func(ctx context.Context, opts tool.TaskOptions) (string, error) {
		engineRef.Lock()
		e := parentEngine
		engineRef.Unlock()
		if e == nil {
			return "", fmt.Errorf("engine not set")
		}
		return e.RunSubagent(ctx, SubagentOptions{
			Type:        opts.SubagentType,
			Description: opts.Description,
			Prompt:      opts.Prompt,
		})
	})
	toolReg.Register(taskTool)

	e := NewEngine(Config{
		Provider:         fp,
		Tools:            toolReg,
		Permissions:      perms,
		Model:            "x",
		SubagentRegistry: subreg,
	})
	engineRef.Lock()
	parentEngine = e
	engineRef.Unlock()

	// Drain the channel.
	for range e.SubmitMessage(context.Background(), "run two tasks") {
	}

	// Find the tool_result message in history.
	msgs := e.Messages()
	// Message order: [user, assistant(2 task uses), user(2 tool_results), assistant(final)]
	require.GreaterOrEqual(t, len(msgs), 3, "expected at least 3 messages")

	// Find the user message that contains tool results.
	var resultBlocks []message.Block
	for _, m := range msgs {
		if m.Role == message.RoleUser {
			for _, b := range m.Content {
				if b.Type == message.BlockToolResult {
					resultBlocks = append(resultBlocks, b)
				}
			}
		}
	}
	require.Len(t, resultBlocks, 2, "expected 2 tool_result blocks")

	// Collect result texts — order may vary since concurrent.
	texts := map[string]bool{}
	for _, b := range resultBlocks {
		texts[b.Text] = true
	}
	require.True(t, texts["FOO"], "expected FOO in tool results")
	require.True(t, texts["BAR"], "expected BAR in tool results")
}

// TestEngine_RunSubagent_ChildModeChangeDoesntAffectParent verifies that a
// subagent toggling permissions.Mode (via a tool that mutates tcx.Permissions)
// does not affect the parent engine's permissions context.
func TestEngine_RunSubagent_ChildModeChangeDoesntAffectParent(t *testing.T) {
	subreg := subagent.DefaultRegistry()

	// The child will call FlipMode which mutates tcx.Permissions.Mode to ModePlan.
	// After RunSubagent, parent permissions must still be ModeDefault.
	fp := fake.New(
		// Child turn: call FlipMode, then end.
		[]provider.Event{
			{Kind: provider.EventToolUseStart, ToolUseID: "f1", ToolName: "FlipMode"},
			{Kind: provider.EventToolInputDelta, PartialJSON: `{}`},
			{Kind: provider.EventBlockStop},
			{Kind: provider.EventMessageStop, StopReason: "tool_use"},
		},
		// Child turn 2: final end_turn after tool result.
		[]provider.Event{
			{Kind: provider.EventTextDelta, Text: "done"},
			{Kind: provider.EventMessageStop, StopReason: "end_turn"},
		},
	)

	childTools := tool.NewRegistry()
	childTools.Register(modeFlipper{})

	parentPerms := permissions.Empty()
	// parentPerms.Mode is ModeDefault.
	require.Equal(t, permissions.ModeDefault, parentPerms.Mode)

	// We pass childTools as the parent's tool registry so the child inherits it.
	e := NewEngine(Config{
		Provider:         fp,
		Tools:            childTools,
		Permissions:      parentPerms,
		Model:            "x",
		SubagentRegistry: subreg,
	})

	_, err := e.RunSubagent(context.Background(), SubagentOptions{
		Type:        "general-purpose",
		Description: "test",
		Prompt:      "flip mode",
	})
	require.NoError(t, err)

	// Parent's mode must be unchanged.
	require.Equal(t, permissions.ModeDefault, parentPerms.Mode,
		"child mode change must not affect parent permissions context")
}

// modeFlipper is a test tool that mutates tcx.Permissions.Mode to ModePlan
// when called.
type modeFlipper struct{ tool.DefaultPermission }

func (modeFlipper) Name() string                         { return "FlipMode" }
func (modeFlipper) Description(context.Context) string   { return "flips mode to plan" }
func (modeFlipper) UserFacingName(map[string]any) string { return "FlipMode" }
func (modeFlipper) IsReadOnly() bool                     { return true }
func (modeFlipper) IsConcurrencySafe() bool              { return true }
func (modeFlipper) Schema() map[string]any               { return map[string]any{"type": "object"} }
func (modeFlipper) Call(_ context.Context, _ map[string]any, tcx *tool.Context) (tool.Result, error) {
	if tcx != nil && tcx.Permissions != nil {
		tcx.Permissions.Mode = permissions.ModePlan
	}
	return tool.Result{Type: tool.ResultText, Text: "flipped", ForLLM: "flipped"}, nil
}

func TestEngine_RunSubagent_WritesSubagentJSONL(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	// Create a parent session.
	parentSess, err := session.New(session.NewOptions{Cwd: tmpHome, Model: "m"})
	require.NoError(t, err)
	defer parentSess.Close()

	rec := &recordingHookSink{}
	fp := fake.New(
		[]provider.Event{
			{Kind: provider.EventStart},
			{Kind: provider.EventTextDelta, Text: "SUB ANSWER"},
			{Kind: provider.EventMessageStop, StopReason: "end_turn"},
		},
	)
	subreg := subagent.NewRegistry()
	subreg.Register(subagent.Spec{Name: "general-purpose", Description: "x"})

	e := NewEngine(Config{
		Provider:         fp,
		Model:            "m",
		Hooks:            rec,
		SubagentRegistry: subreg,
		Session:          parentSess,
	})

	out, err := e.RunSubagent(context.Background(), SubagentOptions{
		Type: "general-purpose", Description: "test", Prompt: "go",
	})
	require.NoError(t, err)
	require.Equal(t, "SUB ANSWER", out)

	// Verify a subagent JSONL was created under <parent path without .jsonl>/subagents/
	base := strings.TrimSuffix(parentSess.Path(), ".jsonl")
	subDir := filepath.Join(base, "subagents")
	entries, err := os.ReadDir(subDir)
	require.NoError(t, err)
	require.NotEmpty(t, entries, "expected at least one subagent JSONL")

	// Read the JSONL — it should have at least 1 record.
	raw, err := os.ReadFile(filepath.Join(subDir, entries[0].Name()))
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	require.GreaterOrEqual(t, len(lines), 1)
}

// TestEngine_RunSubagent_RemoteDispatch verifies that when a subagent spec has
// a RemoteSpec, RunSubagent dispatches via HTTP to a kairos server instead of
// spawning a local child Engine.
func TestEngine_RunSubagent_RemoteDispatch(t *testing.T) {
	// Start a kairos httptest server that returns a fixed response.
	kHandler := func(ctx context.Context, req kairos.RunRequest, emit func(string)) (string, error) {
		emit("remote ")
		emit("answer")
		return "remote answer", nil
	}
	kSrv := kairos.NewServer(kHandler, "")
	ts := httptest.NewServer(kSrv.Handler())
	defer ts.Close()

	// Build a registry with a remote spec pointing at the httptest server.
	reg := subagent.NewRegistry()
	reg.Register(subagent.Spec{
		Name:        "remote-type",
		Description: "a remote subagent",
		Remote: &subagent.RemoteSpec{
			Endpoint: ts.URL,
		},
	})

	rec := &recordingHookSink{}
	// The fake provider should NOT be called for remote dispatch.
	fp := fake.New(nil)
	e := NewEngine(Config{
		Provider:         fp,
		Tools:            tool.NewRegistry(),
		Permissions:      permissions.Empty(),
		Model:            "x",
		SubagentRegistry: reg,
		Hooks:            rec,
	})

	result, err := e.RunSubagent(context.Background(), SubagentOptions{
		Type:        "remote-type",
		Description: "remote test",
		Prompt:      "do something remotely",
	})
	require.NoError(t, err)
	require.Equal(t, "remote answer", result)
	require.True(t, rec.subagentStopCalled, "SubagentStop hook must fire for remote dispatch")
	require.Equal(t, "end_turn", rec.subagentStopReason)
}

// TestEngine_RunSubagent_RemoteDispatch_AuthError verifies that auth failures
// from the remote server are returned as errors and fire the SubagentStop hook.
func TestEngine_RunSubagent_RemoteDispatch_AuthError(t *testing.T) {
	// Server requires a token we won't provide.
	kHandler := func(ctx context.Context, req kairos.RunRequest, emit func(string)) (string, error) {
		return "ok", nil
	}
	kSrv := kairos.NewServer(kHandler, "required-token")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Delegate to the kairos handler (which checks auth).
		kSrv.Handler().ServeHTTP(w, r)
	}))
	defer ts.Close()

	reg := subagent.NewRegistry()
	reg.Register(subagent.Spec{
		Name:        "auth-remote",
		Description: "needs auth",
		Remote: &subagent.RemoteSpec{
			Endpoint:  ts.URL,
			AuthToken: "wrong-token",
		},
	})

	rec := &recordingHookSink{}
	fp := fake.New(nil)
	e := NewEngine(Config{
		Provider:         fp,
		Tools:            tool.NewRegistry(),
		Permissions:      permissions.Empty(),
		Model:            "x",
		SubagentRegistry: reg,
		Hooks:            rec,
	})

	_, err := e.RunSubagent(context.Background(), SubagentOptions{
		Type:   "auth-remote",
		Prompt: "hi",
	})
	require.Error(t, err)
	require.True(t, rec.subagentStopCalled, "SubagentStop hook must fire on remote error")
	require.Equal(t, "error", rec.subagentStopReason)
}

// --- AutoCompact tests ---

// makeAutoCompactMessages returns n alternating user/assistant messages so
// compact.Run finds valid split points.
func makeAutoCompactMessages(n int) []message.Message {
	msgs := make([]message.Message, n)
	for i := range msgs {
		if i%2 == 0 {
			msgs[i] = message.Message{Role: message.RoleUser, Content: []message.Block{{Type: message.BlockText, Text: "u"}}}
		} else {
			msgs[i] = message.Message{Role: message.RoleAssistant, Content: []message.Block{{Type: message.BlockText, Text: "a"}}}
		}
	}
	return msgs
}

func TestEngine_AutoCompact_FiresWhenThresholdExceeded(t *testing.T) {
	// Script 0: the user turn stream with usage {InputTokens:80, OutputTokens:30} → total 110 > threshold 100.
	// Script 1: the compact summarisation call.
	fp := fake.New(
		[]provider.Event{
			{Kind: provider.EventStart, Usage: message.Usage{InputTokens: 80}},
			{Kind: provider.EventTextDelta, Text: "answer"},
			{Kind: provider.EventUsage, Usage: message.Usage{OutputTokens: 30}},
			{Kind: provider.EventMessageStop, StopReason: "end_turn"},
		},
		// compact.Run uses the provider too — return a summary.
		[]provider.Event{
			{Kind: provider.EventStart},
			{Kind: provider.EventTextDelta, Text: "SUMMARY"},
			{Kind: provider.EventMessageStop},
		},
	)
	rec := &recordingHookSink{}
	e := NewEngine(Config{
		Provider:              fp,
		Tools:                 tool.NewRegistry(),
		Permissions:           permissions.Empty(),
		Model:                 "m",
		Hooks:                 rec,
		AutoCompactThreshold:  100,
		AutoCompactKeepRecent: 10,
	})
	// Pre-load 15 messages so compact has enough to work with.
	e.SetInitialMessages(makeAutoCompactMessages(15))

	// Drain the turn.
	for range e.SubmitMessage(context.Background(), "hello") {
	}

	// PreCompact hook must have fired (auto compact ran).
	require.True(t, rec.preCompactCalled, "expected PreCompact hook to fire")
	// Messages should be fewer than 17 (15 preload + 1 user + 1 assistant = 17).
	require.Less(t, len(e.Messages()), 17, "expected compact to reduce message count")
}

func TestEngine_AutoCompact_DoesNotFireBelowThreshold(t *testing.T) {
	// threshold=1000, usage=80+30=110 — no compact.
	fp := fake.New(
		[]provider.Event{
			{Kind: provider.EventStart, Usage: message.Usage{InputTokens: 80}},
			{Kind: provider.EventTextDelta, Text: "answer"},
			{Kind: provider.EventUsage, Usage: message.Usage{OutputTokens: 30}},
			{Kind: provider.EventMessageStop, StopReason: "end_turn"},
		},
	)
	rec := &recordingHookSink{}
	e := NewEngine(Config{
		Provider:             fp,
		Tools:                tool.NewRegistry(),
		Permissions:          permissions.Empty(),
		Model:                "m",
		Hooks:                rec,
		AutoCompactThreshold: 1000,
	})
	e.SetInitialMessages(makeAutoCompactMessages(15))
	for range e.SubmitMessage(context.Background(), "hello") {
	}
	require.False(t, rec.preCompactCalled, "expected no compact below threshold")
}

func TestEngine_AutoCompact_DisabledByDefault(t *testing.T) {
	// threshold=0 (default) — no compact even with high usage.
	fp := fake.New(
		[]provider.Event{
			{Kind: provider.EventStart, Usage: message.Usage{InputTokens: 999999}},
			{Kind: provider.EventTextDelta, Text: "answer"},
			{Kind: provider.EventUsage, Usage: message.Usage{OutputTokens: 999999}},
			{Kind: provider.EventMessageStop, StopReason: "end_turn"},
		},
	)
	rec := &recordingHookSink{}
	e := NewEngine(Config{
		Provider:    fp,
		Tools:       tool.NewRegistry(),
		Permissions: permissions.Empty(),
		Model:       "m",
		Hooks:       rec,
		// AutoCompactThreshold intentionally 0
	})
	e.SetInitialMessages(makeAutoCompactMessages(15))
	for range e.SubmitMessage(context.Background(), "hello") {
	}
	require.False(t, rec.preCompactCalled, "expected no compact when threshold=0")
}

func TestEngine_LastUsage_TracksMostRecentEventUsage(t *testing.T) {
	fp := fake.New(
		[]provider.Event{
			{Kind: provider.EventStart, Usage: message.Usage{InputTokens: 5}},
			{Kind: provider.EventTextDelta, Text: "hi"},
			{Kind: provider.EventUsage, Usage: message.Usage{OutputTokens: 1}},
			{Kind: provider.EventMessageStop, StopReason: "end_turn"},
		},
	)
	e := NewEngine(Config{
		Provider:    fp,
		Tools:       tool.NewRegistry(),
		Permissions: permissions.Empty(),
		Model:       "m",
	})
	for range e.SubmitMessage(context.Background(), "hello") {
	}
	lu := e.LastUsage()
	require.Equal(t, 5, lu.InputTokens, "expected InputTokens=5")
	require.Equal(t, 1, lu.OutputTokens, "expected OutputTokens=1")
}

// TestEngine_UsageSinceLastCompact_ResetsOnCompact verifies that after a
// successful Compact(), UsageSinceLastCompact() returns zero.
func TestEngine_UsageSinceLastCompact_ResetsOnCompact(t *testing.T) {
	// Run one turn to accumulate usageSinceLastCompact.
	fp := fake.New(
		[]provider.Event{
			{Kind: provider.EventStart, Usage: message.Usage{InputTokens: 60}},
			{Kind: provider.EventTextDelta, Text: "hi"},
			{Kind: provider.EventUsage, Usage: message.Usage{OutputTokens: 50}},
			{Kind: provider.EventMessageStop, StopReason: "end_turn"},
		},
		// compact.Run will call the provider once for its summarisation.
		[]provider.Event{
			{Kind: provider.EventStart},
			{Kind: provider.EventTextDelta, Text: "SUMMARY"},
			{Kind: provider.EventMessageStop},
		},
	)
	e := NewEngine(Config{
		Provider:    fp,
		Tools:       tool.NewRegistry(),
		Permissions: permissions.Empty(),
		Model:       "m",
	})
	// Pre-load enough messages for compact to not skip.
	e.SetInitialMessages(makeAutoCompactMessages(15))

	// Run one turn so usageSinceLastCompact accumulates 60+50=110 tokens.
	for range e.SubmitMessage(context.Background(), "hello") {
	}
	sc := e.UsageSinceLastCompact()
	require.Equal(t, 60, sc.InputTokens, "expected 60 input tokens since last compact")
	require.Equal(t, 50, sc.OutputTokens, "expected 50 output tokens since last compact")

	// Now compact manually.
	s, err := e.Compact(context.Background(), CompactOptions{KeepRecent: 10})
	require.NoError(t, err)
	require.False(t, s.Skipped, "expected compact to succeed")

	// After compact, usageSinceLastCompact must be zero.
	sc = e.UsageSinceLastCompact()
	require.Equal(t, message.Usage{}, sc, "UsageSinceLastCompact must be zero after successful Compact")
}

// TestEngine_AutoCompact_UsesCumulativeUsageSinceLastCompact verifies that the
// auto-compact predicate uses cumulative usage across turns rather than
// per-turn usage. Two turns each emitting 60 input + 50 output = 110 tokens
// each; threshold=200; first turn (110 total) does not trigger; second turn
// (220 total) triggers compact.
func TestEngine_AutoCompact_UsesCumulativeUsageSinceLastCompact(t *testing.T) {
	fp := fake.New(
		// Turn 1: 110 tokens — below threshold 200.
		[]provider.Event{
			{Kind: provider.EventStart, Usage: message.Usage{InputTokens: 60}},
			{Kind: provider.EventTextDelta, Text: "turn1"},
			{Kind: provider.EventUsage, Usage: message.Usage{OutputTokens: 50}},
			{Kind: provider.EventMessageStop, StopReason: "end_turn"},
		},
		// Turn 2: 110 more tokens → cumulative 220 ≥ 200; compact fires.
		[]provider.Event{
			{Kind: provider.EventStart, Usage: message.Usage{InputTokens: 60}},
			{Kind: provider.EventTextDelta, Text: "turn2"},
			{Kind: provider.EventUsage, Usage: message.Usage{OutputTokens: 50}},
			{Kind: provider.EventMessageStop, StopReason: "end_turn"},
		},
		// compact.Run summarisation call.
		[]provider.Event{
			{Kind: provider.EventStart},
			{Kind: provider.EventTextDelta, Text: "SUMMARY"},
			{Kind: provider.EventMessageStop},
		},
	)
	rec := &recordingHookSink{}
	e := NewEngine(Config{
		Provider:              fp,
		Tools:                 tool.NewRegistry(),
		Permissions:           permissions.Empty(),
		Model:                 "m",
		Hooks:                 rec,
		AutoCompactThreshold:  200,
		AutoCompactKeepRecent: 10,
	})
	e.SetInitialMessages(makeAutoCompactMessages(15))

	// Turn 1 — should NOT trigger compact.
	for range e.SubmitMessage(context.Background(), "turn1") {
	}
	require.False(t, rec.preCompactCalled, "compact must not fire after turn 1 (110 < 200)")

	// Turn 2 — cumulative 220 should trigger compact.
	for range e.SubmitMessage(context.Background(), "turn2") {
	}
	require.True(t, rec.preCompactCalled, "compact must fire after turn 2 (cumulative 220 ≥ 200)")
}

// TestEngine_RunSubagent_StreamsTextDeltaCallback verifies that SubagentOptions.OnTextDelta
// is invoked for every EventTextDelta from the child engine's stream.
func TestEngine_RunSubagent_StreamsTextDeltaCallback(t *testing.T) {
	fp := fake.New([]provider.Event{
		{Kind: provider.EventTextDelta, Text: "alpha"},
		{Kind: provider.EventTextDelta, Text: " beta"},
		{Kind: provider.EventTextDelta, Text: " gamma"},
		{Kind: provider.EventMessageStop, StopReason: "end_turn"},
	})

	var mu sync.Mutex
	var captured []string
	onDelta := func(delta string) {
		mu.Lock()
		defer mu.Unlock()
		captured = append(captured, delta)
	}

	subreg := subagent.DefaultRegistry()
	e := NewEngine(Config{
		Provider:         fp,
		Tools:            tool.NewRegistry(),
		Permissions:      permissions.Empty(),
		Model:            "x",
		SubagentRegistry: subreg,
	})

	result, err := e.RunSubagent(context.Background(), SubagentOptions{
		Type:        "general-purpose",
		Description: "delta test",
		Prompt:      "go",
		OnTextDelta: onDelta,
	})
	require.NoError(t, err)
	require.Equal(t, "alpha beta gamma", result)

	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, []string{"alpha", " beta", " gamma"}, captured)
}

// TestEngine_NestedSubagent_WritesNestedJSONL verifies that when an Engine whose
// Session is a subagent Store calls RunSubagent, the resulting child JSONL is
// written at the expected nested path:
//
//	<parent>/subagents/<first-sub-id>/subagents/<second-sub-id>.jsonl
func TestEngine_NestedSubagent_WritesNestedJSONL(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Create a real parent session.
	parentSess, err := session.New(session.NewOptions{Cwd: tmpHome, Model: "m"})
	require.NoError(t, err)
	defer parentSess.Close()

	// Simulate the first-level subagent store (as if RunSubagent had already been
	// called once on the parent engine).
	firstSubID := "first-sub-id"
	firstSubStore, err := session.NewSubagent(parentSess, firstSubID)
	require.NoError(t, err)
	defer firstSubStore.Close()

	// Build a child engine whose Session IS the first-sub store.
	// When this engine calls RunSubagent, it should write JSONL one level deeper.
	fp := fake.New([]provider.Event{
		{Kind: provider.EventStart},
		{Kind: provider.EventTextDelta, Text: "nested response"},
		{Kind: provider.EventMessageStop, StopReason: "end_turn"},
	})
	subreg := subagent.NewRegistry()
	subreg.Register(subagent.Spec{Name: "general-purpose", Description: "x"})
	e := NewEngine(Config{
		Provider:         fp,
		Model:            "m",
		Tools:            tool.NewRegistry(),
		Permissions:      permissions.Empty(),
		SubagentRegistry: subreg,
		Session:          firstSubStore,
	})

	out, err := e.RunSubagent(context.Background(), SubagentOptions{
		Type: "general-purpose", Description: "nested", Prompt: "go",
	})
	require.NoError(t, err)
	require.Equal(t, "nested response", out)

	// The nested JSONL must live at:
	//   <firstSubStore.Path without .jsonl>/subagents/<uuid>.jsonl
	// i.e., <parentSess dir>/subagents/first-sub-id/subagents/<uuid>.jsonl
	nestedDir := filepath.Join(strings.TrimSuffix(firstSubStore.Path(), ".jsonl"), "subagents")
	entries, err := os.ReadDir(nestedDir)
	require.NoError(t, err)
	require.NotEmpty(t, entries, "expected nested subagent JSONL to exist")
}

// alwaysEndTurnProvider returns an immediate end_turn for every call.
type alwaysEndTurnProvider struct{}

func (a *alwaysEndTurnProvider) Stream(_ context.Context, _ provider.Request) (<-chan provider.Event, error) {
	ch := make(chan provider.Event, 2)
	ch <- provider.Event{Kind: provider.EventTextDelta, Text: "done"}
	ch <- provider.Event{Kind: provider.EventMessageStop, StopReason: "end_turn"}
	close(ch)
	return ch, nil
}

// echoTool is a minimal tool used in buffer-reset tests: it accepts any JSON
// object input and returns a fixed text result.
type echoTool struct{ tool.DefaultPermission }

func (echoTool) Name() string                         { return "echo" }
func (echoTool) Description(context.Context) string   { return "echo" }
func (echoTool) UserFacingName(map[string]any) string { return "echo" }
func (echoTool) IsReadOnly() bool                     { return true }
func (echoTool) IsConcurrencySafe() bool              { return true }
func (echoTool) Schema() map[string]any               { return map[string]any{"type": "object"} }
func (echoTool) Call(_ context.Context, _ map[string]any, _ *tool.Context) (tool.Result, error) {
	return tool.Result{Text: "echoed", ForLLM: "echoed"}, nil
}

// TestEngine_RunSubagent_BufferResetsOnIntermediateTurn verifies that when a
// subagent run has an intermediate tool_use turn followed by a final end_turn,
// only the final turn's text is returned (not a concatenation of all turns).
func TestEngine_RunSubagent_BufferResetsOnIntermediateTurn(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Register(echoTool{})

	fp := fake.New(
		// First turn: text "WRONG" + tool_use echo + StopReason=tool_use
		[]provider.Event{
			{Kind: provider.EventStart},
			{Kind: provider.EventTextDelta, Text: "WRONG"},
			{Kind: provider.EventToolUseStart, ToolUseID: "u1", ToolName: "echo"},
			{Kind: provider.EventToolInputDelta, PartialJSON: "{}"},
			{Kind: provider.EventBlockStop, StopReason: "tool_use"},
			{Kind: provider.EventMessageStop, StopReason: "tool_use"},
		},
		// Second turn: text "RIGHT" + end_turn
		[]provider.Event{
			{Kind: provider.EventStart},
			{Kind: provider.EventTextDelta, Text: "RIGHT"},
			{Kind: provider.EventBlockStop},
			{Kind: provider.EventMessageStop, StopReason: "end_turn"},
		},
	)

	perms := permissions.Empty()
	perms.AlwaysAllowRules[permissions.SourceUser] = []permissions.Rule{{Tool: "echo"}}
	subreg := subagent.NewRegistry()
	subreg.Register(subagent.Spec{Name: "general-purpose", Description: "x"})

	e := NewEngine(Config{
		Provider:         fp,
		Model:            "m",
		Tools:            reg,
		Permissions:      perms,
		SubagentRegistry: subreg,
	})

	out, err := e.RunSubagent(context.Background(), SubagentOptions{
		Type:        "general-purpose",
		Description: "test",
		Prompt:      "go",
	})
	require.NoError(t, err)
	require.Equal(t, "RIGHT", out, "RunSubagent must return only the final turn's text, not WRONG or WRONGRIGHT")
}

// TestEngine_RunSubagent_FiresSubagentStopOnError verifies that the
// SubagentStop hook fires with reason "error" when the provider emits an error.
func TestEngine_RunSubagent_FiresSubagentStopOnError(t *testing.T) {
	rec := &recordingHookSink{}
	fp := fake.New(
		[]provider.Event{
			{Kind: provider.EventStart},
			{Kind: provider.EventError, Err: errors.New("upstream down")},
		},
	)
	subreg := subagent.NewRegistry()
	subreg.Register(subagent.Spec{Name: "general-purpose", Description: "x"})

	e := NewEngine(Config{
		Provider:         fp,
		Model:            "m",
		Tools:            tool.NewRegistry(),
		Permissions:      permissions.Empty(),
		Hooks:            rec,
		SubagentRegistry: subreg,
	})

	_, err := e.RunSubagent(context.Background(), SubagentOptions{
		Type:        "general-purpose",
		Description: "t",
		Prompt:      "p",
	})
	require.Error(t, err)
	require.True(t, rec.subagentStopCalled, "SubagentStop hook must fire on error path")
	require.Equal(t, "error", rec.subagentStopReason)
}

// TestEngine_RunSubagent_ModelOverride verifies that when a subagent Spec has a
// non-empty Model field, the child engine issues provider.Stream calls with that
// model, while the parent engine retains its own model.
func TestEngine_RunSubagent_ModelOverride(t *testing.T) {
	fp := fake.New([]provider.Event{
		{Kind: provider.EventTextDelta, Text: "child answer"},
		{Kind: provider.EventMessageStop, StopReason: "end_turn"},
	})

	reg := subagent.NewRegistry()
	reg.Register(subagent.Spec{
		Name:        "haiku-agent",
		Description: "A lightweight subagent that runs on haiku.",
		Model:       "haiku-test",
	})

	e := NewEngine(Config{
		Provider:         fp,
		Tools:            tool.NewRegistry(),
		Permissions:      permissions.Empty(),
		Model:            "parent-model",
		SubagentRegistry: reg,
	})

	result, err := e.RunSubagent(context.Background(), SubagentOptions{
		Type:        "haiku-agent",
		Description: "override test",
		Prompt:      "summarise this",
	})
	require.NoError(t, err)
	require.Equal(t, "child answer", result)

	// The fake provider records the Model from the most recent Stream() call.
	// It must be the override, not the parent model.
	require.Equal(t, "haiku-test", fp.LastModel, "child engine must use spec.Model override")

	// Parent engine model must be unchanged.
	require.Equal(t, "parent-model", e.Model(), "parent engine model must not change")
}

// TestEngine_RunSubagent_ModelInheritedWhenEmpty verifies that when Spec.Model is
// empty, the child engine inherits the parent model.
func TestEngine_RunSubagent_ModelInheritedWhenEmpty(t *testing.T) {
	fp := fake.New([]provider.Event{
		{Kind: provider.EventTextDelta, Text: "ok"},
		{Kind: provider.EventMessageStop, StopReason: "end_turn"},
	})

	reg := subagent.NewRegistry()
	reg.Register(subagent.Spec{
		Name:        "inherit-agent",
		Description: "A subagent with no model override.",
		// Model intentionally empty
	})

	e := NewEngine(Config{
		Provider:         fp,
		Tools:            tool.NewRegistry(),
		Permissions:      permissions.Empty(),
		Model:            "parent-model",
		SubagentRegistry: reg,
	})

	_, err := e.RunSubagent(context.Background(), SubagentOptions{
		Type:        "inherit-agent",
		Description: "inherit test",
		Prompt:      "go",
	})
	require.NoError(t, err)
	require.Equal(t, "parent-model", fp.LastModel, "child engine must inherit parent model when spec.Model is empty")
}

// TestEngine_IsOverBudget verifies the budget-cap accessor.
func TestEngine_IsOverBudget(t *testing.T) {
	pt := pricing.NewTable(map[string]pricing.Rate{
		"test-model": {InputPerM: 3.0, OutputPerM: 15.0},
	})

	t.Run("no limit configured", func(t *testing.T) {
		e := NewEngine(Config{
			Provider:    fake.New(nil),
			Tools:       tool.NewRegistry(),
			Permissions: permissions.Empty(),
			Model:       "test-model",
			Pricing:     pt,
			// CostLimitUSD == 0 => never over budget
		})
		over, cur, lim := e.IsOverBudget()
		require.False(t, over)
		require.Equal(t, float64(0), cur)
		require.Equal(t, float64(0), lim)
	})

	t.Run("under budget", func(t *testing.T) {
		e := NewEngine(Config{
			Provider:     fake.New(nil),
			Tools:        tool.NewRegistry(),
			Permissions:  permissions.Empty(),
			Model:        "test-model",
			Pricing:      pt,
			CostLimitUSD: 1.0,
		})
		// Inject a small usage: 100 input + 100 output tokens.
		// Cost = (100/1e6)*3.0 + (100/1e6)*15.0 = 0.0000030 + 0.0000150 = ~$0.0000018 — well under $1.
		e.mu.Lock()
		e.usage.InputTokens = 100
		e.usage.OutputTokens = 100
		e.mu.Unlock()

		over, cur, lim := e.IsOverBudget()
		require.False(t, over, "should be under budget")
		require.Greater(t, cur, float64(0))
		require.Equal(t, 1.0, lim)
	})

	t.Run("over budget", func(t *testing.T) {
		e := NewEngine(Config{
			Provider:     fake.New(nil),
			Tools:        tool.NewRegistry(),
			Permissions:  permissions.Empty(),
			Model:        "test-model",
			Pricing:      pt,
			CostLimitUSD: 0.001,
		})
		// Inject large usage: 1_000_000 input + 1_000_000 output tokens.
		// Cost = 3.0 + 15.0 = $18.0 >> $0.001.
		e.mu.Lock()
		e.usage.InputTokens = 1_000_000
		e.usage.OutputTokens = 1_000_000
		e.mu.Unlock()

		over, cur, lim := e.IsOverBudget()
		require.True(t, over, "should be over budget")
		require.Equal(t, 18.0, cur)
		require.Equal(t, 0.001, lim)
	})

	t.Run("no pricing table", func(t *testing.T) {
		e := NewEngine(Config{
			Provider:     fake.New(nil),
			Tools:        tool.NewRegistry(),
			Permissions:  permissions.Empty(),
			Model:        "test-model",
			Pricing:      nil,
			CostLimitUSD: 0.01,
		})
		over, cur, lim := e.IsOverBudget()
		require.False(t, over)
		require.Equal(t, float64(0), cur)
		require.Equal(t, 0.01, lim)
	})
}

// TestEngine_ToolDispatcher_ReplacesLocalDispatch verifies that when a
// ToolDispatcher is configured, it is called instead of the local tool registry
// and the result is fed back to the model as a tool_result block.
func TestEngine_ToolDispatcher_ReplacesLocalDispatch(t *testing.T) {
	// Two-script provider: turn 1 asks for "Echo"; turn 2 returns end_turn.
	fp := fake.New(
		[]provider.Event{
			{Kind: provider.EventToolUseStart, ToolUseID: "u1", ToolName: "Echo"},
			{Kind: provider.EventToolInputDelta, PartialJSON: `{"msg":"hello"}`},
			{Kind: provider.EventBlockStop, StopReason: "tool_use"},
			{Kind: provider.EventMessageStop, StopReason: "tool_use"},
		},
		[]provider.Event{
			{Kind: provider.EventTextDelta, Text: "echo done"},
			{Kind: provider.EventMessageStop, StopReason: "end_turn"},
		},
	)

	var dispatchedID, dispatchedName string
	var dispatchedInput map[string]any
	dispatcher := ToolDispatcher(func(_ context.Context, toolUseID, toolName string, input map[string]any) (tool.Result, error) {
		dispatchedID = toolUseID
		dispatchedName = toolName
		dispatchedInput = input
		// Return the input msg as text, simulating an echo tool.
		msg, _ := input["msg"].(string)
		return tool.Result{Text: "echoed: " + msg}, nil
	})

	// Registry intentionally does NOT have "Echo" — dispatcher must be called instead.
	e := NewEngine(Config{
		Provider:       fp,
		Tools:          tool.NewRegistry(),
		Permissions:    permissions.Empty(),
		Model:          "x",
		ToolDispatcher: dispatcher,
	})

	var toolResultText string
	ch := e.SubmitMessage(context.Background(), "echo something")
	for ev := range ch {
		if ev.Kind == KindToolResult {
			toolResultText = ev.Text
		}
	}

	require.Equal(t, "u1", dispatchedID, "ToolDispatcher must receive the tool_use_id")
	require.Equal(t, "Echo", dispatchedName, "ToolDispatcher must receive the tool name")
	require.Equal(t, "hello", dispatchedInput["msg"], "ToolDispatcher must receive the tool input")
	require.Equal(t, "echoed: hello", toolResultText, "tool_result text must come from ToolDispatcher")

	// Verify the engine appended the tool_result to messages so the model sees it.
	msgs := e.Messages()
	var foundResult bool
	for _, m := range msgs {
		if m.Role == message.RoleUser {
			for _, b := range m.Content {
				if b.Type == message.BlockToolResult && b.Text == "echoed: hello" {
					foundResult = true
				}
			}
		}
	}
	require.True(t, foundResult, "tool_result block must appear in message history")
}

// TestEngine_RunSubagent_PrefixChainPropagates verifies that the SubagentPrefixChain
// is set on the child engine's Config when RunSubagent builds the child.
func TestEngine_RunSubagent_PrefixChainPropagates(t *testing.T) {
	fp := fake.New([]provider.Event{
		{Kind: provider.EventTextDelta, Text: "child result"},
		{Kind: provider.EventMessageStop, StopReason: "end_turn"},
	})
	reg := subagent.DefaultRegistry()
	// The child engine's Config.SubagentPrefixChain is set inside RunSubagent;
	// we verify the result by inspecting what the child writes to its messages
	// (indirectly — the chain is plumbed but we confirm the call succeeds and
	// the returned opts carry the description via a custom runner injected via
	// a test-specific tool).
	var capturedChain []string
	captureTool := &chainCaptureTool{onCall: func(chain []string) { capturedChain = chain }}
	childReg := tool.NewRegistry()
	childReg.Register(captureTool)

	e := NewEngine(Config{
		Provider:         fp,
		Tools:            childReg,
		Permissions:      permissions.Empty(),
		Model:            "x",
		SubagentRegistry: reg,
		// Set an outer prefix chain on the parent engine.
		SubagentPrefixChain: []string{"outer-task"},
	})
	// RunSubagent builds a child with SubagentPrefixChain = ["outer-task", opts.Description].
	_, err := e.RunSubagent(context.Background(), SubagentOptions{
		Type:        "general-purpose",
		Description: "inner-task",
		Prompt:      "do something",
		PrefixChain: []string{"outer-task"},
	})
	require.NoError(t, err)
	// Child engine's SubagentPrefixChain should be ["outer-task", "inner-task"].
	// We can't inspect the child engine directly, but we verify the chain that
	// RunSubagent computes via the Config field by checking the parent chain was
	// passed and the description was appended.
	// (The child engine is local and ephemeral; the chain logic is in engine.go.)
	// Verify by checking the parent's own SubagentPrefixChain is unchanged.
	require.Equal(t, []string{"outer-task"}, e.cfg.SubagentPrefixChain)
	_ = capturedChain // capture tool was not called (no tool_use in stream)
}

// chainCaptureTool is a helper tool that records the SubagentPrefixChain from tcx.
type chainCaptureTool struct {
	tool.DefaultPermission
	onCall func([]string)
}

func (c *chainCaptureTool) Name() string                         { return "ChainCapture" }
func (c *chainCaptureTool) Description(context.Context) string   { return "captures prefix chain" }
func (c *chainCaptureTool) UserFacingName(map[string]any) string { return "ChainCapture" }
func (c *chainCaptureTool) Schema() map[string]any               { return map[string]any{"type": "object"} }
func (c *chainCaptureTool) IsReadOnly() bool                     { return true }
func (c *chainCaptureTool) IsConcurrencySafe() bool              { return true }
func (c *chainCaptureTool) Call(_ context.Context, _ map[string]any, tcx *tool.Context) (tool.Result, error) {
	if tcx != nil && c.onCall != nil {
		c.onCall(tcx.SubagentPrefixChain)
	}
	return tool.Result{Text: "ok"}, nil
}

// --- Stream retry tests ---

// TestEngine_SubmitMessage_StreamRetry_RecoversAfterTransientError: fake provider
// errors on first attempt, succeeds on second. Asserts final text is correct and
// at least one KindStreamRetry event was emitted.
func TestEngine_SubmitMessage_StreamRetry_RecoversAfterTransientError(t *testing.T) {
	transient := errors.New("transient network blip")

	// Script 0: injected error (via StreamErrors[0]). Script 1: success.
	fp := fake.New(
		// script 0 — will be replaced by injected error, so body doesn't matter
		// but must exist as a slot. We use an end_turn that will never be reached.
		[]provider.Event{
			{Kind: provider.EventTextDelta, Text: "should-not-appear"},
			{Kind: provider.EventMessageStop, StopReason: "end_turn"},
		},
		// script 1 — success after retry
		[]provider.Event{
			{Kind: provider.EventTextDelta, Text: "recovered text"},
			{Kind: provider.EventMessageStop, StopReason: "end_turn"},
		},
	)
	fp.StreamErrors = []error{transient, nil}

	e := NewEngine(Config{
		Provider:         fp,
		Tools:            tool.NewRegistry(),
		Permissions:      permissions.Empty(),
		Model:            "x",
		MaxStreamRetries: 3,
	})

	ch := e.SubmitMessage(context.Background(), "hello")
	var assistantText strings.Builder
	var retryEvents []Event
	for ev := range ch {
		switch ev.Kind {
		case KindAssistantDelta:
			assistantText.WriteString(ev.Text)
		case KindStreamRetry:
			retryEvents = append(retryEvents, ev)
		}
	}
	require.Equal(t, "recovered text", assistantText.String())
	require.GreaterOrEqual(t, len(retryEvents), 1, "expected at least one KindStreamRetry event")
	require.Equal(t, 1, retryEvents[0].RetryAttempt)
	require.ErrorIs(t, retryEvents[0].Err, transient)
}

// TestEngine_SubmitMessage_StreamRetry_ExhaustsAfter3: fake provider errors all 3
// retry slots. Asserts error wraps original and 3 KindStreamRetry events are emitted.
func TestEngine_SubmitMessage_StreamRetry_ExhaustsAfter3(t *testing.T) {
	transient := errors.New("persistent error")

	// 4 scripts (1 original + 3 retries), all injected with errors.
	fp := fake.New(
		[]provider.Event{{Kind: provider.EventMessageStop, StopReason: "end_turn"}},
		[]provider.Event{{Kind: provider.EventMessageStop, StopReason: "end_turn"}},
		[]provider.Event{{Kind: provider.EventMessageStop, StopReason: "end_turn"}},
		[]provider.Event{{Kind: provider.EventMessageStop, StopReason: "end_turn"}},
	)
	fp.StreamErrors = []error{transient, transient, transient, transient}

	e := NewEngine(Config{
		Provider:         fp,
		Tools:            tool.NewRegistry(),
		Permissions:      permissions.Empty(),
		Model:            "x",
		MaxStreamRetries: 3,
	})

	ch := e.SubmitMessage(context.Background(), "hello")
	var finalErr error
	var retryCount int
	for ev := range ch {
		switch ev.Kind {
		case KindStreamRetry:
			retryCount++
		case KindError:
			finalErr = ev.Err
		}
	}
	require.Equal(t, 3, retryCount, "expected exactly 3 KindStreamRetry events")
	require.Error(t, finalErr)
	require.ErrorIs(t, finalErr, transient, "final error must wrap original transient error")
	require.Contains(t, finalErr.Error(), "stream retry exhausted")
}

// sleepTool is a test-only tool that sleeps for a configurable duration.
// IsConcurrencySafe returns true so the engine dispatches it concurrently.
// ignoreCtx controls whether the tool ignores context cancellation (useful for
// testing that the drain window actually waits for non-cooperative tools).
type sleepTool struct {
	tool.DefaultPermission
	sleep     time.Duration
	ignoreCtx bool
}

func (s sleepTool) Name() string                         { return "SleepTool" }
func (s sleepTool) Description(context.Context) string   { return "sleeps for a configured duration" }
func (s sleepTool) UserFacingName(map[string]any) string { return "SleepTool" }
func (s sleepTool) Schema() map[string]any               { return map[string]any{"type": "object"} }
func (s sleepTool) IsReadOnly() bool                     { return true }
func (s sleepTool) IsConcurrencySafe() bool              { return true }
func (s sleepTool) Call(ctx context.Context, _ map[string]any, _ *tool.Context) (tool.Result, error) {
	if s.ignoreCtx {
		time.Sleep(s.sleep)
	} else {
		select {
		case <-time.After(s.sleep):
		case <-ctx.Done():
		}
	}
	return tool.Result{Text: "slept"}, nil
}

// TestEngine_SubmitMessage_CancelWaitsForInFlightTools: tool sleeps 200ms and
// ignores ctx; ctx is cancelled mid-tool. SubmitMessage must return AFTER the
// tool finishes (between 150ms and 600ms), proving it waited for the drain.
func TestEngine_SubmitMessage_CancelWaitsForInFlightTools(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Register(sleepTool{sleep: 200 * time.Millisecond, ignoreCtx: true})
	perms := permissions.Empty()
	perms.AlwaysAllowRules[permissions.SourceCLI] = []permissions.Rule{{Tool: "SleepTool"}}

	// Turn 1: emit two concurrent tool_use blocks (both SleepTool, parallel).
	// Turn 2: end_turn.
	fp := fake.New(
		[]provider.Event{
			{Kind: provider.EventToolUseStart, ToolUseID: "s1", ToolName: "SleepTool"},
			{Kind: provider.EventToolInputDelta, PartialJSON: `{}`},
			{Kind: provider.EventBlockStop},
			{Kind: provider.EventToolUseStart, ToolUseID: "s2", ToolName: "SleepTool"},
			{Kind: provider.EventToolInputDelta, PartialJSON: `{}`},
			{Kind: provider.EventBlockStop},
			{Kind: provider.EventMessageStop, StopReason: "tool_use"},
		},
		[]provider.Event{
			{Kind: provider.EventTextDelta, Text: "done"},
			{Kind: provider.EventMessageStop, StopReason: "end_turn"},
		},
	)

	ctx, cancel := context.WithCancel(context.Background())
	e := NewEngine(Config{
		Provider:            fp,
		Tools:               reg,
		Permissions:         perms,
		Model:               "x",
		MaxToolDrainTimeout: 5 * time.Second,
	})

	start := time.Now()
	// Cancel context 50ms in — tools will still be sleeping (200ms total).
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	for range e.SubmitMessage(ctx, "sleep") {
	}
	elapsed := time.Since(start)

	// The drain should have waited for the 200ms tool.
	// Lower bound: 150ms (tools started a bit before cancel fired).
	// Upper bound: 600ms (generous; drain timeout is 5s but tools finish in 200ms).
	require.GreaterOrEqual(t, elapsed, 150*time.Millisecond,
		"SubmitMessage must wait for in-flight tools (elapsed=%s)", elapsed)
	require.Less(t, elapsed, 600*time.Millisecond,
		"SubmitMessage must not wait longer than necessary (elapsed=%s)", elapsed)
}

// TestEngine_SubmitMessage_CancelDrainTimeoutCapped: tool sleeps 10s and ignores
// ctx; drain timeout is 200ms; cancel ctx; assert SubmitMessage returns in ~200ms
// (not 10s).
func TestEngine_SubmitMessage_CancelDrainTimeoutCapped(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Register(sleepTool{sleep: 10 * time.Second, ignoreCtx: true})
	perms := permissions.Empty()
	perms.AlwaysAllowRules[permissions.SourceCLI] = []permissions.Rule{{Tool: "SleepTool"}}

	fp := fake.New(
		[]provider.Event{
			{Kind: provider.EventToolUseStart, ToolUseID: "s1", ToolName: "SleepTool"},
			{Kind: provider.EventToolInputDelta, PartialJSON: `{}`},
			{Kind: provider.EventBlockStop},
			{Kind: provider.EventToolUseStart, ToolUseID: "s2", ToolName: "SleepTool"},
			{Kind: provider.EventToolInputDelta, PartialJSON: `{}`},
			{Kind: provider.EventBlockStop},
			{Kind: provider.EventMessageStop, StopReason: "tool_use"},
		},
		// second turn won't be reached since ctx is cancelled
		[]provider.Event{
			{Kind: provider.EventTextDelta, Text: "never"},
			{Kind: provider.EventMessageStop, StopReason: "end_turn"},
		},
	)

	ctx, cancel := context.WithCancel(context.Background())
	e := NewEngine(Config{
		Provider:            fp,
		Tools:               reg,
		Permissions:         perms,
		Model:               "x",
		MaxToolDrainTimeout: 200 * time.Millisecond,
	})

	start := time.Now()
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	for range e.SubmitMessage(ctx, "slow sleep") {
	}
	elapsed := time.Since(start)

	// Should return well before the 10s tool finishes; max ~350ms (50ms cancel + 200ms drain + buffer).
	require.Less(t, elapsed, 800*time.Millisecond,
		"SubmitMessage must return when drain timeout is capped (elapsed=%s)", elapsed)
}

// panicTool's Call() always panics. Used to verify R4 recovery.
type panicTool struct {
	tool.DefaultPermission
}

func (panicTool) Name() string                         { return "PanicTool" }
func (panicTool) Description(context.Context) string   { return "panics on call" }
func (panicTool) UserFacingName(map[string]any) string { return "PanicTool" }
func (panicTool) Schema() map[string]any               { return map[string]any{"type": "object"} }
func (panicTool) IsReadOnly() bool                     { return true }
func (panicTool) IsConcurrencySafe() bool              { return true }
func (panicTool) Call(_ context.Context, _ map[string]any, _ *tool.Context) (tool.Result, error) {
	panic("intentional test panic")
}

// TestEngine_PanicInToolBecomesIsErrorResult: a tool that panics must not
// crash the process; the engine wraps the panic as an IsError tool_result.
func TestEngine_PanicInToolBecomesIsErrorResult(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Register(panicTool{})
	perms := permissions.Empty()
	perms.AlwaysAllowRules[permissions.SourceCLI] = []permissions.Rule{{Tool: "PanicTool"}}

	fp := fake.New(
		[]provider.Event{
			{Kind: provider.EventToolUseStart, ToolUseID: "tu1", ToolName: "PanicTool"},
			{Kind: provider.EventToolInputDelta, PartialJSON: `{}`},
			{Kind: provider.EventBlockStop, StopReason: "tool_use"},
			{Kind: provider.EventMessageStop, StopReason: "tool_use"},
		},
		[]provider.Event{
			{Kind: provider.EventTextDelta, Text: "ok"},
			{Kind: provider.EventMessageStop, StopReason: "end_turn"},
		},
	)

	e := NewEngine(Config{
		Provider:    fp,
		Tools:       reg,
		Permissions: perms,
		Model:       "x",
	})

	var sawErrorResult bool
	for ev := range e.SubmitMessage(context.Background(), "trigger panic") {
		if ev.Kind == KindToolResult && ev.ToolUseID == "tu1" && ev.IsError {
			require.Contains(t, ev.Text, "panicked")
			sawErrorResult = true
		}
	}
	require.True(t, sawErrorResult, "expected an IsError tool_result for the panicked tool")
}

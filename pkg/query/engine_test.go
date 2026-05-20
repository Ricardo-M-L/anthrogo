package query

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/pkg/message"
	"github.com/ricardo/anthrogo/pkg/permissions"
	"github.com/ricardo/anthrogo/pkg/provider"
	"github.com/ricardo/anthrogo/pkg/provider/fake"
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
	perms.HookDecide = func(toolName string, input map[string]any) permissions.HookOutcome {
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

// recordingHookSink records FirePreCompact calls for compact tests.
type recordingHookSink struct {
	preCompactCalled bool
}

func (r *recordingHookSink) FirePostToolUse(_ context.Context, _ string, _, _ map[string]any) string {
	return ""
}
func (r *recordingHookSink) FireStop(_ context.Context, _ string) {}
func (r *recordingHookSink) FirePreCompact(_ context.Context, _ string) {
	r.preCompactCalled = true
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

	d := permissions.Decide(perms, "Skill", map[string]any{"skill": "git-flow"})
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

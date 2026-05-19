package query

import (
	"context"
	"strings"
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

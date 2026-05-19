package query

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/pkg/permissions"
	"github.com/ricardo/anthrogo/pkg/provider"
	"github.com/ricardo/anthrogo/pkg/provider/fake"
	"github.com/ricardo/anthrogo/pkg/tool"
)

func TestEngine_TracksPermissionDenials(t *testing.T) {
	fp := fake.New(
		[]provider.Event{
			{Kind: provider.EventToolUseStart, ToolUseID: "u1", ToolName: "Bash"},
			{Kind: provider.EventToolInputDelta, PartialJSON: `{"command":"rm -rf /"}`},
			{Kind: provider.EventBlockStop, StopReason: "tool_use"},
			{Kind: provider.EventMessageStop, StopReason: "tool_use"},
		},
		[]provider.Event{
			{Kind: provider.EventTextDelta, Text: "ok"},
			{Kind: provider.EventMessageStop, StopReason: "end_turn"},
		},
	)
	reg := tool.NewRegistry()
	reg.Register(tool.Bash{})
	perms := permissions.Empty()
	perms.AlwaysDenyRules[permissions.SourceCLI] = []permissions.Rule{{Tool: "Bash", Pattern: "rm -rf*"}}
	perms.ShouldAvoidPrompts = true

	e := NewEngine(Config{Provider: fp, Tools: reg, Permissions: perms, Model: "x"})
	for range e.SubmitMessage(context.Background(), "delete everything") {
	}
	dens := e.Denials()
	require.Len(t, dens, 1)
	require.Equal(t, "Bash", dens[0].ToolName)
	require.Equal(t, "u1", dens[0].ToolUseID)
	require.Equal(t, "rm -rf /", dens[0].ToolInput["command"])
}

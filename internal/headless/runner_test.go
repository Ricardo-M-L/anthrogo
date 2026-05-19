package headless

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/pkg/permissions"
	"github.com/ricardo/anthrogo/pkg/provider"
	"github.com/ricardo/anthrogo/pkg/provider/fake"
	"github.com/ricardo/anthrogo/pkg/tool"
)

func TestRunner_PrintsAssistantTextAndExits(t *testing.T) {
	fp := fake.New([]provider.Event{
		{Kind: provider.EventTextDelta, Text: "hello "},
		{Kind: provider.EventTextDelta, Text: "world"},
		{Kind: provider.EventMessageStop, StopReason: "end_turn"},
	})
	var out bytes.Buffer
	err := Run(context.Background(), Options{
		Prompt:       "hi",
		Provider:     fp,
		Tools:        tool.NewRegistry(),
		Permissions:  permissions.Empty(),
		Stdout:       &out,
		Model:        "x",
		SystemPrompt: "you are a helper",
	})
	require.NoError(t, err)
	require.Equal(t, "hello world\n", out.String())
}

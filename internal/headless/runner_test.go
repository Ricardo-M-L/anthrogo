package headless

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/pkg/message"
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

func TestHeadlessRun_JSONMode(t *testing.T) {
	fp := fake.New([]provider.Event{
		{Kind: provider.EventStart},
		{Kind: provider.EventTextDelta, Text: "hi"},
		{Kind: provider.EventMessageStop, StopReason: "end_turn"},
		{Kind: provider.EventUsage, Usage: message.Usage{InputTokens: 5, OutputTokens: 1}},
	})
	var stdout bytes.Buffer
	err := Run(context.Background(), Options{
		Prompt:      "test",
		Model:       "m",
		Provider:    fp,
		Permissions: permissions.Empty(),
		Tools:       tool.NewRegistry(),
		Stdout:      &stdout,
		Stderr:      io.Discard,
		JSON:        true,
	})
	require.NoError(t, err)

	lines := strings.Split(strings.TrimRight(stdout.String(), "\n"), "\n")
	var seenText, seenStop, seenUsage bool
	for _, l := range lines {
		if l == "" {
			continue
		}
		var m map[string]any
		require.NoError(t, json.Unmarshal([]byte(l), &m), "invalid JSON line: %s", l)
		switch m["kind"] {
		// query engine translates provider.EventTextDelta → KindAssistantDelta
		case "assistant_delta":
			seenText = true
		// query engine translates provider.EventMessageStop → KindTurnComplete
		case "turn_complete":
			seenStop = true
		case "usage":
			seenUsage = true
		}
	}
	require.True(t, seenText, "expected assistant_delta event in JSON output")
	require.True(t, seenStop, "expected turn_complete event in JSON output")
	require.True(t, seenUsage, "expected usage event in JSON output")
}

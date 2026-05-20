package builtins

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/pkg/message"
	"github.com/ricardo/anthrogo/pkg/provider"
	"github.com/ricardo/anthrogo/pkg/provider/fake"
	"github.com/ricardo/anthrogo/pkg/query"
)

func TestUsageBuiltin_NoEngine(t *testing.T) {
	h := newFakeHost() // engine is nil
	res, err := (Usage{}).Run(context.Background(), "", h)
	require.NoError(t, err)
	require.Equal(t, "no active engine", res.Text)
}

func TestUsageBuiltin_AutoCompactDisabled(t *testing.T) {
	fp := fake.New(nil)
	e := query.NewEngine(query.Config{
		Provider: fp,
		Model:    "m",
		// AutoCompactThreshold intentionally 0 (disabled)
	})
	h := newFakeHost()
	h.engine = e

	res, err := (Usage{}).Run(context.Background(), "", h)
	require.NoError(t, err)
	require.Contains(t, res.Text, "Auto-compact: disabled")
	require.Contains(t, res.Text, "Session totals:")
	require.Contains(t, res.Text, "Since last compact:")
}

func TestUsageBuiltin_AutoCompactEnabled(t *testing.T) {
	// Wire a turn that emits 60 input + 50 output tokens.
	fp := fake.New(
		[]provider.Event{
			{Kind: provider.EventStart, Usage: message.Usage{InputTokens: 60}},
			{Kind: provider.EventTextDelta, Text: "hi"},
			{Kind: provider.EventUsage, Usage: message.Usage{OutputTokens: 50}},
			{Kind: provider.EventMessageStop, StopReason: "end_turn"},
		},
	)
	const threshold = 500
	e := query.NewEngine(query.Config{
		Provider:             fp,
		Model:                "m",
		AutoCompactThreshold: threshold,
		AutoCompactKeepRecent: 10,
	})

	// Run one turn to accumulate usage.
	for range e.SubmitMessage(context.Background(), "hello") {
	}

	h := newFakeHost()
	h.engine = e

	res, err := (Usage{}).Run(context.Background(), "", h)
	require.NoError(t, err)
	require.Contains(t, res.Text, "Auto-compact at:")
	require.Contains(t, res.Text, "500 tokens")

	// 60+50 = 110 tokens since start; remaining should be 390.
	require.True(t, strings.Contains(res.Text, "390 tokens until trigger"),
		"expected 390 tokens until trigger, got: %s", res.Text)
}

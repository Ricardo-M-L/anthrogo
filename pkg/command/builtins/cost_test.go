package builtins

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/pkg/message"
	"github.com/ricardo/anthrogo/pkg/pricing"
	"github.com/ricardo/anthrogo/pkg/provider"
	"github.com/ricardo/anthrogo/pkg/provider/fake"
	"github.com/ricardo/anthrogo/pkg/query"
)

func TestCostBuiltin_NoEngine(t *testing.T) {
	h := newFakeHost()
	res, err := (Cost{}).Run(context.Background(), "", h)
	require.NoError(t, err)
	require.Equal(t, "no active engine", res.Text)
}

func TestCostBuiltin_NoPricingMatch(t *testing.T) {
	fp := fake.New(nil)
	// Pricing table has no entry for model "m"
	tbl := pricing.NewTable(map[string]pricing.Rate{
		"claude-sonnet-4-6": {InputPerM: 3.0, OutputPerM: 15.0},
	})
	e := query.NewEngine(query.Config{
		Provider: fp,
		Model:    "m",
		Pricing:  tbl,
	})
	h := newFakeHost()
	h.engine = e

	res, err := (Cost{}).Run(context.Background(), "", h)
	require.NoError(t, err)
	require.Contains(t, res.Text, "no pricing rate found for model")
}

func TestCost_Reset(t *testing.T) {
	// Build an engine with accumulated usage via a real turn.
	fp := fake.New(
		[]provider.Event{
			{Kind: provider.EventStart, Usage: message.Usage{InputTokens: 500}},
			{Kind: provider.EventTextDelta, Text: "ok"},
			{Kind: provider.EventUsage, Usage: message.Usage{OutputTokens: 200}},
			{Kind: provider.EventMessageStop, StopReason: "end_turn"},
		},
	)
	tbl := pricing.NewTable(map[string]pricing.Rate{
		"test-model": {InputPerM: 3.0, OutputPerM: 15.0},
	})
	e := query.NewEngine(query.Config{
		Provider: fp,
		Model:    "test-model",
		Pricing:  tbl,
	})
	for range e.SubmitMessage(context.Background(), "hi") {
	}
	// Verify usage is non-zero before reset.
	u := e.Usage()
	require.True(t, u.InputTokens > 0 || u.OutputTokens > 0, "expected non-zero usage before reset")

	h := newFakeHost()
	h.engine = e
	res, err := Cost{}.Run(context.Background(), "reset", h)
	require.NoError(t, err)
	require.Contains(t, res.Text, "reset")
	require.Contains(t, res.Text, "$0.0000")

	// Usage should now be zero.
	u2 := e.Usage()
	require.Equal(t, 0, u2.InputTokens)
	require.Equal(t, 0, u2.OutputTokens)
}

func TestCost_Reset_NoEngine(t *testing.T) {
	h := newFakeHost()
	res, err := Cost{}.Run(context.Background(), "reset", h)
	require.NoError(t, err)
	require.Equal(t, "no active engine", res.Text)
}

func TestCostBuiltin_PricingMatch(t *testing.T) {
	// Wire a turn: 1000 input + 500 output tokens
	fp := fake.New(
		[]provider.Event{
			{Kind: provider.EventStart, Usage: message.Usage{InputTokens: 1000}},
			{Kind: provider.EventTextDelta, Text: "hello"},
			{Kind: provider.EventUsage, Usage: message.Usage{OutputTokens: 500}},
			{Kind: provider.EventMessageStop, StopReason: "end_turn"},
		},
	)
	tbl := pricing.NewTable(map[string]pricing.Rate{
		"test-model": {InputPerM: 3.0, OutputPerM: 15.0},
	})
	e := query.NewEngine(query.Config{
		Provider: fp,
		Model:    "test-model",
		Pricing:  tbl,
	})

	// Run a turn to accumulate usage.
	for range e.SubmitMessage(context.Background(), "hi") {
	}

	h := newFakeHost()
	h.engine = e

	res, err := (Cost{}).Run(context.Background(), "", h)
	require.NoError(t, err)
	require.True(t, strings.Contains(res.Text, "Session usage:"), "want 'Session usage:' in %q", res.Text)
	require.True(t, strings.Contains(res.Text, "Estimated cost:"), "want 'Estimated cost:' in %q", res.Text)
	// 1000 input * 3/1M + 500 output * 15/1M = 0.003 + 0.0075 = 0.0105
	require.True(t, strings.Contains(res.Text, "$0.0105"), "want $0.0105 in %q", res.Text)
}

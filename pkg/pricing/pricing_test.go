package pricing_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/pkg/pricing"
)

func TestTable_ExactMatch(t *testing.T) {
	t.Parallel()
	tbl := pricing.NewTable(map[string]pricing.Rate{
		"claude-sonnet-4-6":  {InputPerM: 3.0, OutputPerM: 15.0},
		"claude-haiku-4-5-*": {InputPerM: 1.0, OutputPerM: 5.0},
	})
	r, ok := tbl.Lookup("claude-sonnet-4-6")
	require.True(t, ok)
	require.InDelta(t, 3.0, r.InputPerM, 1e-9)
	require.InDelta(t, 15.0, r.OutputPerM, 1e-9)
}

func TestTable_GlobMatch(t *testing.T) {
	t.Parallel()
	tbl := pricing.NewTable(map[string]pricing.Rate{
		"claude-haiku-4-5-*": {InputPerM: 1.0, OutputPerM: 5.0},
	})
	r, ok := tbl.Lookup("claude-haiku-4-5-20250130")
	require.True(t, ok)
	require.InDelta(t, 1.0, r.InputPerM, 1e-9)
	require.InDelta(t, 5.0, r.OutputPerM, 1e-9)
}

func TestTable_NoMatch(t *testing.T) {
	t.Parallel()
	tbl := pricing.NewTable(map[string]pricing.Rate{
		"claude-sonnet-4-6": {InputPerM: 3.0, OutputPerM: 15.0},
	})
	_, ok := tbl.Lookup("deepseek-chat")
	require.False(t, ok)
}

func TestEstimateUSD_LinearScaling(t *testing.T) {
	t.Parallel()
	rate := pricing.Rate{InputPerM: 3.0, OutputPerM: 15.0}

	// 1M input + 0 output = $3.00
	cost := pricing.EstimateUSD(rate, 1_000_000, 0)
	require.InDelta(t, 3.0, cost, 1e-9)

	// 0 input + 1M output = $15.00
	cost = pricing.EstimateUSD(rate, 0, 1_000_000)
	require.InDelta(t, 15.0, cost, 1e-9)

	// 500K input + 200K output = 1.5 + 3.0 = $4.50
	cost = pricing.EstimateUSD(rate, 500_000, 200_000)
	require.InDelta(t, 4.50, cost, 1e-9)

	// Linear: doubling tokens doubles cost
	cost1 := pricing.EstimateUSD(rate, 100, 100)
	cost2 := pricing.EstimateUSD(rate, 200, 200)
	require.InDelta(t, cost1*2, cost2, 1e-12)
}

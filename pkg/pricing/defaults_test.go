package pricing

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultRates_HasMajorModels(t *testing.T) {
	rates := DefaultRates()
	for _, model := range []string{
		"claude-sonnet-4-6",
		"claude-haiku-4-5",
		"claude-opus-4-7",
		"gpt-4o",
		"deepseek-chat",
		"kimi-k2",
		"MiniMax-M2",
		"glm-4.6",
	} {
		tbl := NewTable(rates)
		r, ok := tbl.Lookup(model)
		require.True(t, ok, "expected default rate for %s", model)
		require.Greater(t, r.InputPerM, 0.0)
		require.Greater(t, r.OutputPerM, 0.0)
	}
}

func TestMergeWithUserRates_UserOverridesBuiltin(t *testing.T) {
	user := map[string]Rate{
		"claude-sonnet-4-6": {InputPerM: 99.0, OutputPerM: 999.0},
	}
	merged := MergeWithUserRates(user)
	tbl := NewTable(merged)
	r, ok := tbl.Lookup("claude-sonnet-4-6")
	require.True(t, ok)
	require.Equal(t, 99.0, r.InputPerM)
	require.Equal(t, 999.0, r.OutputPerM)
}

func TestMergeWithUserRates_PreservesBuiltinsWhenUserMissing(t *testing.T) {
	user := map[string]Rate{
		"my-private-model": {InputPerM: 1.0, OutputPerM: 2.0},
	}
	merged := MergeWithUserRates(user)
	tbl := NewTable(merged)
	r, ok := tbl.Lookup("claude-sonnet-4-6")
	require.True(t, ok)
	require.Greater(t, r.InputPerM, 0.0)
	_, ok = tbl.Lookup("my-private-model")
	require.True(t, ok)
}

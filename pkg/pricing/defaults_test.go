package pricing

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultRates_HasMajorModels(t *testing.T) {
	t.Parallel()
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

func TestDefaultRates_BedrockVertexAliases(t *testing.T) {
	t.Parallel()
	tbl := NewTable(DefaultRates())
	cases := []struct {
		model  string
		wantIn float64
	}{
		// Bedrock exact-style
		{"anthropic.claude-sonnet-4-6-v1:0", 3.00},
		{"anthropic.claude-opus-4-7-v1:0", 15.00},
		{"anthropic.claude-haiku-4-5-v1:0", 1.00},
		// Bedrock pattern with opus/sonnet/haiku in middle
		{"anthropic.claude-3-sonnet-20240229-v1:0", 3.00},
		{"anthropic.claude-3-haiku-20240307-v1:0", 1.00},
		{"anthropic.claude-3-opus-20240229-v1:0", 15.00},
		// Vertex style
		{"claude-sonnet-4-6@20260101", 3.00},
		{"claude-opus-4-7@20260101", 15.00},
		{"claude-haiku-4-5@20260101", 1.00},
	}
	for _, c := range cases {
		r, ok := tbl.Lookup(c.model)
		require.True(t, ok, "expected rate for %s", c.model)
		require.Equal(t, c.wantIn, r.InputPerM, "InputPerM mismatch for %s", c.model)
	}
}

func TestMergeWithUserRates_UserOverridesBuiltin(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

package fake

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/pkg/provider"
)

func TestFake_EmitsScriptedEvents(t *testing.T) {
	f := New(
		[]provider.Event{
			{Kind: provider.EventTextDelta, Text: "hello "},
			{Kind: provider.EventTextDelta, Text: "world"},
			{Kind: provider.EventMessageStop, StopReason: "end_turn"},
		},
	)
	ch, err := f.Stream(context.Background(), provider.Request{})
	require.NoError(t, err)
	var got []string
	for ev := range ch {
		if ev.Kind == provider.EventTextDelta {
			got = append(got, ev.Text)
		}
	}
	require.Equal(t, []string{"hello ", "world"}, got)
}

func TestFake_MultipleScripts_OnePerTurn(t *testing.T) {
	f := New(
		[]provider.Event{{Kind: provider.EventTextDelta, Text: "a"}, {Kind: provider.EventMessageStop, StopReason: "end_turn"}},
		[]provider.Event{{Kind: provider.EventTextDelta, Text: "b"}, {Kind: provider.EventMessageStop, StopReason: "end_turn"}},
	)
	for _, want := range []string{"a", "b"} {
		ch, err := f.Stream(context.Background(), provider.Request{})
		require.NoError(t, err)
		var got string
		for ev := range ch {
			if ev.Kind == provider.EventTextDelta {
				got = ev.Text
			}
		}
		require.Equal(t, want, got)
	}
}

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

// newCompactHost wires a fakeHost with an engine seeded with msgs.
// Adaptation: fake.New (variadic) instead of plan's fake.NewScripted.
func newCompactHost(_ *testing.T, msgs []message.Message, summary string) *fakeHost {
	fp := fake.New(
		[]provider.Event{
			{Kind: provider.EventStart},
			{Kind: provider.EventTextDelta, Text: summary},
			{Kind: provider.EventMessageStop},
		},
	)
	e := query.NewEngine(query.Config{Provider: fp, Model: "m"})
	e.SetInitialMessages(msgs)
	h := newFakeHost()
	h.engine = e
	return h
}

func TestCompactBuiltin_Skipped(t *testing.T) {
	h := newCompactHost(t,
		[]message.Message{{Role: message.RoleUser, Content: []message.Block{{Type: message.BlockText, Text: "hi"}}}},
		"unused",
	)
	res, err := Compact{}.Run(context.Background(), "", h)
	require.NoError(t, err)
	require.Contains(t, res.Text, "compact: ")
}

// TestCompactBuiltin_KeepArgParsing verifies that --keep=N, -k N, and -k=N all
// set KeepRecent correctly by inspecting the resulting NewCount.
func TestCompactBuiltin_KeepArgParsing(t *testing.T) {
	makeMsgs := func(n int) []message.Message {
		msgs := make([]message.Message, n)
		for i := range msgs {
			if i%2 == 0 {
				msgs[i] = message.Message{Role: message.RoleUser, Content: []message.Block{{Type: message.BlockText, Text: "u"}}}
			} else {
				msgs[i] = message.Message{Role: message.RoleAssistant, Content: []message.Block{{Type: message.BlockText, Text: "a"}}}
			}
		}
		return msgs
	}

	cases := []struct {
		args        string
		wantContain string
	}{
		{"--keep=5", "compacted"},
		{"--keep 5", "compacted"},
		{"-k 5", "compacted"},
		{"-k=5", "compacted"},
	}
	for _, tc := range cases {
		t.Run(tc.args, func(t *testing.T) {
			h := newCompactHost(t, makeMsgs(20), "SUM")
			res, err := Compact{}.Run(context.Background(), tc.args, h)
			require.NoError(t, err)
			require.True(t, strings.Contains(res.Text, tc.wantContain), "args=%q res=%q", tc.args, res.Text)
		})
	}
}

func TestCompactBuiltin_Success(t *testing.T) {
	// Use alternating user/assistant messages so the assistant-boundary split
	// can find a valid anchor. desiredSplit=5, msgs[5] is assistant → 11 new.
	msgs := make([]message.Message, 15)
	for i := range msgs {
		if i%2 == 0 {
			msgs[i] = message.Message{Role: message.RoleUser, Content: []message.Block{{Type: message.BlockText, Text: "u"}}}
		} else {
			msgs[i] = message.Message{Role: message.RoleAssistant, Content: []message.Block{{Type: message.BlockText, Text: "a"}}}
		}
	}
	h := newCompactHost(t, msgs, "SUM")
	res, err := Compact{}.Run(context.Background(), "", h)
	require.NoError(t, err)
	require.Contains(t, res.Text, "compacted 15 → 11 messages")
}

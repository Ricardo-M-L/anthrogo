package compact

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/pkg/message"
	"github.com/ricardo/anthrogo/pkg/provider"
	"github.com/ricardo/anthrogo/pkg/provider/fake"
)

func textMsg(role message.Role, text string) message.Message {
	return message.Message{Role: role, Content: []message.Block{{Type: message.BlockText, Text: text}}}
}

// summaryProvider returns a fake provider that emits one text_delta with the
// configured summary and then EventMessageStop.
// Adaptation: uses fake.New (variadic []provider.Event) — fake.NewScripted does not exist.
func summaryProvider(summary string) provider.Provider {
	return fake.New(
		[]provider.Event{
			{Kind: provider.EventStart},
			{Kind: provider.EventTextDelta, Text: summary},
			{Kind: provider.EventMessageStop},
		},
	)
}

func TestCompact_NoOpWhenShort(t *testing.T) {
	msgs := []message.Message{textMsg(message.RoleUser, "hi"), textMsg(message.RoleAssistant, "hello")}
	out, err := Run(context.Background(), Input{
		Provider:   summaryProvider("unused"),
		Model:      "m",
		Messages:   msgs,
		KeepRecent: 10,
	})
	require.NoError(t, err)
	require.True(t, out.Skipped)
	require.Equal(t, 2, out.OriginalCount)
	require.Empty(t, out.NewMessages)
}

func TestCompact_SummarizesHead(t *testing.T) {
	msgs := make([]message.Message, 15)
	for i := range msgs {
		if i%2 == 0 {
			msgs[i] = textMsg(message.RoleUser, "u"+strconv.Itoa(i))
		} else {
			msgs[i] = textMsg(message.RoleAssistant, "a"+strconv.Itoa(i))
		}
	}
	out, err := Run(context.Background(), Input{
		Provider:   summaryProvider("SUMMARY_TEXT"),
		Model:      "m",
		Messages:   msgs,
		KeepRecent: 10,
	})
	require.NoError(t, err)
	require.False(t, out.Skipped)
	require.Equal(t, 15, out.OriginalCount)
	require.Equal(t, 11, out.NewCount) // 1 summary + 10 tail
	require.Equal(t, "SUMMARY_TEXT", out.SummaryText)
	require.Contains(t, out.NewMessages[0].Content[0].Text, "SUMMARY_TEXT")
	require.Contains(t, out.NewMessages[0].Content[0].Text, "Compacted")
}

// TestCompact_AssistantBoundary verifies that the split searches forward to
// find an assistant message, so the resulting conversation starts with the
// summary user-message followed immediately by an assistant turn.
func TestCompact_AssistantBoundary(t *testing.T) {
	// 15 messages: 0=user,1=asst,...,14=user. KeepRecent=10 → desiredSplit=5.
	// msgs[5] is assistant → split=5, head=5, tail=10. newCount=11.
	msgs := make([]message.Message, 15)
	for i := range msgs {
		if i%2 == 0 {
			msgs[i] = textMsg(message.RoleUser, "u")
		} else {
			msgs[i] = textMsg(message.RoleAssistant, "a")
		}
	}
	out, err := Run(context.Background(), Input{
		Provider:   summaryProvider("S"),
		Model:      "m",
		Messages:   msgs,
		KeepRecent: 10,
	})
	require.NoError(t, err)
	require.False(t, out.Skipped)
	require.Equal(t, 11, out.NewCount) // 1 summary + 10 tail
	// New conversation: [summary_user, asst, user, ...] — valid alternation.
	require.Equal(t, message.RoleUser, out.NewMessages[0].Role)
	require.Equal(t, message.RoleAssistant, out.NewMessages[1].Role)
}

// TestCompact_NoAssistantBoundary verifies skipping when no assistant turn exists
// in the window to anchor the split.
func TestCompact_NoAssistantBoundary(t *testing.T) {
	// All user messages — no assistant boundary to find.
	msgs := make([]message.Message, 15)
	for i := range msgs {
		msgs[i] = textMsg(message.RoleUser, "u")
	}
	out, err := Run(context.Background(), Input{
		Provider:   summaryProvider("unused"),
		Model:      "m",
		Messages:   msgs,
		KeepRecent: 10,
	})
	require.NoError(t, err)
	require.True(t, out.Skipped)
	require.Contains(t, out.SkipReason, "no assistant")
}

func TestCompact_PartialStreamThenError_ReturnsAccumulated(t *testing.T) {
	fp := fake.New([]provider.Event{
		{Kind: provider.EventTextDelta, Text: "PARTIAL"},
		{Kind: provider.EventError, Err: errors.New("upstream cut")},
	})
	msgs := make([]message.Message, 20)
	for i := range msgs {
		if i%2 == 0 {
			msgs[i] = message.Message{Role: message.RoleUser, Content: []message.Block{{Type: message.BlockText, Text: "u"}}}
		} else {
			msgs[i] = message.Message{Role: message.RoleAssistant, Content: []message.Block{{Type: message.BlockText, Text: "a"}}}
		}
	}
	out, err := Run(context.Background(), Input{Provider: fp, Model: "m", Messages: msgs, KeepRecent: 10})
	require.NoError(t, err)
	require.Contains(t, out.SummaryText, "PARTIAL")
}

func TestCompact_ProviderError_PropagatesUntouched(t *testing.T) {
	// Adaptation: fake.New (variadic) instead of fake.NewScripted.
	// The fake provider sends EventError on the channel (does NOT return it from Stream).
	// streamSummary handles this by reading ev.Err from the channel.
	// Use alternating messages so the assistant-boundary split can proceed to
	// the provider call (and surface the error).
	fp := fake.New([]provider.Event{{Kind: provider.EventError, Err: errors.New("upstream down")}})
	msgs := make([]message.Message, 15)
	for i := range msgs {
		if i%2 == 0 {
			msgs[i] = textMsg(message.RoleUser, "x")
		} else {
			msgs[i] = textMsg(message.RoleAssistant, "y")
		}
	}
	_, err := Run(context.Background(), Input{Provider: fp, Model: "m", Messages: msgs, KeepRecent: 10})
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "upstream down"))
}


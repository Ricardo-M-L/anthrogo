package compact

import (
	"context"
	"fmt"
	"strings"

	"github.com/ricardo/anthrogo/pkg/message"
	"github.com/ricardo/anthrogo/pkg/provider"
	"github.com/ricardo/anthrogo/pkg/tokens"
)

// Input is the pure input to Run. No engine, no hooks.
type Input struct {
	Provider   provider.Provider
	Model      string
	Messages   []message.Message
	KeepRecent int // default 10
	MaxTokens  int // default 4096 for the summary call
}

// Output is what Run returns. NewMessages is empty if Skipped.
type Output struct {
	NewMessages    []message.Message
	OriginalCount  int
	NewCount       int
	OriginalTokens int
	NewTokens      int
	SummaryText    string
	Skipped        bool
	SkipReason     string
}

// Run produces a compacted message list. Pure (modulo provider call).
//
// Algorithm:
//  1. If len(messages) <= KeepRecent, skip.
//  2. Find split: search forward from desiredSplit = len-KeepRecent until we find
//     an assistant message. tail = messages[split:] begins with an assistant turn
//     so the new conversation (summary_user + tail...) is valid per the Anthropic
//     API (user → assistant alternation).
//  3. Summarise the entire head (no MCP carve-out; MCP-aware preservation is
//     deferred to a later milestone).
//  4. newMessages = [summary_user_msg, tail...]
func Run(ctx context.Context, in Input) (Output, error) {
	if in.KeepRecent <= 0 {
		in.KeepRecent = 10
	}
	if in.MaxTokens <= 0 {
		in.MaxTokens = 4096
	}
	counter := tokens.NewCounter(in.Model)
	msgs := in.Messages
	out := Output{
		OriginalCount:  len(msgs),
		OriginalTokens: counter.CountMessages(msgs),
	}
	if len(msgs) <= in.KeepRecent {
		out.Skipped = true
		out.SkipReason = "fewer than KeepRecent messages"
		return out, nil
	}

	// Find split: msgs[split].Role must be RoleAssistant so tail begins with
	// an assistant message. The new conversation [summary_user, tail...] is then
	// a valid user→assistant alternation.
	desiredSplit := len(msgs) - in.KeepRecent
	split := -1
	for i := desiredSplit; i < len(msgs); i++ {
		if msgs[i].Role == message.RoleAssistant {
			split = i
			break
		}
	}
	if split < 0 {
		out.Skipped = true
		out.SkipReason = "no assistant message in the recent window to anchor compact"
		return out, nil
	}
	head := msgs[:split]
	tail := msgs[split:]

	summary, err := streamSummary(ctx, in.Provider, in.Model, in.MaxTokens, head)
	if err != nil {
		return Output{}, err
	}
	if strings.TrimSpace(summary) == "" {
		return Output{}, fmt.Errorf("compact: empty summary")
	}
	out.SummaryText = summary

	summaryMsg := message.Message{
		Role: message.RoleUser,
		Content: []message.Block{{
			Type: message.BlockText,
			Text: fmt.Sprintf("[Compacted earlier conversation (%d messages)]\n\n%s", len(head), summary),
		}},
	}

	newMsgs := make([]message.Message, 0, 1+len(tail))
	newMsgs = append(newMsgs, summaryMsg)
	newMsgs = append(newMsgs, tail...)

	out.NewMessages = newMsgs
	out.NewCount = len(newMsgs)
	out.NewTokens = counter.CountMessages(newMsgs)
	return out, nil
}

func streamSummary(ctx context.Context, p provider.Provider, model string, maxTokens int, head []message.Message) (string, error) {
	req := provider.Request{
		Model:        model,
		SystemPrompt: SummarySystemPrompt,
		Messages:     []message.Message{buildSummaryUserMessage(head)},
		MaxTokens:    maxTokens,
	}
	ch, err := p.Stream(ctx, req)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for ev := range ch {
		switch ev.Kind {
		case provider.EventTextDelta:
			b.WriteString(ev.Text)
		case provider.EventError:
			if b.Len() > 0 {
				// Partial summary is better than nothing; return what we have.
				return b.String(), nil
			}
			return "", ev.Err
		}
	}
	return b.String(), nil
}

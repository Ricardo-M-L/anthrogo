package anthropic

import (
	"context"
	"encoding/json"

	sdk "github.com/anthropics/anthropic-sdk-go"

	"github.com/ricardo/anthrogo/pkg/message"
	"github.com/ricardo/anthrogo/pkg/provider"
)

// Stream implements provider.Provider against anthropic-sdk-go v1.x.
//
// The v1 SDK dropped the v0.2-alpha `sdk.F[T]()` opt wrappers and reorganised
// the stream-event API around `MessageStreamEventUnion.AsAny()`. This file
// is the full rewrite — same external behaviour, new SDK shape.
func (p *Provider) Stream(ctx context.Context, req provider.Request) (<-chan provider.Event, error) {
	params, err := buildParams(p.model, req)
	if err != nil {
		return nil, err
	}

	stream := p.client.Messages.NewStreaming(ctx, params)

	out := make(chan provider.Event, 64)
	go func() {
		defer close(out)
		for stream.Next() {
			ev := stream.Current()
			translateEvent(ev, out)
		}
		if err := stream.Err(); err != nil {
			out <- provider.Event{Kind: provider.EventError, Err: err}
		}
	}()
	return out, nil
}

// buildParams converts a provider.Request into SDK MessageNewParams.
func buildParams(defaultModel string, req provider.Request) (sdk.MessageNewParams, error) {
	model := req.Model
	if model == "" {
		model = defaultModel
	}

	msgs, err := convertMessages(req.Messages)
	if err != nil {
		return sdk.MessageNewParams{}, err
	}

	params := sdk.MessageNewParams{
		Model:     sdk.Model(model),
		MaxTokens: int64(req.MaxTokens),
		Messages:  msgs,
	}

	if req.SystemPrompt != "" {
		// Prompt cache: mark the system block as ephemeral so Anthropic
		// reuses it on subsequent turns. ~90% input-token discount on
		// cached hits. Safe to always set — API ignores it on non-
		// cacheable content (< 1024 tokens).
		params.System = []sdk.TextBlockParam{
			{
				Text:         req.SystemPrompt,
				CacheControl: sdk.CacheControlEphemeralParam{},
			},
		}
	}

	if req.Temperature != 0 {
		params.Temperature = sdk.Float(req.Temperature)
	}

	if len(req.Tools) > 0 {
		tools := make([]sdk.ToolUnionParam, 0, len(req.Tools))
		for i, ts := range req.Tools {
			tp := sdk.ToolParam{
				Name:        ts.Name,
				Description: sdk.String(ts.Description),
				InputSchema: sdk.ToolInputSchemaParam{
					Properties: ts.InputSchema["properties"],
				},
			}
			// Pass through 'required' if the source schema specified one.
			if r, ok := ts.InputSchema["required"].([]string); ok {
				tp.InputSchema.Required = r
			} else if r, ok := ts.InputSchema["required"].([]any); ok {
				required := make([]string, 0, len(r))
				for _, v := range r {
					if s, ok := v.(string); ok {
						required = append(required, s)
					}
				}
				tp.InputSchema.Required = required
			}
			// Cache breakpoint on the LAST tool description — Anthropic
			// caches everything up to the most recent cache_control mark,
			// so one breakpoint covers system + tools together.
			if i == len(req.Tools)-1 {
				tp.CacheControl = sdk.CacheControlEphemeralParam{}
			}
			tools = append(tools, sdk.ToolUnionParam{OfTool: &tp})
		}
		params.Tools = tools
	}

	return params, nil
}

// convertMessages converts []message.Message to []sdk.MessageParam, skipping RoleSystem.
func convertMessages(msgs []message.Message) ([]sdk.MessageParam, error) {
	out := make([]sdk.MessageParam, 0, len(msgs))
	for _, m := range msgs {
		if m.Role == message.RoleSystem {
			continue
		}
		blocks, err := convertBlocks(m.Content)
		if err != nil {
			return nil, err
		}
		switch m.Role {
		case message.RoleUser:
			out = append(out, sdk.NewUserMessage(blocks...))
		case message.RoleAssistant:
			out = append(out, sdk.NewAssistantMessage(blocks...))
		}
	}
	return out, nil
}

// convertBlocks converts []message.Block to []sdk.ContentBlockParamUnion.
func convertBlocks(blocks []message.Block) ([]sdk.ContentBlockParamUnion, error) {
	out := make([]sdk.ContentBlockParamUnion, 0, len(blocks))
	for _, b := range blocks {
		switch b.Type {
		case message.BlockText:
			out = append(out, sdk.NewTextBlock(b.Text))
		case message.BlockToolUse:
			raw, err := json.Marshal(b.Input)
			if err != nil {
				return nil, err
			}
			var inputVal any
			if err := json.Unmarshal(raw, &inputVal); err != nil {
				return nil, err
			}
			out = append(out, sdk.NewToolUseBlock(b.ToolUseID, inputVal, b.ToolName))
		case message.BlockToolResult:
			out = append(out, sdk.NewToolResultBlock(b.ToolUseID, b.Text, b.IsError))
		case message.BlockThinking:
			// Thinking blocks in input are output-only; skip.
		case message.BlockImage:
			if b.ImageSource != nil {
				out = append(out, sdk.NewImageBlockBase64(b.ImageSource.MediaType, b.ImageSource.Data))
			}
		}
	}
	return out, nil
}

// translateEvent converts one SDK stream event union to provider.Event(s).
//
// v1 SDK exposes events through MessageStreamEventUnion.AsAny() rather than
// the v0.2-alpha (raw sdk.MessageStreamEvent).AsUnion() pattern.
func translateEvent(raw sdk.MessageStreamEventUnion, out chan<- provider.Event) {
	switch ev := raw.AsAny().(type) {

	case sdk.MessageStartEvent:
		u := ev.Message.Usage
		out <- provider.Event{
			Kind: provider.EventStart,
			Usage: message.Usage{
				InputTokens:              int(u.InputTokens),
				OutputTokens:             int(u.OutputTokens),
				CacheCreationInputTokens: int(u.CacheCreationInputTokens),
				CacheReadInputTokens:     int(u.CacheReadInputTokens),
			},
		}

	case sdk.ContentBlockStartEvent:
		cb := ev.ContentBlock
		if cb.Type == "tool_use" {
			out <- provider.Event{
				Kind:      provider.EventToolUseStart,
				ToolUseID: cb.ID,
				ToolName:  cb.Name,
			}
		}
		// text + thinking blocks: no start event needed.

	case sdk.ContentBlockDeltaEvent:
		delta := ev.Delta
		switch delta.Type {
		case "text_delta":
			out <- provider.Event{Kind: provider.EventTextDelta, Text: delta.Text}
		case "input_json_delta":
			out <- provider.Event{Kind: provider.EventToolInputDelta, PartialJSON: delta.PartialJSON}
		case "thinking_delta":
			out <- provider.Event{Kind: provider.EventThinkingDelta, Text: delta.Thinking}
		}

	case sdk.ContentBlockStopEvent:
		out <- provider.Event{Kind: provider.EventBlockStop}

	case sdk.MessageDeltaEvent:
		out <- provider.Event{
			Kind: provider.EventUsage,
			Usage: message.Usage{
				OutputTokens: int(ev.Usage.OutputTokens),
			},
		}
		if ev.Delta.StopReason != "" {
			out <- provider.Event{
				Kind:       provider.EventMessageStop,
				StopReason: string(ev.Delta.StopReason),
			}
		}

	case sdk.MessageStopEvent:
		// Intentionally NOT emitting EventMessageStop here — the SDK
		// already surfaced stop_reason through MessageDeltaEvent above.
		_ = ev
	}
}

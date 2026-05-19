package anthropic

import (
	"context"
	"encoding/json"

	sdk "github.com/anthropics/anthropic-sdk-go"

	"github.com/ricardo/anthrogo/pkg/message"
	"github.com/ricardo/anthrogo/pkg/provider"
)

// Stream implements provider.Provider.
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
		Model:     sdk.F(sdk.Model(model)),
		MaxTokens: sdk.F(int64(req.MaxTokens)),
		Messages:  sdk.F(msgs),
	}

	if req.SystemPrompt != "" {
		params.System = sdk.F([]sdk.TextBlockParam{
			sdk.NewTextBlock(req.SystemPrompt),
		})
	}

	if req.Temperature != 0 {
		params.Temperature = sdk.F(req.Temperature)
	}

	if len(req.Tools) > 0 {
		tools := make([]sdk.ToolUnionUnionParam, 0, len(req.Tools))
		for _, ts := range req.Tools {
			tools = append(tools, sdk.ToolParam{
				Name:        sdk.F(ts.Name),
				Description: sdk.F(ts.Description),
				InputSchema: sdk.F[interface{}](ts.InputSchema),
			})
		}
		params.Tools = sdk.F(tools)
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
			// Serialize Input map to raw JSON then unmarshal as interface{}
			raw, err := json.Marshal(b.Input)
			if err != nil {
				return nil, err
			}
			var inputVal interface{}
			if err := json.Unmarshal(raw, &inputVal); err != nil {
				return nil, err
			}
			out = append(out, sdk.NewToolUseBlockParam(b.ToolUseID, b.ToolName, inputVal))
		case message.BlockToolResult:
			out = append(out, sdk.NewToolResultBlock(b.ToolUseID, b.Text, b.IsError))
		case message.BlockThinking:
			// Thinking blocks in input: skip (they are output-only)
		case message.BlockImage:
			if b.ImageSource != nil {
				mediaType := sdk.Base64ImageSourceMediaType(b.ImageSource.MediaType)
				out = append(out, sdk.ImageBlockParam{
					Type: sdk.F(sdk.ImageBlockParamTypeImage),
					Source: sdk.F(sdk.ImageBlockParamSourceUnion(sdk.Base64ImageSourceParam{
						Type:      sdk.F(sdk.Base64ImageSourceTypeBase64),
						MediaType: sdk.F(mediaType),
						Data:      sdk.F(b.ImageSource.Data),
					})),
				})
			}
		}
	}
	return out, nil
}

// translateEvent converts one SDK stream event to provider.Event(s).
func translateEvent(raw sdk.MessageStreamEvent, out chan<- provider.Event) {
	union := raw.AsUnion()
	switch ev := union.(type) {
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
		switch cb.Type {
		case sdk.ContentBlockStartEventContentBlockTypeToolUse:
			out <- provider.Event{
				Kind:      provider.EventToolUseStart,
				ToolUseID: cb.ID,
				ToolName:  cb.Name,
			}
		// text / thinking blocks: no start event needed
		}

	case sdk.ContentBlockDeltaEvent:
		delta := ev.Delta
		switch delta.Type {
		case sdk.ContentBlockDeltaEventDeltaTypeTextDelta:
			out <- provider.Event{Kind: provider.EventTextDelta, Text: delta.Text}
		case sdk.ContentBlockDeltaEventDeltaTypeInputJSONDelta:
			out <- provider.Event{Kind: provider.EventToolInputDelta, PartialJSON: delta.PartialJSON}
		case sdk.ContentBlockDeltaEventDeltaTypeThinkingDelta:
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
		// Intentionally NOT emitting EventMessageStop here — the SDK already
		// surfaces stop_reason through MessageDeltaEvent above. A second empty-
		// StopReason emit here would overwrite the real stopReason in the
		// engine loop and surface "" as the TurnComplete reason.
		_ = ev
	}
}


package openai

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ricardo/anthrogo/pkg/message"
	"github.com/ricardo/anthrogo/pkg/provider"
)

var dataPrefix = []byte("data: ")

// buildRequest converts a provider.Request into an OpenAI chatRequest.
func buildRequest(model string, req provider.Request) chatRequest {
	var msgs []chatMsg

	if req.SystemPrompt != "" {
		msgs = append(msgs, chatMsg{Role: "system", Content: req.SystemPrompt})
	}

	for _, m := range req.Messages {
		switch m.Role {
		case message.RoleSystem:
			// Already inserted above if top-level; skip per-message system blocks.
			continue
		case message.RoleUser:
			// Check if any block is tool_result; if so emit per-result tool messages.
			hasToolResult := false
			hasImage := false
			for _, b := range m.Content {
				if b.Type == message.BlockToolResult {
					hasToolResult = true
					break
				}
				if b.Type == message.BlockImage {
					hasImage = true
				}
			}
			if hasToolResult {
				for _, b := range m.Content {
					if b.Type == message.BlockToolResult {
						msgs = append(msgs, chatMsg{
							Role:       "tool",
							ToolCallID: b.ToolUseID,
							Content:    b.Text,
						})
					}
					// skip non-tool_result blocks in a tool-result turn
				}
			} else if hasImage {
				// Multimodal user turn: build []chatContent array.
				var parts []chatContent
				for _, b := range m.Content {
					switch b.Type {
					case message.BlockText:
						parts = append(parts, chatContent{Type: "text", Text: b.Text})
					case message.BlockImage:
						if b.ImageSource != nil {
							url := "data:" + b.ImageSource.MediaType + ";base64," + b.ImageSource.Data
							parts = append(parts, chatContent{
								Type:     "image_url",
								ImageURL: &chatImageURL{URL: url},
							})
						}
					case message.BlockThinking:
						// silently drop
					}
				}
				msgs = append(msgs, chatMsg{Role: "user", Content: parts})
			} else {
				// Plain user turn: concatenate text blocks.
				var sb strings.Builder
				for _, b := range m.Content {
					switch b.Type {
					case message.BlockText:
						sb.WriteString(b.Text)
					case message.BlockThinking:
						// silently drop
					}
				}
				msgs = append(msgs, chatMsg{Role: "user", Content: sb.String()})
			}
		case message.RoleAssistant:
			var text strings.Builder
			var toolCalls []chatToolCall
			for _, b := range m.Content {
				switch b.Type {
				case message.BlockText:
					text.WriteString(b.Text)
				case message.BlockToolUse:
					raw, _ := json.Marshal(b.Input)
					toolCalls = append(toolCalls, chatToolCall{
						ID:   b.ToolUseID,
						Type: "function",
						Function: chatFunc{
							Name:      b.ToolName,
							Arguments: string(raw),
						},
					})
				case message.BlockThinking, message.BlockImage:
					// silently drop
				}
			}
			msg := chatMsg{Role: "assistant", Content: text.String()}
			if len(toolCalls) > 0 {
				msg.ToolCalls = toolCalls
			}
			msgs = append(msgs, msg)
		}
	}

	tools := make([]chatTool, 0, len(req.Tools))
	for _, ts := range req.Tools {
		tools = append(tools, chatTool{
			Type: "function",
			Function: chatToolFuncSpec{
				Name:        ts.Name,
				Description: ts.Description,
				Parameters:  ts.InputSchema,
			},
		})
	}

	r := chatRequest{
		Model:         model,
		Messages:      msgs,
		Stream:        true,
		StreamOptions: &streamOpts{IncludeUsage: true},
	}
	if len(tools) > 0 {
		r.Tools = tools
	}
	if req.MaxTokens > 0 {
		r.MaxTokens = req.MaxTokens
	}
	if req.Temperature != 0 {
		r.Temperature = req.Temperature
	}
	return r
}

// mapFinishReason converts OpenAI finish_reason to anthrogo stop reason.
func mapFinishReason(reason string) string {
	switch reason {
	case "stop":
		return "end_turn"
	case "tool_calls":
		return "tool_use"
	case "length":
		return "max_tokens"
	default:
		return reason
	}
}

// parseSSE reads one SSE response and emits events to out.
// It closes out when done.
func parseSSE(resp *http.Response, out chan<- provider.Event) {
	defer close(out)
	defer resp.Body.Close()

	// Track which tool-call indexes we've already emitted EventToolUseStart for.
	seenToolCallIdx := map[int]bool{}
	// Buffer partial tool call arguments per index (not needed for streaming event translation,
	// but we track seen status per index).

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Bytes()
		if !bytes.HasPrefix(line, dataPrefix) {
			continue
		}
		payload := bytes.TrimPrefix(line, dataPrefix)
		payload = bytes.TrimSpace(payload)
		if string(payload) == "[DONE]" {
			return
		}

		var chunk chatChunk
		if err := json.Unmarshal(payload, &chunk); err != nil {
			// skip malformed chunks
			continue
		}

		if len(chunk.Choices) == 0 {
			// May be a usage-only chunk.
			if chunk.Usage != nil {
				out <- provider.Event{
					Kind: provider.EventUsage,
					Usage: message.Usage{
						InputTokens:  chunk.Usage.PromptTokens,
						OutputTokens: chunk.Usage.CompletionTokens,
					},
				}
			}
			continue
		}

		choice := chunk.Choices[0]
		delta := choice.Delta

		if delta.Content != "" {
			out <- provider.Event{Kind: provider.EventTextDelta, Text: delta.Content}
		}

		for _, tc := range delta.ToolCalls {
			if !seenToolCallIdx[tc.Index] && tc.ID != "" {
				seenToolCallIdx[tc.Index] = true
				out <- provider.Event{
					Kind:      provider.EventToolUseStart,
					ToolUseID: tc.ID,
					ToolName:  tc.Function.Name,
				}
			}
			if tc.Function.Arguments != "" {
				out <- provider.Event{
					Kind:        provider.EventToolInputDelta,
					PartialJSON: tc.Function.Arguments,
				}
			}
		}

		if choice.FinishReason != "" {
			out <- provider.Event{Kind: provider.EventBlockStop}
			out <- provider.Event{
				Kind:       provider.EventMessageStop,
				StopReason: mapFinishReason(choice.FinishReason),
			}
		}

		if chunk.Usage != nil {
			out <- provider.Event{
				Kind: provider.EventUsage,
				Usage: message.Usage{
					InputTokens:  chunk.Usage.PromptTokens,
					OutputTokens: chunk.Usage.CompletionTokens,
				},
			}
		}
	}

	if err := scanner.Err(); err != nil {
		out <- provider.Event{Kind: provider.EventError, Err: err}
	}
}

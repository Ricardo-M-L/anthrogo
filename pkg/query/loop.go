package query

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/ricardo/anthrogo/internal/session"
	"github.com/ricardo/anthrogo/pkg/message"
	"github.com/ricardo/anthrogo/pkg/permissions"
	"github.com/ricardo/anthrogo/pkg/provider"
	"github.com/ricardo/anthrogo/pkg/tool"
)

func (e *Engine) recordIfHooked(r session.Record) {
	if e.cfg.RecordHook != nil {
		e.cfg.RecordHook(r)
	}
}

// SubmitMessage runs one user turn — including any tool_use sub-turns —
// until the model emits end_turn (or an error/abort).
func (e *Engine) SubmitMessage(ctx context.Context, prompt string) <-chan Event {
	out := make(chan Event, 64)
	e.mu.Lock()
	e.messages = append(e.messages, message.Text(message.RoleUser, prompt))
	lastContent := e.messages[len(e.messages)-1].Content
	e.mu.Unlock()
	e.recordIfHooked(session.Record{
		Kind:        session.KindUserMessage,
		UserMessage: &session.UserMessage{Content: lastContent},
	})
	go func() {
		defer close(out)
		for turn := 0; e.cfg.MaxTurns == 0 || turn < e.cfg.MaxTurns; turn++ {
			stop, err := e.runOneAPITurn(ctx, out)
			if err != nil {
				e.recordIfHooked(session.Record{
					Kind:  session.KindError,
					Error: &session.ErrorRecord{Error: err.Error(), During: "stream"},
				})
				out <- Event{Kind: KindError, Err: err}
				return
			}
			if stop != "tool_use" {
				e.recordIfHooked(session.Record{
					Kind:         session.KindTurnComplete,
					TurnComplete: &session.TurnComplete{StopReason: stop},
				})
				out <- Event{Kind: KindTurnComplete, StopReason: stop}
				if e.cfg.Hooks != nil {
					stopReason := stop
					if stopReason == "" {
						stopReason = "end_turn"
					}
					e.cfg.Hooks.FireStop(ctx, stopReason)
				}
				return
			}
		}
		out <- Event{Kind: KindError, Err: fmt.Errorf("max_turns exceeded")}
	}()
	return out
}

func (e *Engine) runOneAPITurn(ctx context.Context, out chan<- Event) (stopReason string, err error) {
	e.mu.Lock()
	msgsCopy := append([]message.Message(nil), e.messages...)
	e.mu.Unlock()
	req := provider.Request{
		Model:        e.cfg.Model,
		Messages:     msgsCopy,
		SystemPrompt: e.cfg.SystemPrompt,
		MaxTokens:    e.cfg.MaxTokens,
		Temperature:  e.cfg.Temperature,
		Tools:        e.toolSpecs(),
	}
	ch, err := e.cfg.Provider.Stream(ctx, req)
	if err != nil {
		return "", err
	}

	// Accumulate assistant content blocks as we stream.
	var (
		assistant     []message.Block
		textBuf       bytes.Buffer
		pendingTool   *message.Block
		toolInputJSON bytes.Buffer
	)
	flushText := func() {
		if textBuf.Len() > 0 {
			assistant = append(assistant, message.Block{Type: message.BlockText, Text: textBuf.String()})
			textBuf.Reset()
		}
	}
	flushTool := func() {
		if pendingTool == nil {
			return
		}
		if toolInputJSON.Len() > 0 {
			_ = json.Unmarshal(toolInputJSON.Bytes(), &pendingTool.Input)
		}
		assistant = append(assistant, *pendingTool)
		pendingTool = nil
		toolInputJSON.Reset()
	}

	for ev := range ch {
		switch ev.Kind {
		case provider.EventTextDelta:
			textBuf.WriteString(ev.Text)
			out <- Event{Kind: KindAssistantDelta, Text: ev.Text}
		case provider.EventToolUseStart:
			flushText()
			pendingTool = &message.Block{
				Type:      message.BlockToolUse,
				ToolUseID: ev.ToolUseID,
				ToolName:  ev.ToolName,
				Input:     map[string]any{},
			}
		case provider.EventToolInputDelta:
			toolInputJSON.WriteString(ev.PartialJSON)
		case provider.EventBlockStop:
			flushTool()
		case provider.EventStart, provider.EventUsage:
			// Both carry token counts; EventStart has input tokens, EventUsage
			// has output tokens (and cache stats). Accumulate both.
			e.usage.Add(ev.Usage)
			out <- Event{Kind: KindUsage, Usage: ev.Usage}
			e.recordIfHooked(session.Record{
				Kind: session.KindUsage,
				Usage: &session.UsageRecord{
					InputTokens:              ev.Usage.InputTokens,
					OutputTokens:             ev.Usage.OutputTokens,
					CacheCreationInputTokens: ev.Usage.CacheCreationInputTokens,
					CacheReadInputTokens:     ev.Usage.CacheReadInputTokens,
				},
			})
		case provider.EventMessageStop:
			flushText()
			flushTool()
			if ev.StopReason != "" {
				stopReason = ev.StopReason
			}
		case provider.EventError:
			return "", ev.Err
		}
	}

	if len(assistant) > 0 {
		e.mu.Lock()
		e.messages = append(e.messages, message.Message{Role: message.RoleAssistant, Content: assistant})
		e.mu.Unlock()
		e.recordIfHooked(session.Record{
			Kind: session.KindAssistantMessage,
			AssistantMessage: &session.AssistantMessage{Content: assistant, StopReason: stopReason},
		})
	}
	out <- Event{Kind: KindAssistantStop, StopReason: stopReason}

	// If the model asked for tools, run them and append tool_result blocks.
	// When ALL tool_use blocks have IsConcurrencySafe=true, run them in parallel.
	// If even one says false, fall back to sequential execution.
	if stopReason == "tool_use" {
		var toolBlocks []message.Block
		for _, b := range assistant {
			if b.Type == message.BlockToolUse {
				toolBlocks = append(toolBlocks, b)
			}
		}

		allSafe := len(toolBlocks) > 0
		for _, b := range toolBlocks {
			t, ok := e.cfg.Tools.Get(b.ToolName)
			if !ok || !t.IsConcurrencySafe() {
				allSafe = false
				break
			}
		}

		results := make([]message.Block, len(toolBlocks))
		if allSafe && len(toolBlocks) > 1 {
			// Parallel dispatch: goroutines write into fixed result slots so
			// the tool_result order matches the tool_use order.
			var wg sync.WaitGroup
			// out channel is buffered (64); concurrent writes are safe.
			for i, b := range toolBlocks {
				wg.Add(1)
				go func(idx int, blk message.Block) {
					defer wg.Done()
					results[idx] = e.executeTool(ctx, blk, out)
				}(i, b)
			}
			wg.Wait()
		} else {
			for i, b := range toolBlocks {
				results[i] = e.executeTool(ctx, b, out)
			}
		}

		if len(results) > 0 {
			e.mu.Lock()
			e.messages = append(e.messages, message.Message{Role: message.RoleUser, Content: results})
			e.mu.Unlock()
		}
	}
	return stopReason, nil
}

func (e *Engine) executeTool(ctx context.Context, b message.Block, out chan<- Event) message.Block {
	t, ok := e.cfg.Tools.Get(b.ToolName)
	if !ok {
		msg := "unknown tool: " + b.ToolName
		out <- Event{Kind: KindToolResult, ToolUseID: b.ToolUseID, ToolName: b.ToolName, IsError: true, Text: msg}
		e.recordIfHooked(session.Record{
			Kind:       session.KindToolResult,
			ToolResult: &session.ToolResult{ToolUseID: b.ToolUseID, Text: msg, IsError: true},
		})
		return message.Block{Type: message.BlockToolResult, ToolUseID: b.ToolUseID, Text: msg, IsError: true}
	}
	out <- Event{Kind: KindToolUseRequest, ToolUseID: b.ToolUseID, ToolName: b.ToolName, ToolInput: b.Input}
	e.recordIfHooked(session.Record{
		Kind: session.KindToolUseRequest,
		ToolUseRequest: &session.ToolUseRequest{
			ToolUseID: b.ToolUseID, ToolName: b.ToolName, ToolInput: b.Input,
		},
	})

	// Consult the tool's own Permission opinion first. Tools that embed
	// DefaultPermission return BehaviorAsk (defer to gate). Tools that want a
	// hard answer (e.g. a future BashTool that knows a command is read-only)
	// can short-circuit the gate by returning Allow/Deny here.
	decision := t.Permission(ctx, b.Input)
	if decision.Behavior == permissions.BehaviorAsk {
		decision = permissions.Decide(e.cfg.Permissions, b.ToolName, b.Input)
	}
	// Apply any input mutation produced by the hook (ModifiedInput) so the
	// tool receives the rewritten input, not the original model-generated one.
	if decision.ModifiedInput != nil {
		b.Input = decision.ModifiedInput
	}
	if decision.Behavior == permissions.BehaviorAsk && e.cfg.RequestPrompt != nil {
		resp, err := e.cfg.RequestPrompt("tool", tool.PromptRequest{
			Kind:      tool.PromptToolPermission,
			ToolName:  b.ToolName,
			ToolInput: b.Input,
		})
		switch {
		case err != nil:
			decision = permissions.Decision{Behavior: permissions.BehaviorDeny, Reason: err.Error()}
		case resp.Allow:
			decision = permissions.Decision{Behavior: permissions.BehaviorAllow, Reason: "user approved"}
		default:
			decision = permissions.Decision{Behavior: permissions.BehaviorDeny, Reason: resp.Reason}
		}
	} else if decision.Behavior == permissions.BehaviorAsk {
		decision = permissions.Decision{Behavior: permissions.BehaviorDeny, Reason: "no prompt surface; ask denied"}
	}

	if decision.Behavior == permissions.BehaviorDeny {
		e.denials = append(e.denials, PermissionDenial{
			ToolName:  b.ToolName,
			ToolUseID: b.ToolUseID,
			ToolInput: b.Input,
		})
		msg := "permission denied: " + decision.Reason
		out <- Event{Kind: KindToolResult, ToolUseID: b.ToolUseID, ToolName: b.ToolName, IsError: true, Text: msg}
		e.recordIfHooked(session.Record{
			Kind:       session.KindToolResult,
			ToolResult: &session.ToolResult{ToolUseID: b.ToolUseID, Text: msg, IsError: true},
		})
		return message.Block{Type: message.BlockToolResult, ToolUseID: b.ToolUseID, Text: msg, IsError: true}
	}

	e.mu.Lock()
	msgsSnap := append([]message.Message(nil), e.messages...)
	e.mu.Unlock()
	tcx := &tool.Context{
		Cwd:           e.cfg.Cwd,
		Messages:      msgsSnap,
		Permissions:   e.cfg.Permissions,
		AbortContext:  ctx,
		RequestPrompt: e.cfg.RequestPrompt,
	}
	res, err := t.Call(ctx, b.Input, tcx)
	if err != nil {
		msg := err.Error()
		out <- Event{Kind: KindToolResult, ToolUseID: b.ToolUseID, ToolName: b.ToolName, IsError: true, Text: msg}
		e.recordIfHooked(session.Record{
			Kind:       session.KindToolResult,
			ToolResult: &session.ToolResult{ToolUseID: b.ToolUseID, Text: msg, IsError: true},
		})
		return message.Block{Type: message.BlockToolResult, ToolUseID: b.ToolUseID, Text: msg, IsError: true}
	}

	resultText := res.ModelText()
	// Invoke PostToolUse hook; any returned additional context is appended to result.
	if e.cfg.Hooks != nil {
		responseMap := map[string]any{"text": resultText, "is_error": res.IsError}
		if extra := e.cfg.Hooks.FirePostToolUse(ctx, b.ToolName, b.Input, responseMap); extra != "" {
			resultText = resultText + "\n\n" + extra
		}
	}

	out <- Event{Kind: KindToolResult, ToolUseID: b.ToolUseID, ToolName: b.ToolName, IsError: res.IsError, Text: resultText}
	e.recordIfHooked(session.Record{
		Kind:       session.KindToolResult,
		ToolResult: &session.ToolResult{ToolUseID: b.ToolUseID, Text: resultText, IsError: res.IsError},
	})
	return message.Block{
		Type:      message.BlockToolResult,
		ToolUseID: b.ToolUseID,
		Text:      resultText,
		IsError:   res.IsError,
	}
}

func (e *Engine) toolSpecs() []provider.ToolSpec {
	if e.cfg.Tools == nil {
		return nil
	}
	all := e.cfg.Tools.All()
	out := make([]provider.ToolSpec, 0, len(all))
	for _, t := range all {
		out = append(out, provider.ToolSpec{
			Name:        t.Name(),
			Description: t.Description(context.Background()),
			InputSchema: t.Schema(),
		})
	}
	return out
}

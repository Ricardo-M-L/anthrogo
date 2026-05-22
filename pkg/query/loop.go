package query

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"runtime/debug"
	"strconv"
	"sync"
	"time"

	"github.com/ricardo/anthrogo/internal/session"
	"github.com/ricardo/anthrogo/pkg/message"
	"github.com/ricardo/anthrogo/pkg/observability"
	"github.com/ricardo/anthrogo/pkg/permissions"
	"github.com/ricardo/anthrogo/pkg/provider"
	"github.com/ricardo/anthrogo/pkg/tool"
)

func (e *Engine) recordIfHooked(r session.Record) {
	if e.cfg.RecordHook != nil {
		e.cfg.RecordHook(r)
	}
}

// SubmitMessage runs one user turn with a plain-text prompt.
// It wraps the string in a single BlockText and delegates to SubmitMessageBlocks.
func (e *Engine) SubmitMessage(ctx context.Context, prompt string) <-chan Event {
	return e.SubmitMessageBlocks(ctx, []message.Block{{Type: message.BlockText, Text: prompt}})
}

// SubmitMessageBlocks runs one user turn — including any tool_use sub-turns —
// until the model emits end_turn (or an error/abort). It accepts a pre-built
// slice of content blocks (e.g. from message.ParseUserPrompt) so callers can
// include image blocks alongside text without constructing raw Messages.
func (e *Engine) SubmitMessageBlocks(ctx context.Context, blocks []message.Block) <-chan Event {
	out := make(chan Event, 64)
	e.mu.Lock()
	e.messages = append(e.messages, message.Message{Role: message.RoleUser, Content: blocks})
	lastContent := e.messages[len(e.messages)-1].Content
	// Reset lastUsage at the start of each new turn.
	e.lastUsage = message.Usage{}
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
				// Auto-compact check: after the turn completes, check if cumulative
				// usage since the last compact exceeds the configured threshold.
				// Uses usageSinceLastCompact (not per-turn lastUsage) so small turns
				// that aggregate to a large context also trigger compact.
				// Run synchronously so the next SubmitMessage call sees compacted history.
				if e.cfg.AutoCompactThreshold > 0 {
					e.mu.Lock()
					sc := e.usageSinceLastCompact
					e.mu.Unlock()
					if sc.InputTokens+sc.OutputTokens >= e.cfg.AutoCompactThreshold {
						keep := e.cfg.AutoCompactKeepRecent
						if keep == 0 {
							keep = 10
						}
						if _, compactErr := e.Compact(ctx, CompactOptions{
							Trigger:    "auto",
							KeepRecent: keep,
						}); compactErr != nil {
							log.Printf("anthrogo: auto-compact failed: %v", compactErr)
						}
					}
				}
				return
			}
		}
		out <- Event{Kind: KindError, Err: fmt.Errorf("max_turns exceeded")}
	}()
	return out
}

// isTransientStreamError reports whether err warrants a stream retry.
// It returns false for io.EOF (clean end), context cancellation/deadline, and nil.
func isTransientStreamError(err error) bool {
	if err == nil || errors.Is(err, io.EOF) {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	return true
}

// streamRetryDelays returns the backoff delays for successive retry attempts.
var streamRetryDelays = []time.Duration{200 * time.Millisecond, 600 * time.Millisecond, 2 * time.Second}

func (e *Engine) runOneAPITurn(ctx context.Context, out chan<- Event) (stopReason string, err error) {
	maxRetries := e.cfg.MaxStreamRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		stopReason, lastErr = e.runOneAPITurnAttempt(ctx, out)
		if lastErr == nil {
			return stopReason, nil
		}
		// Do not retry context cancellation / EOF.
		if !isTransientStreamError(lastErr) {
			return "", lastErr
		}
		// Exhausted retries.
		if attempt >= maxRetries {
			break
		}
		// Emit a retry event and wait.
		delay := streamRetryDelays[attempt]
		if attempt >= len(streamRetryDelays) {
			delay = streamRetryDelays[len(streamRetryDelays)-1]
		}
		out <- Event{
			Kind:         KindStreamRetry,
			RetryAttempt: attempt + 1,
			RetryDelayMs: int(delay.Milliseconds()),
			Err:          lastErr,
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(delay):
		}
	}
	return "", fmt.Errorf("stream retry exhausted: %w", lastErr)
}

func (e *Engine) runOneAPITurnAttempt(ctx context.Context, out chan<- Event) (stopReason string, err error) {
	ctx, span := observability.Tracer().Start(ctx, "engine.turn")
	defer func() {
		if stopReason != "" {
			observability.TurnsTotal.WithLabelValues(stopReason).Inc()
		}
		span.End()
	}()

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

	// Wrap the provider.Stream call in its own span so we can attribute errors.
	provCtx, provSpan := observability.Tracer().Start(ctx, "provider.Stream "+e.cfg.Model)
	ch, err := e.cfg.Provider.Stream(provCtx, req)
	if err != nil {
		transient := isTransientStreamError(err)
		observability.ProviderStreamErrorsTotal.WithLabelValues("provider", strconv.FormatBool(transient)).Inc()
		provSpan.End()
		return "", err
	}
	provSpan.End()

	// Accumulate assistant content blocks as we stream.
	var (
		assistant     []message.Block
		textBuf       bytes.Buffer
		pendingTool   *message.Block
		toolInputJSON bytes.Buffer
		sawStop       bool
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
			e.mu.Lock()
			e.usage.Add(ev.Usage)
			e.lastUsage.Add(ev.Usage)
			e.usageSinceLastCompact.Add(ev.Usage)
			e.mu.Unlock()
			// Mirror token counts to Prometheus.
			if ev.Usage.InputTokens > 0 {
				observability.TokensIn.Add(float64(ev.Usage.InputTokens))
			}
			if ev.Usage.OutputTokens > 0 {
				observability.TokensOut.Add(float64(ev.Usage.OutputTokens))
			}
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
			sawStop = true
		case provider.EventError:
			transient := !sawStop && isTransientStreamError(ev.Err)
			observability.ProviderStreamErrorsTotal.WithLabelValues("provider", strconv.FormatBool(transient)).Inc()
			if transient {
				// Transient mid-stream error before MessageStop — signal for retry.
				// Discard partial assistant content; the retry will regenerate.
				return "", ev.Err
			}
			return "", ev.Err
		}
	}

	if len(assistant) > 0 {
		e.mu.Lock()
		e.messages = append(e.messages, message.Message{Role: message.RoleAssistant, Content: assistant})
		e.mu.Unlock()
		e.recordIfHooked(session.Record{
			Kind:             session.KindAssistantMessage,
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
			// Parallel dispatch with cancel-safe drain.
			// Goroutines write into fixed result slots so tool_result order
			// matches the tool_use order.
			var wg sync.WaitGroup
			// out channel is buffered (64); concurrent writes are safe.
			for i, b := range toolBlocks {
				wg.Add(1)
				go func(idx int, blk message.Block) {
					defer wg.Done()
					results[idx] = e.executeToolSafe(ctx, blk, out)
				}(i, b)
			}

			// Cancel-safe drain: if ctx fires before tools complete, wait up to
			// MaxToolDrainTimeout before giving up, emitting periodic drain events.
			done := make(chan struct{})
			go func() {
				wg.Wait()
				close(done)
			}()

			drainTimeout := e.cfg.MaxToolDrainTimeout
			if drainTimeout <= 0 {
				drainTimeout = 5 * time.Second
			}

			select {
			case <-done:
				// All tools finished cleanly.
			case <-ctx.Done():
				// Context cancelled; give in-flight tools a bounded drain window.
				deadline := time.Now().Add(drainTimeout)
				ticker := time.NewTicker(100 * time.Millisecond)
				defer ticker.Stop()
			drainLoop:
				for {
					select {
					case <-done:
						break drainLoop
					case <-ticker.C:
						remaining := time.Until(deadline)
						if remaining <= 0 {
							log.Printf("anthrogo: cancel drain timeout (%s) exceeded; %d tool goroutine(s) may still be running",
								drainTimeout, len(toolBlocks))
							break drainLoop
						}
						out <- Event{
							Kind:          KindCancelDraining,
							InFlightCount: len(toolBlocks),
							RemainingMs:   int(remaining.Milliseconds()),
						}
					}
				}
			}
		} else {
			for i, b := range toolBlocks {
				results[i] = e.executeToolSafe(ctx, b, out)
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

// executeToolSafe wraps executeTool with a panic recovery. A panic in any
// tool's Call() previously took down the entire process; now it becomes an
// IsError tool_result with the panic message + stack logged.
func (e *Engine) executeToolSafe(ctx context.Context, b message.Block, out chan<- Event) (res message.Block) {
	defer func() {
		if r := recover(); r != nil {
			msg := fmt.Sprintf("tool %s panicked: %v", b.ToolName, r)
			log.Printf("anthrogo: query: %s\n%s", msg, debug.Stack())
			out <- Event{Kind: KindToolResult, ToolUseID: b.ToolUseID, ToolName: b.ToolName, IsError: true, Text: msg}
			res = message.Block{
				Type:      message.BlockToolResult,
				ToolUseID: b.ToolUseID,
				Text:      msg,
				IsError:   true,
			}
		}
	}()
	return e.executeTool(ctx, b, out)
}

func (e *Engine) executeTool(ctx context.Context, b message.Block, out chan<- Event) (result message.Block) {
	// Start an OTel span and duration timer for this tool call.
	ctx, span := observability.Tracer().Start(ctx, "tool."+b.ToolName)
	toolStart := time.Now()
	defer func() {
		span.End()
		elapsed := time.Since(toolStart).Seconds()
		observability.ToolDuration.WithLabelValues(b.ToolName).Observe(elapsed)
		observability.ToolCallsTotal.WithLabelValues(b.ToolName, strconv.FormatBool(result.IsError)).Inc()
	}()

	// When a ToolDispatcher is configured it takes full responsibility for
	// executing the tool (including any permission gate on the caller's side).
	// The local registry lookup and permissions.Decide path are both skipped.
	if e.cfg.ToolDispatcher != nil {
		out <- Event{Kind: KindToolUseRequest, ToolUseID: b.ToolUseID, ToolName: b.ToolName, ToolInput: b.Input}
		e.recordIfHooked(session.Record{
			Kind: session.KindToolUseRequest,
			ToolUseRequest: &session.ToolUseRequest{
				ToolUseID: b.ToolUseID, ToolName: b.ToolName, ToolInput: b.Input,
			},
		})
		res, err := e.cfg.ToolDispatcher(ctx, b.ToolUseID, b.ToolName, b.Input)
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
		decision = permissions.Decide(ctx, e.cfg.Permissions, b.ToolName, b.Input)
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
		e.mu.Lock()
		e.denials = append(e.denials, PermissionDenial{
			ToolName:  b.ToolName,
			ToolUseID: b.ToolUseID,
			ToolInput: b.Input,
		})
		e.mu.Unlock()
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
		Cwd:                 e.cfg.Cwd,
		Messages:            msgsSnap,
		Permissions:         e.cfg.Permissions,
		AbortContext:        ctx,
		RequestPrompt:       e.cfg.RequestPrompt,
		SubagentPrefixChain: e.cfg.SubagentPrefixChain,
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
	// Invoke PostToolUse hook; any returned additional context is appended
	// to the tool result. The hook output is delimited by HOOK_CONTEXT tags
	// + capped at 32 KiB so a compromised or buggy hook cannot:
	//   - inject instructions that look like model-authored content
	//     (HOOK_CONTEXT tags clearly fence the boundary)
	//   - balloon a tool_result to consume the context window
	//     (cap with truncation marker)
	// The trust model: user-configured hooks are trusted by definition,
	// but defense-in-depth helps when hook scripts are themselves
	// generated or fetched (template engines, shell shared with other
	// processes, etc.).
	if e.cfg.Hooks != nil {
		responseMap := map[string]any{"text": resultText, "is_error": res.IsError}
		if extra := e.cfg.Hooks.FirePostToolUse(ctx, b.ToolName, b.Input, responseMap); extra != "" {
			const maxHookExtraBytes = 32 * 1024
			if len(extra) > maxHookExtraBytes {
				extra = extra[:maxHookExtraBytes] + "\n... [hook output truncated]"
			}
			resultText = resultText + "\n\n<hook_additional_context>\n" + extra + "\n</hook_additional_context>"
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

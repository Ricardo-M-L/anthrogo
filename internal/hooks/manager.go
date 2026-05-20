package hooks

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ManagerOptions holds session-level metadata for the Manager.
type ManagerOptions struct {
	SessionID string
	Cwd       string
	Version   string
	// LogSink, if non-nil, is called for informational/error log lines.
	// eventName is the hook event (e.g. "PreToolUse"); msg is free-form text.
	LogSink func(eventName, msg string)
}

// Manager coordinates hook dispatch for all nine events.
type Manager struct {
	cfg  Config
	opts ManagerOptions
	wg   sync.WaitGroup
}

// NewManager creates a Manager. cfg should already have been Validate()d.
func NewManager(cfg Config, opts ManagerOptions) *Manager {
	return &Manager{cfg: cfg, opts: opts}
}

// log emits a log line via LogSink if set.
func (m *Manager) log(event, msg string) {
	if m.opts.LogSink != nil {
		m.opts.LogSink(event, msg)
	}
}

// common builds the Common envelope for payloads.
func (m *Manager) common(name EventName) Common {
	return Common{
		HookEventName: name,
		SessionID:     m.opts.SessionID,
		Cwd:           m.opts.Cwd,
		Version:       m.opts.Version,
	}
}

// matchingHooks returns hooks from list whose matcher (if any) matches toolName.
func matchingHooks(list []Spec, toolName string) []Spec {
	var out []Spec
	for _, s := range list {
		if s.matcherRE == nil || s.matcherRE.MatchString(toolName) {
			out = append(out, s)
		}
	}
	return out
}

// FirePreToolUse runs PreToolUse hooks sequentially and returns a Decision.
//
// Exit semantics:
//   - timeout          → DecisionDeny, reason "hook X timeout"
//   - exit 2           → DecisionDeny, reason = stderr text
//   - exit non-zero/non-2 → DecisionDeny, reason "hook X exited N"
//   - exit 0, permissionDecision "deny"  → DecisionDeny immediately
//   - exit 0, permissionDecision "allow" → mark DecisionAllow, keep looping
//   - exit 0, modifiedInput present      → merge into accumulated ModifiedInput
func (m *Manager) FirePreToolUse(ctx context.Context, toolName string, input map[string]any) Decision {
	hooks := matchingHooks(m.cfg.PreToolUse, toolName)

	// Work with a copy of input so we can accumulate modifications.
	curInput := shallowCopy(input)

	d := Decision{Behavior: DecisionPass}

	for _, spec := range hooks {
		payload := PreToolUsePayload{
			Common:    m.common(EventPreToolUse),
			ToolName:  toolName,
			ToolInput: curInput,
		}

		res, err := RunHook(ctx, spec, payload)
		if err != nil {
			m.log(string(EventPreToolUse), fmt.Sprintf("hook %s start error: %v", spec.Command, err))
			return Decision{Behavior: DecisionDeny, Reason: fmt.Sprintf("hook %s start error: %v", spec.Command, err)}
		}

		switch {
		case res.TimedOut:
			return Decision{Behavior: DecisionDeny, Reason: fmt.Sprintf("hook %s timeout", spec.Command)}

		case res.ExitCode == 2:
			reason := strings.TrimSpace(string(res.Stderr))
			return Decision{Behavior: DecisionDeny, Reason: reason}

		case res.ExitCode != 0:
			return Decision{
				Behavior: DecisionDeny,
				Reason:   fmt.Sprintf("hook %s exited %d", spec.Command, res.ExitCode),
			}
		}

		// exit 0 — inspect JSON output if present.
		if res.Output != nil && res.Output.HookSpecificOutput != nil {
			hso := res.Output.HookSpecificOutput

			// Merge ModifiedInput into accumulated state.
			if len(hso.ModifiedInput) > 0 {
				for k, v := range hso.ModifiedInput {
					curInput[k] = v
				}
				if d.ModifiedInput == nil {
					d.ModifiedInput = make(map[string]any)
				}
				for k, v := range hso.ModifiedInput {
					d.ModifiedInput[k] = v
				}
			}

			switch strings.ToLower(hso.PermissionDecision) {
			case "deny":
				reason := hso.PermissionDecisionReason
				if reason == "" {
					reason = fmt.Sprintf("hook %s denied", spec.Command)
				}
				return Decision{Behavior: DecisionDeny, Reason: reason}
			case "allow":
				// Keep looping; a later hook can still deny.
				d.Behavior = DecisionAllow
			}
		}
	}

	return d
}

// FirePostToolUse runs PostToolUse hooks sequentially and returns concatenated
// additionalContext strings. Errors are logged only, never propagated.
func (m *Manager) FirePostToolUse(ctx context.Context, toolName string, input map[string]any, response map[string]any) string {
	hooks := matchingHooks(m.cfg.PostToolUse, toolName)

	var parts []string
	for _, spec := range hooks {
		payload := PostToolUsePayload{
			Common:       m.common(EventPostToolUse),
			ToolName:     toolName,
			ToolInput:    input,
			ToolResponse: response,
		}
		res, err := RunHook(ctx, spec, payload)
		if err != nil {
			m.log(string(EventPostToolUse), fmt.Sprintf("hook %s start error: %v", spec.Command, err))
			continue
		}
		if res.TimedOut {
			m.log(string(EventPostToolUse), fmt.Sprintf("hook %s timed out", spec.Command))
			continue
		}
		if res.ExitCode != 0 {
			m.log(string(EventPostToolUse), fmt.Sprintf("hook %s exited %d: %s", spec.Command, res.ExitCode, strings.TrimSpace(string(res.Stderr))))
			continue
		}
		if res.Output != nil && res.Output.HookSpecificOutput != nil {
			if ctx := res.Output.HookSpecificOutput.AdditionalContext; ctx != "" {
				parts = append(parts, ctx)
			}
		}
	}
	return strings.Join(parts, "\n\n")
}

// FireUserPromptSubmit runs UserPromptSubmit hooks and returns:
//
//	(ctx, finalPrompt, abort, reason)
//
// Sequential; any exit-2 or timeout causes abort=true.
// Exit 0 with additionalContext appends it to the prompt.
func (m *Manager) FireUserPromptSubmit(ctx context.Context, prompt string) (context.Context, string, bool, string) {
	hooks := m.cfg.UserPromptSubmit

	parts := []string{prompt}

	for _, spec := range hooks {
		payload := UserPromptSubmitPayload{
			Common: m.common(EventUserPromptSubmit),
			Prompt: prompt,
		}
		res, err := RunHook(ctx, spec, payload)
		if err != nil {
			m.log(string(EventUserPromptSubmit), fmt.Sprintf("hook %s start error: %v", spec.Command, err))
			return ctx, prompt, true, fmt.Sprintf("hook %s start error: %v", spec.Command, err)
		}

		if res.TimedOut {
			return ctx, prompt, true, fmt.Sprintf("hook %s timeout", spec.Command)
		}
		if res.ExitCode == 2 {
			reason := strings.TrimSpace(string(res.Stderr))
			return ctx, prompt, true, reason
		}
		if res.ExitCode != 0 {
			return ctx, prompt, true, fmt.Sprintf("hook %s exited %d", spec.Command, res.ExitCode)
		}

		// exit 0 — collect additionalContext.
		if res.Output != nil && res.Output.HookSpecificOutput != nil {
			if ac := res.Output.HookSpecificOutput.AdditionalContext; ac != "" {
				parts = append(parts, ac)
			}
		}
	}

	finalPrompt := strings.Join(parts, "\n\n")
	return ctx, finalPrompt, false, ""
}

// fireAsync runs hooks asynchronously (fire-and-forget). One goroutine iterates
// the hooks sequentially; errors are logged. ctx cancellation aborts mid-iteration.
func (m *Manager) fireAsync(ctx context.Context, eventName EventName, hooks []Spec, payload any) {
	if len(hooks) == 0 {
		return
	}
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		for _, spec := range hooks {
			select {
			case <-ctx.Done():
				m.log(string(eventName), "aborted by ctx cancel")
				return
			default:
			}
			res, err := RunHook(ctx, spec, payload)
			if err != nil {
				m.log(string(eventName), fmt.Sprintf("hook %s start error: %v", spec.Command, err))
				continue
			}
			if res.TimedOut {
				m.log(string(eventName), fmt.Sprintf("hook %s timed out", spec.Command))
				continue
			}
			if res.ExitCode != 0 {
				m.log(string(eventName), fmt.Sprintf("hook %s exited %d: %s", spec.Command, res.ExitCode, strings.TrimSpace(string(res.Stderr))))
			}
		}
	}()
}

// Drain blocks until all in-flight async hook goroutines finish or timeout elapses.
func (m *Manager) Drain(timeout time.Duration) {
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
	}
}

// FireStop fires Stop hooks asynchronously.
func (m *Manager) FireStop(ctx context.Context, stopReason string) {
	payload := StopPayload{
		Common:     m.common(EventStop),
		StopReason: stopReason,
	}
	m.fireAsync(ctx, EventStop, m.cfg.Stop, payload)
}

// FireSubagentStop fires SubagentStop hooks asynchronously.
func (m *Manager) FireSubagentStop(ctx context.Context, stopReason string) {
	payload := StopPayload{
		Common:     m.common(EventSubagentStop),
		StopReason: stopReason,
	}
	m.fireAsync(ctx, EventSubagentStop, m.cfg.SubagentStop, payload)
}

// FireNotification fires Notification hooks asynchronously.
func (m *Manager) FireNotification(ctx context.Context, message, kind string) {
	payload := NotificationPayload{
		Common:  m.common(EventNotification),
		Message: message,
		Kind:    kind,
	}
	m.fireAsync(ctx, EventNotification, m.cfg.Notification, payload)
}

// FireSessionStart fires SessionStart hooks asynchronously.
func (m *Manager) FireSessionStart(ctx context.Context, kind string) {
	payload := SessionStartPayload{
		Common: m.common(EventSessionStart),
		Kind:   kind,
	}
	m.fireAsync(ctx, EventSessionStart, m.cfg.SessionStart, payload)
}

// FireSessionEnd fires SessionEnd hooks asynchronously.
func (m *Manager) FireSessionEnd(ctx context.Context, kind string) {
	payload := SessionEndPayload{
		Common: m.common(EventSessionEnd),
		Kind:   kind,
	}
	m.fireAsync(ctx, EventSessionEnd, m.cfg.SessionEnd, payload)
}

// FirePreCompact runs PreCompact hooks synchronously (log-only per spec §5.2).
func (m *Manager) FirePreCompact(ctx context.Context, trigger string) {
	payload := PreCompactPayload{
		Common:  m.common(EventPreCompact),
		Trigger: trigger,
	}
	for _, spec := range m.cfg.PreCompact {
		res, err := RunHook(ctx, spec, payload)
		if err != nil {
			m.log(string(EventPreCompact), fmt.Sprintf("hook %s start error: %v", spec.Command, err))
			continue
		}
		if res.TimedOut {
			m.log(string(EventPreCompact), fmt.Sprintf("hook %s timed out", spec.Command))
			continue
		}
		if res.ExitCode != 0 {
			m.log(string(EventPreCompact), fmt.Sprintf("hook %s exited %d: %s", spec.Command, res.ExitCode, strings.TrimSpace(string(res.Stderr))))
		}
	}
}

// shallowCopy returns a one-level copy of m so mutations don't affect the original.
func shallowCopy(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

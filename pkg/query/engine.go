package query

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"strings"
	"sync"

	"github.com/google/uuid"

	"github.com/ricardo/anthrogo/internal/hooks"
	"github.com/ricardo/anthrogo/internal/session"
	"github.com/ricardo/anthrogo/pkg/compact"
	"github.com/ricardo/anthrogo/pkg/kairos"
	"github.com/ricardo/anthrogo/pkg/message"
	"github.com/ricardo/anthrogo/pkg/permissions"
	"github.com/ricardo/anthrogo/pkg/pricing"
	"github.com/ricardo/anthrogo/pkg/provider"
	"github.com/ricardo/anthrogo/pkg/subagent"
	"github.com/ricardo/anthrogo/pkg/tool"
)

// HookSink is the subset of hooks.Manager that the query engine needs.
// All methods are nil-safe when the field is nil.
type HookSink interface {
	FirePostToolUse(ctx context.Context, toolName string, input, response map[string]any) string
	FireStop(ctx context.Context, reason string)
	FirePreCompact(ctx context.Context, trigger string)
	FireSubagentStop(ctx context.Context, reason string)
}

// ToolDispatcher is a pluggable hook that replaces local tools.Registry dispatch.
// When non-nil in Config, the engine calls it instead of looking up and calling
// the tool from the registry. The caller is responsible for applying its own
// permission gate; the normal permissions.Decide path is skipped for this path.
// This is used by the KAIROS worker to forward tool calls back to the client
// process when exec-tools-locally mode is active.
type ToolDispatcher func(ctx context.Context, toolUseID, toolName string, input map[string]any) (tool.Result, error)

type Config struct {
	Provider     provider.Provider
	Tools        *tool.Registry
	Permissions  *permissions.Context
	Model        string
	SystemPrompt string
	MaxTokens    int
	Temperature  float64
	Cwd          string
	MaxTurns     int // 0 = unlimited

	// ToolDispatcher, when non-nil, replaces the local tools.Registry dispatch.
	// The permissions gate (permissions.Decide) is NOT applied on this path —
	// the dispatcher itself (e.g. the remote client) applies its own gate.
	// When nil the engine uses the normal local dispatch with full permission checking.
	ToolDispatcher ToolDispatcher

	// Surface hooks; nil-safe.
	RequestPrompt   func(source string, req tool.PromptRequest) (tool.PromptResponse, error)
	AppendUIMessage func(msg string)
	RecordHook      func(session.Record)

	// Hooks is an optional hook sink for PostToolUse, Stop, and SubagentStop events.
	Hooks HookSink

	// HooksConfig, when non-nil, is the raw hooks.Config that was used to build
	// the Hooks manager. It is forwarded to KAIROS workers via RemoteContext so
	// the worker can apply the client's hook rules to its subagent run.
	HooksConfig *hooks.Config

	// Session is the parent session Store. If non-nil, Engine.RunSubagent
	// creates an independent JSONL file under <Session.Path>/subagents/<subagent-id>.jsonl
	// and routes the subagent's records there. nil-safe — subagent runs without
	// persistence if absent.
	Session *session.Store

	// Subagent configuration.
	SubagentRegistry *subagent.Registry
	SubagentDepth    int // set by parent engine when constructing child; 0 at top level
	MaxSubagentDepth int // default 3 if zero

	// SubagentPrefixChain carries ancestor Task descriptions for nested
	// subagent prefix construction. Propagated into tool.Context per call.
	SubagentPrefixChain []string

	// AutoCompactThreshold is the combined (input + output) token count at
	// which Engine automatically fires Compact() at the boundary between
	// turns. 0 disables (default).
	AutoCompactThreshold int

	// AutoCompactKeepRecent is passed to Compact(). 0 → default 10.
	AutoCompactKeepRecent int

	// Pricing is the optional pricing table used by EstimatedCost(). nil = no cost tracking.
	Pricing *pricing.Table

	// CostLimitUSD, when > 0 and Pricing != nil, enables hard budget enforcement.
	// IsOverBudget() returns true once the cumulative session cost >= this value.
	CostLimitUSD float64

	// KairosTrustKey, when non-nil, is the global ed25519 public key used to
	// verify SSE signatures for ALL remote KAIROS subagent dispatches. A per-spec
	// trust_key in the subagent YAML takes precedence over this global setting.
	KairosTrustKey ed25519.PublicKey
}

// Engine owns one conversation. Each SubmitMessage starts a new turn within
// the same conversation; messages, usage, cwd persist across turns.
type Engine struct {
	mu            sync.Mutex
	cfg           Config
	messages      []message.Message
	usage         message.Usage
	// lastUsage holds the most recently observed Usage from a provider stream.
	// Reset to message.Usage{} at the start of each new turn; updated on every
	// EventUsage; consulted at end-of-turn for auto-compact decision.
	lastUsage     message.Usage
	// usageSinceLastCompact accumulates token usage since the session started
	// or since the most recent successful Compact(), whichever is later.
	// Used for the auto-compact threshold check instead of per-turn lastUsage.
	// Reset to zero inside Compact() when Skipped==false.
	usageSinceLastCompact message.Usage
	denials       []PermissionDenial
	subagentDepth int
}

func NewEngine(cfg Config) *Engine {
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 4096
	}
	return &Engine{cfg: cfg, subagentDepth: cfg.SubagentDepth}
}

// SystemPrompt returns the system prompt that was configured for this engine.
func (e *Engine) SystemPrompt() string { return e.cfg.SystemPrompt }

// Model returns the model name that was configured for this engine.
func (e *Engine) Model() string { return e.cfg.Model }

// SubagentOptions carries the parameters for running a subagent.
type SubagentOptions struct {
	Type        string
	Description string
	Prompt      string
	// OnTextDelta, if non-nil, fires for every EventTextDelta from the child.
	// Called from a background goroutine; implementations must be thread-safe.
	OnTextDelta func(delta string)
	// PrefixChain carries outer Task descriptions for nested prefix display.
	// When a Task tool invokes RunSubagent, it passes its own description so
	// the inner task's TUI prefix chains as "outer → inner".
	PrefixChain []string
}

// RunSubagent spawns a child Engine for the named subagent type, runs one turn
// with opts.Prompt, drains the stream collecting the last assistant turn's text,
// fires the SubagentStop hook, and returns the result.
func (e *Engine) RunSubagent(ctx context.Context, opts SubagentOptions) (string, error) {
	// 1. Depth check + increment under lock.
	e.mu.Lock()
	maxDepth := e.cfg.MaxSubagentDepth
	if maxDepth <= 0 {
		maxDepth = 3
	}
	if e.subagentDepth >= maxDepth {
		e.mu.Unlock()
		return "", fmt.Errorf("subagent depth limit (%d) exceeded", maxDepth)
	}
	e.subagentDepth++
	currentDepth := e.subagentDepth
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		e.subagentDepth--
		e.mu.Unlock()
	}()

	// 2. Registry lookup.
	if e.cfg.SubagentRegistry == nil {
		return "", fmt.Errorf("subagent: no registry configured")
	}
	spec, ok := e.cfg.SubagentRegistry.Get(opts.Type)
	if !ok {
		return "", fmt.Errorf("subagent: unknown type %q", opts.Type)
	}

	// 2a. Remote dispatch: if the spec has a RemoteSpec, route via HTTP instead
	// of spawning a local child Engine. Hooks still fire locally.
	if spec.Remote != nil {
		// Build a RemoteContext so the worker can inherit the client's
		// hook config and permission rules.
		remoteCtx := &kairos.RemoteContext{
			HopDepth: 0, // top-level client; worker increments on forward
		}
		if e.cfg.HooksConfig != nil {
			cfg := *e.cfg.HooksConfig // shallow copy — spec slices are immutable at runtime
			remoteCtx.Hooks = &cfg
		}
		if e.cfg.Permissions != nil {
			remoteCtx.Permissions = &kairos.PermSnapshot{
				Mode:             string(e.cfg.Permissions.Mode),
				AlwaysAllowRules: e.cfg.Permissions.AlwaysAllowRules,
				AlwaysDenyRules:  e.cfg.Permissions.AlwaysDenyRules,
				AlwaysAskRules:   e.cfg.Permissions.AlwaysAskRules,
			}
		}

		var text string
		var err error
		// Resolve optional trust key: per-spec YAML takes precedence over global.
		var clientOpts kairos.ClientOptions
		if spec.Remote.TrustKey != "" {
			pub, trustErr := kairos.LoadPublicKey(spec.Remote.TrustKey)
			if trustErr != nil {
				return "", fmt.Errorf("kairos: trust_key for %q: %w", spec.Name, trustErr)
			}
			clientOpts.TrustKey = pub
		} else if len(e.cfg.KairosTrustKey) > 0 {
			clientOpts.TrustKey = e.cfg.KairosTrustKey
		}
		if spec.Remote.ExecToolsLocally {
			// Exec-tools-locally: tool calls from the remote subagent run on this
			// (client) process using the parent engine's tool registry and
			// permission context.
			clientOpts.AuthToken = spec.Remote.AuthToken
			clientOpts.ExecToolsLocally = true
			clientOpts.ToolRegistry = e.cfg.Tools
			clientOpts.Permissions = e.cfg.Permissions
			clientOpts.OnTextDelta = opts.OnTextDelta
			text, err = kairos.DispatchRemoteWithOptions(ctx, spec.Remote.Endpoint,
				kairos.RunRequest{
					SubagentType:  opts.Type,
					Description:   opts.Description,
					Prompt:        opts.Prompt,
					RemoteContext: remoteCtx,
				},
				clientOpts,
			)
		} else {
			clientOpts.AuthToken = spec.Remote.AuthToken
			clientOpts.OnTextDelta = opts.OnTextDelta
			text, err = kairos.DispatchRemoteWithOptions(ctx, spec.Remote.Endpoint,
				kairos.RunRequest{
					SubagentType:  opts.Type,
					Description:   opts.Description,
					Prompt:        opts.Prompt,
					RemoteContext: remoteCtx,
				},
				clientOpts,
			)
		}
		if e.cfg.Hooks != nil {
			if err != nil {
				e.cfg.Hooks.FireSubagentStop(ctx, "error")
			} else {
				e.cfg.Hooks.FireSubagentStop(ctx, "end_turn")
			}
		}
		return text, err
	}

	// 2b. Mint a subagent ID and create an independent JSONL store when a
	// parent session is available.
	subagentID := uuid.New().String()
	var childRecordHook func(session.Record)
	var childSessionStore *session.Store

	if e.cfg.Session != nil {
		store, err := session.NewSubagent(e.cfg.Session, subagentID)
		if err == nil {
			childSessionStore = store
			childRecordHook = store.NewRecordHook()
		}
		// On error: degrade gracefully — subagent runs without persistence.
	}

	// Emit a KindSubagentStart record into the parent JSONL for correlation.
	if e.cfg.RecordHook != nil {
		e.cfg.RecordHook(session.Record{
			Kind: session.KindSubagentStart,
			Subagent: &session.SubagentRecord{
				ID:          subagentID,
				Type:        opts.Type,
				Description: opts.Description,
			},
		})
	}

	defer func() {
		if childSessionStore != nil {
			_ = childSessionStore.Close()
		}
	}()

	// 3. Build filtered tool registry (or share parent's).
	var childTools *tool.Registry
	if len(spec.ToolAllowlist) > 0 {
		childTools = tool.NewRegistry()
		allow := make(map[string]bool, len(spec.ToolAllowlist))
		for _, n := range spec.ToolAllowlist {
			allow[n] = true
		}
		for _, t := range e.cfg.Tools.All() {
			if allow[t.Name()] {
				childTools.Register(t)
			}
		}
	} else {
		// Share parent's tool registry. Safe today because tool.Registry is
		// frozen at startup (cmd/anthrogo populates it once before Run). If a
		// future change makes the registry hot-mutable, defensive copy here.
		childTools = e.cfg.Tools
	}

	// 4. Build child Config.
	// Clone the permissions context so subagent Mode toggles don't affect parent.
	childPerms := e.cfg.Permissions.Clone()
	// Build the propagated prefix chain: parent chain + this subagent's description.
	childPrefixChain := make([]string, len(opts.PrefixChain)+1)
	copy(childPrefixChain, opts.PrefixChain)
	childPrefixChain[len(opts.PrefixChain)] = opts.Description
	childCfg := Config{
		Provider:            e.cfg.Provider,
		Model:               e.cfg.Model,
		Tools:               childTools,
		Permissions:         childPerms,
		SystemPrompt:        e.cfg.SystemPrompt + spec.SystemPromptSuffix,
		Hooks:               e.cfg.Hooks,
		Cwd:                 e.cfg.Cwd,
		RecordHook:          childRecordHook,
		Session:             childSessionStore, // enables nested sub-sub-agent JSONL at <parent>/<sub-id>/subagents/<sub-sub-id>.jsonl
		SubagentRegistry:    e.cfg.SubagentRegistry,
		SubagentDepth:       currentDepth,
		MaxSubagentDepth:    maxDepth,
		MaxTokens:           e.cfg.MaxTokens,
		Temperature:         e.cfg.Temperature,
		MaxTurns:            e.cfg.MaxTurns,
		SubagentPrefixChain: childPrefixChain,
	}

	child := NewEngine(childCfg)

	// 5. Run one turn, drain the channel collecting the LAST assistant turn's text.
	// We reset the buffer on every KindAssistantStop so we end up with only the
	// final assistant turn (the child may cycle through tool_use sub-turns).
	ch := child.SubmitMessage(ctx, opts.Prompt)
	var buf strings.Builder
	for ev := range ch {
		switch ev.Kind {
		case KindAssistantDelta:
			if opts.OnTextDelta != nil {
				opts.OnTextDelta(ev.Text)
			}
			buf.WriteString(ev.Text)
		case KindAssistantStop:
			// KindAssistantStop fires after every assistant message (including
			// intermediate ones before tool_use cycles). Reset to keep only the
			// final assistant message text.
			if ev.StopReason == "tool_use" {
				// intermediate turn — clear for next accumulation
				buf.Reset()
			}
			// if end_turn / other, keep (will be returned)
		case KindError:
			if e.cfg.Hooks != nil {
				e.cfg.Hooks.FireSubagentStop(ctx, "error")
			}
			return "", ev.Err
		}
	}

	// 6. Fire SubagentStop with success.
	if e.cfg.Hooks != nil {
		e.cfg.Hooks.FireSubagentStop(ctx, "end_turn")
	}

	return strings.TrimSpace(buf.String()), nil
}

func (e *Engine) Messages() []message.Message {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]message.Message, len(e.messages))
	copy(out, e.messages)
	return out
}

func (e *Engine) SetInitialMessages(msgs []message.Message) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if msgs == nil {
		e.messages = nil
		return
	}
	e.messages = append([]message.Message(nil), msgs...)
}

func (e *Engine) Usage() message.Usage {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.usage
}

// LastUsage returns the most recently observed Usage from the last turn's stream.
func (e *Engine) LastUsage() message.Usage {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.lastUsage
}

// UsageSinceLastCompact returns the cumulative token usage accumulated since
// the session started or since the most recent successful Compact(), whichever
// is later. Updated under lock on every EventUsage; reset by Compact when
// Skipped==false.
func (e *Engine) UsageSinceLastCompact() message.Usage {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.usageSinceLastCompact
}

// AutoCompactConfig returns the configured auto-compact threshold and
// keep-recent values. A threshold of 0 means auto-compact is disabled.
func (e *Engine) AutoCompactConfig() (threshold, keep int) {
	return e.cfg.AutoCompactThreshold, e.cfg.AutoCompactKeepRecent
}

// EstimatedCost returns the estimated USD cost of cumulative session usage,
// or (0, false) if no pricing table is configured or no matching model rate
// is found.
func (e *Engine) EstimatedCost() (float64, bool) {
	if e.cfg.Pricing == nil {
		return 0, false
	}
	rate, ok := e.cfg.Pricing.Lookup(e.cfg.Model)
	if !ok {
		return 0, false
	}
	u := e.Usage()
	return pricing.EstimateUSD(rate, u.InputTokens, u.OutputTokens), true
}

// ResetUsage zeroes the cumulative usage counter (and the since-last-compact
// counter). The last-turn usage is left intact. Tool-budget calculations
// based on Usage() will see zero until the next EventUsage arrives.
func (e *Engine) ResetUsage() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.usage = message.Usage{}
	e.usageSinceLastCompact = message.Usage{}
}

// IsOverBudget reports whether the session's cumulative estimated cost has
// reached or exceeded the configured CostLimitUSD. It returns (over, current,
// limit). When no limit is configured or pricing is unavailable it returns
// (false, 0, 0).
func (e *Engine) IsOverBudget() (bool, float64, float64) {
	if e.cfg.CostLimitUSD <= 0 {
		return false, 0, 0
	}
	cur, ok := e.EstimatedCost()
	if !ok {
		return false, 0, e.cfg.CostLimitUSD
	}
	return cur >= e.cfg.CostLimitUSD, cur, e.cfg.CostLimitUSD
}

// CompactOptions controls Engine.Compact behaviour.
type CompactOptions struct {
	KeepRecent int    // default 10
	Trigger    string // "manual" | "auto"
}

// Summary is the result of Engine.Compact.
type Summary struct {
	OriginalCount  int
	NewCount       int
	OriginalTokens int
	NewTokens      int
	SummaryText    string
	Skipped        bool
	SkipReason     string
	Trigger        string
}

// Compact summarizes earlier conversation turns to reduce context size.
// It fires the PreCompact hook, calls compact.Run, then swaps the engine's
// message list and emits a compact record via RecordHook.
//
// The lock is held only to take a snapshot before the (slow) LLM call and to
// install the result after; the LLM call itself runs without the lock so
// concurrent reads of Messages() or SubmitMessage turns are not blocked for
// the entire summarisation duration.
func (e *Engine) Compact(ctx context.Context, opts CompactOptions) (Summary, error) {
	if opts.Trigger == "" {
		opts.Trigger = "manual"
	}
	if e.cfg.Hooks != nil {
		e.cfg.Hooks.FirePreCompact(ctx, opts.Trigger)
	}

	// Snapshot current messages under lock; release before the LLM call.
	e.mu.Lock()
	msgs := append([]message.Message(nil), e.messages...)
	e.mu.Unlock()

	in := compact.Input{
		Provider:   e.cfg.Provider,
		Model:      e.cfg.Model,
		Messages:   msgs,
		KeepRecent: opts.KeepRecent,
	}
	out, err := compact.Run(ctx, in)
	if err != nil {
		return Summary{}, err
	}
	s := Summary{
		OriginalCount:  out.OriginalCount,
		NewCount:       out.NewCount,
		OriginalTokens: out.OriginalTokens,
		NewTokens:      out.NewTokens,
		SummaryText:    out.SummaryText,
		Skipped:        out.Skipped,
		SkipReason:     out.SkipReason,
		Trigger:        opts.Trigger,
	}
	if !out.Skipped {
		// Install compacted messages directly under lock to avoid a double-copy
		// that SetInitialMessages would introduce.
		// Reset usageSinceLastCompact so the next auto-compact threshold check
		// starts from the post-compact baseline.
		e.mu.Lock()
		e.messages = out.NewMessages
		e.usageSinceLastCompact = message.Usage{}
		e.mu.Unlock()

		if e.cfg.RecordHook != nil {
			e.cfg.RecordHook(session.Record{
				Kind: session.KindCompact,
				Compact: &session.CompactRecord{
					OriginalCount:  s.OriginalCount,
					NewCount:       s.NewCount,
					OriginalTokens: s.OriginalTokens,
					NewTokens:      s.NewTokens,
					Trigger:        s.Trigger,
				},
			})
		}
	}
	return s, nil
}

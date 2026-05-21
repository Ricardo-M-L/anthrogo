# `github.com/ricardo/anthrogo/pkg/query`

```go
package query // import "github.com/ricardo/anthrogo/pkg/query"


TYPES

type CompactOptions struct {
	KeepRecent int    // default 10
	Trigger    string // "manual" | "auto"
}
    CompactOptions controls Engine.Compact behaviour.

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

type Engine struct {
	// Has unexported fields.
}
    Engine owns one conversation. Each SubmitMessage starts a new turn within
    the same conversation; messages, usage, cwd persist across turns.

func NewEngine(cfg Config) *Engine

func (e *Engine) AutoCompactConfig() (threshold, keep int)
    AutoCompactConfig returns the configured auto-compact threshold and
    keep-recent values. A threshold of 0 means auto-compact is disabled.

func (e *Engine) Compact(ctx context.Context, opts CompactOptions) (Summary, error)
    Compact summarizes earlier conversation turns to reduce context size.
    It fires the PreCompact hook, calls compact.Run, then swaps the engine's
    message list and emits a compact record via RecordHook.

    The lock is held only to take a snapshot before the (slow) LLM call and
    to install the result after; the LLM call itself runs without the lock so
    concurrent reads of Messages() or SubmitMessage turns are not blocked for
    the entire summarisation duration.

func (e *Engine) Denials() []PermissionDenial
    Denials returns a copy of all denials recorded during this engine's
    lifetime.

func (e *Engine) EstimatedCost() (float64, bool)
    EstimatedCost returns the estimated USD cost of cumulative session usage,
    or (0, false) if no pricing table is configured or no matching model rate is
    found.

func (e *Engine) IsOverBudget() (bool, float64, float64)
    IsOverBudget reports whether the session's cumulative estimated cost has
    reached or exceeded the configured CostLimitUSD. It returns (over, current,
    limit). When no limit is configured or pricing is unavailable it returns
    (false, 0, 0).

func (e *Engine) LastUsage() message.Usage
    LastUsage returns the most recently observed Usage from the last turn's
    stream.

func (e *Engine) Messages() []message.Message

func (e *Engine) Model() string
    Model returns the model name that was configured for this engine.

func (e *Engine) ResetUsage()
    ResetUsage zeroes the cumulative usage counter (and the since-last-compact
    counter). The last-turn usage is left intact. Tool-budget calculations based
    on Usage() will see zero until the next EventUsage arrives.

func (e *Engine) RunSubagent(ctx context.Context, opts SubagentOptions) (string, error)
    RunSubagent spawns a child Engine for the named subagent type, runs one turn
    with opts.Prompt, drains the stream collecting the last assistant turn's
    text, fires the SubagentStop hook, and returns the result.

func (e *Engine) SetInitialMessages(msgs []message.Message)

func (e *Engine) SubmitMessage(ctx context.Context, prompt string) <-chan Event
    SubmitMessage runs one user turn with a plain-text prompt. It wraps the
    string in a single BlockText and delegates to SubmitMessageBlocks.

func (e *Engine) SubmitMessageBlocks(ctx context.Context, blocks []message.Block) <-chan Event
    SubmitMessageBlocks runs one user turn — including any tool_use sub-turns —
    until the model emits end_turn (or an error/abort). It accepts a pre-built
    slice of content blocks (e.g. from message.ParseUserPrompt) so callers can
    include image blocks alongside text without constructing raw Messages.

func (e *Engine) SystemPrompt() string
    SystemPrompt returns the system prompt that was configured for this engine.

func (e *Engine) Usage() message.Usage

func (e *Engine) UsageSinceLastCompact() message.Usage
    UsageSinceLastCompact returns the cumulative token usage accumulated
    since the session started or since the most recent successful Compact(),
    whichever is later. Updated under lock on every EventUsage; reset by Compact
    when Skipped==false.

type Event struct {
	Kind EventKind

	// assistant_delta / thinking_delta
	Text string

	// tool_use_request / tool_result
	ToolUseID string
	ToolName  string
	ToolInput map[string]any
	IsError   bool

	// usage
	Usage message.Usage

	// error
	Err error

	// turn_complete
	StopReason string
}

type EventKind string
    EventKind discriminates the engine's outgoing event stream. Surfaces (TUI /
    headless) consume this; provider events are internal.

const (
	KindAssistantDelta EventKind = "assistant_delta"
	KindAssistantStop  EventKind = "assistant_stop"
	KindToolUseRequest EventKind = "tool_use_request"
	KindToolResult     EventKind = "tool_result"
	KindPermissionAsk  EventKind = "permission_ask"
	KindTurnComplete   EventKind = "turn_complete"
	KindUsage          EventKind = "usage"
	KindError          EventKind = "error"
)
type HookSink interface {
	FirePostToolUse(ctx context.Context, toolName string, input, response map[string]any) string
	FireStop(ctx context.Context, reason string)
	FirePreCompact(ctx context.Context, trigger string)
	FireSubagentStop(ctx context.Context, reason string)
}
    HookSink is the subset of hooks.Manager that the query engine needs.
    All methods are nil-safe when the field is nil.

type PermissionDenial struct {
	ToolName  string         `json:"tool_name"`
	ToolUseID string         `json:"tool_use_id"`
	ToolInput map[string]any `json:"tool_input"`
}
    PermissionDenial captures one rejected tool invocation.

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
    SubagentOptions carries the parameters for running a subagent.

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
    Summary is the result of Engine.Compact.

type ToolDispatcher func(ctx context.Context, toolUseID, toolName string, input map[string]any) (tool.Result, error)
    ToolDispatcher is a pluggable hook that replaces local tools.Registry
    dispatch. When non-nil in Config, the engine calls it instead of looking
    up and calling the tool from the registry. The caller is responsible for
    applying its own permission gate; the normal permissions.Decide path is
    skipped for this path. This is used by the KAIROS worker to forward tool
    calls back to the client process when exec-tools-locally mode is active.

```

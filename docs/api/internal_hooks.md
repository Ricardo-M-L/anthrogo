# `github.com/ricardo/anthrogo/internal/hooks`

```go
package hooks // import "github.com/ricardo/anthrogo/internal/hooks"


TYPES

type Behavior int
    Behavior describes what the manager decided for a PreToolUse check.

const (
	// DecisionPass means no hook expressed an opinion; proceed normally.
	DecisionPass Behavior = iota
	// DecisionAllow means at least one hook explicitly allowed the action.
	DecisionAllow
	// DecisionDeny means at least one hook denied the action.
	DecisionDeny
)
type Common struct {
	HookEventName EventName `json:"hook_event_name"`
	SessionID     string    `json:"session_id"`
	Cwd           string    `json:"cwd"`
	Version       string    `json:"anthrogo_version"`
}
    Common is the envelope every payload carries.

type Config struct {
	PreToolUse       []Spec `yaml:"PreToolUse,omitempty"`
	PostToolUse      []Spec `yaml:"PostToolUse,omitempty"`
	UserPromptSubmit []Spec `yaml:"UserPromptSubmit,omitempty"`
	Stop             []Spec `yaml:"Stop,omitempty"`
	SubagentStop     []Spec `yaml:"SubagentStop,omitempty"`
	Notification     []Spec `yaml:"Notification,omitempty"`
	PreCompact       []Spec `yaml:"PreCompact,omitempty"`
	SessionStart     []Spec `yaml:"SessionStart,omitempty"`
	SessionEnd       []Spec `yaml:"SessionEnd,omitempty"`
}
    Config holds per-event hook lists. Field names match event names so YAML
    keys are PascalCase (PreToolUse, etc.).

func (c Config) AppendOverlay(overlay Config) Config
    AppendOverlay returns a new Config = c with each event's list extended by
    overlay's list.

func (c *Config) Expand()
    Expand replaces ~/ and $VAR in every Command, fills in default Timeout per
    event.

func (c *Config) Validate() []string
    Validate compiles all matchers; invalid ones drop their spec and append a
    warning.

type Decision struct {
	Behavior      Behavior
	Reason        string
	ModifiedInput map[string]any
}
    Decision is the result of FirePreToolUse.

type EventName string
    EventName is one of the 9 hook event names.

const (
	EventPreToolUse       EventName = "PreToolUse"
	EventPostToolUse      EventName = "PostToolUse"
	EventUserPromptSubmit EventName = "UserPromptSubmit"
	EventStop             EventName = "Stop"
	EventSubagentStop     EventName = "SubagentStop"
	EventNotification     EventName = "Notification"
	EventPreCompact       EventName = "PreCompact"
	EventSessionStart     EventName = "SessionStart"
	EventSessionEnd       EventName = "SessionEnd"
)
type HookSpecificOutput struct {
	HookEventName            EventName      `json:"hookEventName"`
	PermissionDecision       string         `json:"permissionDecision,omitempty"`
	PermissionDecisionReason string         `json:"permissionDecisionReason,omitempty"`
	ModifiedInput            map[string]any `json:"modifiedInput,omitempty"`
	AdditionalContext        string         `json:"additionalContext,omitempty"`
}

type Manager struct {
	// Has unexported fields.
}
    Manager coordinates hook dispatch for all nine events.

func NewManager(cfg Config, opts ManagerOptions) *Manager
    NewManager creates a Manager. cfg should already have been Validate()d.

func (m *Manager) Drain(timeout time.Duration)
    Drain blocks until all in-flight async hook goroutines finish or timeout
    elapses.

func (m *Manager) FireNotification(ctx context.Context, message, kind string)
    FireNotification fires Notification hooks asynchronously.

func (m *Manager) FirePostToolUse(ctx context.Context, toolName string, input map[string]any, response map[string]any) string
    FirePostToolUse runs PostToolUse hooks sequentially and returns concatenated
    additionalContext strings. Errors are logged only, never propagated.

func (m *Manager) FirePreCompact(ctx context.Context, trigger string)
    FirePreCompact runs PreCompact hooks synchronously (log-only per spec §5.2).

func (m *Manager) FirePreToolUse(ctx context.Context, toolName string, input map[string]any) Decision
    FirePreToolUse runs PreToolUse hooks sequentially and returns a Decision.

    Exit semantics:
      - timeout → DecisionDeny, reason "hook X timeout"
      - exit 2 → DecisionDeny, reason = stderr text
      - exit non-zero/non-2 → DecisionDeny, reason "hook X exited N"
      - exit 0, permissionDecision "deny" → DecisionDeny immediately
      - exit 0, permissionDecision "allow" → mark DecisionAllow, keep looping
      - exit 0, modifiedInput present → merge into accumulated ModifiedInput

func (m *Manager) FireSessionEnd(ctx context.Context, kind string)
    FireSessionEnd fires SessionEnd hooks asynchronously.

func (m *Manager) FireSessionStart(ctx context.Context, kind string)
    FireSessionStart fires SessionStart hooks asynchronously.

func (m *Manager) FireStop(ctx context.Context, stopReason string)
    FireStop fires Stop hooks asynchronously.

func (m *Manager) FireSubagentStop(ctx context.Context, stopReason string)
    FireSubagentStop fires SubagentStop hooks asynchronously.

func (m *Manager) FireUserPromptSubmit(ctx context.Context, prompt string) (context.Context, string, bool, string)
    FireUserPromptSubmit runs UserPromptSubmit hooks and returns:

        (ctx, finalPrompt, abort, reason)

    Sequential; any exit-2 or timeout causes abort=true. Exit 0 with
    additionalContext appends it to the prompt.

type ManagerOptions struct {
	SessionID string
	Cwd       string
	Version   string
	// LogSink, if non-nil, is called for informational/error log lines.
	// eventName is the hook event (e.g. "PreToolUse"); msg is free-form text.
	LogSink func(eventName, msg string)
}
    ManagerOptions holds session-level metadata for the Manager.

type NotificationPayload struct {
	Common
	Message string `json:"message"`
	Kind    string `json:"kind"`
}

type Output struct {
	Continue           *bool               `json:"continue,omitempty"`
	StopReason         string              `json:"stopReason,omitempty"`
	HookSpecificOutput *HookSpecificOutput `json:"hookSpecificOutput,omitempty"`
}
    Output is what the hook writes to stdout. All fields optional.

type PostToolUsePayload struct {
	Common
	ToolName     string         `json:"tool_name"`
	ToolInput    map[string]any `json:"tool_input"`
	ToolResponse map[string]any `json:"tool_response"`
}

type PreCompactPayload struct {
	Common
	Trigger string `json:"trigger"`
}

type PreToolUsePayload struct {
	Common
	ToolName  string         `json:"tool_name"`
	ToolInput map[string]any `json:"tool_input"`
}

type Result struct {
	ExitCode int
	Stdout   []byte
	Stderr   []byte
	Output   *Output
	TimedOut bool
}
    Result is what the runner reports back. Output is nil if stdout was empty or
    not parseable JSON.

func RunHook(ctx context.Context, spec Spec, payload any) (*Result, error)
    RunHook spawns `spec.Command`, feeds `payload` (marshaled JSON) to stdin,
    waits up to spec.Timeout. Always returns a Result on completion or timeout;
    returns a non-nil error only on setup failure (e.g. cmd.Start() failure).

type SessionEndPayload struct {
	Common
	Kind string `json:"kind"`
}

type SessionStartPayload struct {
	Common
	Kind string `json:"kind"`
}

type Spec struct {
	Matcher string        `yaml:"matcher,omitempty"`
	Command string        `yaml:"command"`
	Timeout time.Duration `yaml:"timeout,omitempty"`

	// Has unexported fields.
}
    Spec is one hook entry under a given event.

type StopPayload struct {
	Common
	StopReason string `json:"stop_reason"`
}

type UserPromptSubmitPayload struct {
	Common
	Prompt string `json:"prompt"`
}

```

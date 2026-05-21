# `github.com/ricardo/anthrogo/internal/headless`

```go
package headless // import "github.com/ricardo/anthrogo/internal/headless"


FUNCTIONS

func Run(ctx context.Context, opts Options) error
    Run executes one prompt and writes the assistant's final text to Stdout.
    Tool denials in headless mode flow through naturally because Permissions is
    configured with ShouldAvoidPrompts=true at the CLI level (Task 26).

func RunExecRequest(req *command.ExecRequest, stdout io.Writer)
    RunExecRequest executes a command.ExecRequest in headless mode:
    the subprocess inherits stdio so an editor (or any interactive program) can
    paint the terminal directly. OnComplete is called after the process exits,
    and the returned status string (if any) is written to stdout.

    Callers that dispatch slash commands in headless mode should call this after
    checking result.ExecCmd != nil.


TYPES

type Options struct {
	Prompt          string
	Model           string
	SystemPrompt    string
	Cwd             string
	Provider        provider.Provider
	Tools           *tool.Registry
	Permissions     *permissions.Context
	InitialMessages []message.Message
	RecordHook      func(session.Record)
	Session         *session.Store
	Stdout          io.Writer
	Stderr          io.Writer
	Hooks           PromptHookSink
	// HooksConfig is the raw Config that was used to build the Hooks manager.
	// When set, it is forwarded to KAIROS workers via RemoteContext so they can
	// apply the client's hook rules. nil-safe.
	HooksConfig *hooks.Config
	Subagents   *subagent.Registry
	// OnEngineReady, if non-nil, is called with the engine before the first
	// SubmitMessage. Callers use this to wire deferred runners (e.g. Task tool).
	OnEngineReady func(*query.Engine)

	// AutoCompactThreshold and AutoCompactKeepRecent are forwarded to the engine.
	AutoCompactThreshold  int
	AutoCompactKeepRecent int

	// Pricing is the optional pricing table for cost tracking. nil = disabled.
	Pricing *pricing.Table

	// CostLimitUSD, when > 0, enables hard budget enforcement via IsOverBudget().
	CostLimitUSD float64

	// JSON, when true, writes line-delimited JSON events to Stdout instead of
	// plain text. Each engine event becomes one JSON object on its own line.
	JSON bool

	// KairosTrustKey, when non-nil, is forwarded to query.Config and used as the
	// global ed25519 public key for verifying SSE signatures on ALL KAIROS dispatches.
	KairosTrustKey ed25519.PublicKey
}
    Options bundles everything Run needs. The CLI assembles this from cobra
    flags + config.

type PromptHookSink interface {
	FireUserPromptSubmit(ctx context.Context, prompt string) (context.Context, string, bool, string)
	FireSessionStart(ctx context.Context, kind string)
	FireSessionEnd(ctx context.Context, kind string)
	FireNotification(ctx context.Context, message, kind string)
	FirePostToolUse(ctx context.Context, toolName string, input, response map[string]any) string
	FireStop(ctx context.Context, reason string)
	FirePreCompact(ctx context.Context, trigger string)
	FireSubagentStop(ctx context.Context, reason string)
}
    PromptHookSink is the subset of hooks.Manager that headless.Run needs.
    Declared locally to keep import graph flat (no import of internal/hooks).

```

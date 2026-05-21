# `github.com/ricardo/anthrogo/internal/tui`

```go
package tui // import "github.com/ricardo/anthrogo/internal/tui"


FUNCTIONS

func ThemeNames() []string
    ThemeNames returns the list of all built-in theme names.


TYPES

type App struct {
	// Has unexported fields.
}

func New(opts Options) *App

func (a *App) AppendHookLog(event, msg string)
    AppendHookLog routes hook log lines through the bubbletea event loop when
    the program is running. Always pushes to logPane; also pushes to chat in
    single layout (back-compat). When program is nil (tests), mutates directly.

func (a *App) AppendServerLog(server, msg string)
    AppendServerLog routes MCP log lines through the bubbletea event loop when
    the program is running. Always pushes to logPane; also pushes to chat in
    single layout (back-compat). When program is nil (tests), mutates directly.

func (a *App) AppendUIMessage(s string)

func (a *App) ClaudeMd() string

func (a *App) Cwd() string

func (a *App) Engine() *query.Engine

func (a *App) Init() tea.Cmd

func (a *App) MCP() *mcp.Manager

func (a *App) Messages() []message.Message

func (a *App) Permissions() *permissions.Context

func (a *App) Plugins() any

func (a *App) Quit()

func (a *App) Registry() *command.Registry

func (a *App) ReplaceMessages(msgs []message.Message)

func (a *App) RequestPrompt(source string, req tool.PromptRequest) (tool.PromptResponse, error)
    RequestPrompt routes a prompt request through the TUI permission modal.
    It is safe to call from any goroutine (e.g. the MCP elicitation handler).

func (a *App) ResetSession() error

func (a *App) Session() *session.Store

func (a *App) SetProgram(p *tea.Program)
    SetProgram must be called with the tea.Program before Run so that
    AppendServerLog can route through Program.Send instead of mutating chat
    directly from a background goroutine.

func (a *App) SetTheme(name string) error
    SetTheme swaps the active theme by name and propagates it to all
    sub-components. bubbletea will re-render on the next event automatically.

func (a *App) Skills() *skill.Registry

func (a *App) Subagents() *subagent.Registry

func (a *App) ThemeName() string
    ThemeName returns the name of the currently active theme.

func (a *App) Tools() *tool.Registry

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd)

func (a *App) View() string

type Options struct {
	Provider        provider.Provider
	Tools           *tool.Registry
	Permissions     *permissions.Context
	Model           string
	SystemPrompt    string
	Cwd             string
	ClaudeMd        string
	Commands        *command.Registry
	Session         *session.Store
	InitialMessages []message.Message
	RecordHook      func(session.Record)
	MCP             *mcp.Manager
	Hooks           PromptHookSink
	// HooksConfig is the raw hooks.Config used to build the Hooks manager.
	// Forwarded to the engine so KAIROS workers can inherit the client's hook rules.
	HooksConfig *hooks.Config
	Skills      *skill.Registry
	Subagents   *subagent.Registry
	// Plugins is the *plugin.Registry. Typed as any to avoid an import cycle
	// between tui and pkg/plugin (which imports pkg/command which tui uses).
	Plugins any
	// OnEngineReady, if non-nil, is called with the newly-constructed engine
	// before New returns. Callers can use this to wire deferred runners (e.g.
	// the Task tool's runner).
	OnEngineReady func(*query.Engine)

	// AutoCompactThreshold and AutoCompactKeepRecent are forwarded to the engine.
	AutoCompactThreshold  int
	AutoCompactKeepRecent int

	// Pricing is the optional pricing table for cost tracking. nil = disabled.
	Pricing *pricing.Table

	// CostLimitUSD, when > 0, enables hard budget enforcement via IsOverBudget().
	CostLimitUSD float64

	// KairosTrustKey, when non-nil, is the global ed25519 public key for verifying
	// SSE signatures on all KAIROS subagent dispatches.
	KairosTrustKey ed25519.PublicKey

	// Theme, when non-nil, overrides the default dark theme.
	Theme *Theme
}

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
    PromptHookSink is the subset of hooks.Manager the TUI (and engine
    via tui.New) needs. Includes both per-prompt methods and tool-level
    methods so the same concrete value (*hooks.Manager) can be forwarded to
    query.Config.Hooks without a cast. All methods are nil-safe when the Hooks
    field is nil.

type Theme struct {
	Name        string
	UserPrompt  lipgloss.Style
	Assistant   lipgloss.Style
	ToolHeader  lipgloss.Style
	ToolBody    lipgloss.Style
	Error       lipgloss.Style
	StatusLine  lipgloss.Style
	Border      lipgloss.Style
	ModalBorder lipgloss.Style
}

func DarkTheme() Theme

func DefaultTheme() Theme
    DefaultTheme returns the dark theme (back-compat alias).

func LightTheme() Theme

func ThemeByName(name string) (Theme, bool)
    ThemeByName looks up a built-in theme by name (case-insensitive). Empty
    string or "dark" both resolve to the dark theme.

func ThemeFromConfig(name string, overrides map[string]string) Theme
    ThemeFromConfig resolves the active theme from a config name and optional
    per-field hex colour overrides. When name is "custom" and overrides is
    non-empty the dark theme is used as a base and the supplied colours are
    applied on top.

```

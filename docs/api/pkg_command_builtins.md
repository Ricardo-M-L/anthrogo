# `github.com/ricardo/anthrogo/pkg/command/builtins`

```go
package builtins // import "github.com/ricardo/anthrogo/pkg/command/builtins"


TYPES

type AddDir struct{}

func (AddDir) Aliases() []string

func (AddDir) Description() string

func (AddDir) Name() string

func (AddDir) Run(_ context.Context, args string, host command.Host) (command.Result, error)

func (AddDir) Type() command.Type

type Audit struct{}
    Audit implements the /audit builtin command.

func (Audit) Aliases() []string

func (Audit) Description() string

func (Audit) Name() string

func (Audit) Run(ctx context.Context, args string, host command.Host) (command.Result, error)

func (Audit) Type() command.Type

type Clear struct{}

func (Clear) Aliases() []string

func (Clear) Description() string

func (Clear) Name() string

func (Clear) Run(_ context.Context, _ string, host command.Host) (command.Result, error)

func (Clear) Type() command.Type

type Compact struct{}

func (Compact) Aliases() []string

func (Compact) Description() string

func (Compact) Name() string

func (Compact) Run(ctx context.Context, args string, host command.Host) (command.Result, error)

func (Compact) Type() command.Type

type Cost struct{}
    Cost implements the /cost builtin command.

func (Cost) Aliases() []string

func (Cost) Description() string

func (Cost) Name() string

func (Cost) Run(ctx context.Context, args string, host command.Host) (command.Result, error)

func (Cost) Type() command.Type

type Cwd struct{}

func (Cwd) Aliases() []string

func (Cwd) Description() string

func (Cwd) Name() string

func (Cwd) Run(_ context.Context, _ string, host command.Host) (command.Result, error)

func (Cwd) Type() command.Type

type Help struct {
	Reg *command.Registry
}

func (Help) Aliases() []string

func (Help) Description() string

func (Help) Name() string

func (h *Help) Run(_ context.Context, _ string, host command.Host) (command.Result, error)

func (Help) Type() command.Type

type History struct {
	// Path overrides the default ~/.anthrogo/input_history location (useful in tests).
	Path string
}
    History is the /history slash command: list, search, and clear past input
    history.

func (History) Aliases() []string

func (History) Description() string

func (History) Name() string

func (h History) Run(_ context.Context, args string, _ command.Host) (command.Result, error)

func (History) Type() command.Type

type Login struct {
	Config oauth.Config
}
    Login implements /login — runs the M6.5 OAuth 2.1 PKCE flow and saves the
    resulting token to ~/.anthrogo/auth/anthropic.json.

func (Login) Aliases() []string

func (Login) Description() string

func (Login) Name() string

func (l Login) Run(ctx context.Context, args string, host command.Host) (command.Result, error)

func (Login) Type() command.Type

type MCP struct{}

func (MCP) Aliases() []string

func (MCP) Description() string

func (MCP) Name() string

func (MCP) Run(ctx context.Context, args string, host command.Host) (command.Result, error)

func (MCP) Type() command.Type

type Memory struct{}

func (Memory) Aliases() []string

func (Memory) Description() string

func (Memory) Name() string

func (Memory) Run(_ context.Context, _ string, host command.Host) (command.Result, error)

func (Memory) Type() command.Type

type Mode struct{}

func (Mode) Aliases() []string

func (Mode) Description() string

func (Mode) Name() string

func (Mode) Run(_ context.Context, args string, host command.Host) (command.Result, error)

func (Mode) Type() command.Type

type Model struct {
	Available []string
}

func (Model) Aliases() []string

func (Model) Description() string

func (Model) Name() string

func (m Model) Run(_ context.Context, args string, host command.Host) (command.Result, error)

func (Model) Type() command.Type

type Plugin struct {
	HomeRoot string
	CwdRoot  string
}
    Plugin implements the /plugin slash command.

func (Plugin) Aliases() []string

func (Plugin) Description() string

func (Plugin) Name() string

func (p Plugin) Run(_ context.Context, args string, host command.Host) (command.Result, error)

func (Plugin) Type() command.Type

type Resume struct{}

func (Resume) Aliases() []string

func (Resume) Description() string

func (Resume) Name() string

func (Resume) Run(_ context.Context, args string, host command.Host) (command.Result, error)

func (Resume) Type() command.Type

type SessionCache interface {
	Get(path string) ([]session.Record, error)
	Invalidate(path string)
	Clear()
}
    SessionCache is the interface satisfied by both *session.ReplayCache and
    *session.PersistentCache. Either can be assigned to Sessions.ReplayCache.

type Sessions struct {
	ReplayCache SessionCache // optional; nil-safe falls back to direct Replay
}
    Sessions implements the /sessions builtin command.

func (Sessions) Aliases() []string

func (Sessions) Description() string

func (Sessions) Name() string

func (s Sessions) Run(ctx context.Context, args string, host command.Host) (command.Result, error)

func (Sessions) Type() command.Type

type Skills struct {
	HomeRoot string
	CwdRoot  string
}

func (Skills) Aliases() []string

func (Skills) Description() string

func (Skills) Name() string

func (s Skills) Run(ctx context.Context, args string, host command.Host) (command.Result, error)

func (Skills) Type() command.Type

type Subagents struct {
	HomeRoot string
	CwdRoot  string
}
    Subagents is the /subagents slash command. It lists, shows, and reloads
    user-defined subagent types loaded from YAML files.

func (Subagents) Aliases() []string

func (Subagents) Description() string

func (Subagents) Name() string

func (s Subagents) Run(ctx context.Context, args string, host command.Host) (command.Result, error)

func (Subagents) Type() command.Type

type System struct {
	// HomeOverlayPath is the path to the home-level overlay.
	// If empty, derived from $HOME at run time (back-compat).
	HomeOverlayPath string
	// ProjectOverlayPath is the path to the project (cwd-level) overlay.
	// If empty, /system show will not display a project section.
	ProjectOverlayPath string
}
    System implements /system [show | edit [home|project] | reset
    [home|project]].

func (System) Aliases() []string

func (System) Description() string

func (System) Name() string

func (s System) Run(_ context.Context, args string, host command.Host) (command.Result, error)

func (System) Type() command.Type

type Telemetry struct {
	Reporter *telemetry.Reporter
}
    Telemetry implements the /telemetry builtin command.

func (Telemetry) Aliases() []string

func (Telemetry) Description() string

func (Telemetry) Name() string

func (t Telemetry) Run(_ context.Context, args string, _ command.Host) (command.Result, error)

func (Telemetry) Type() command.Type

type Theme struct{}
    Theme implements the /theme builtin command.

func (Theme) Aliases() []string

func (Theme) Description() string

func (Theme) Name() string

func (Theme) Run(_ context.Context, args string, host command.Host) (command.Result, error)

func (Theme) Type() command.Type

type Tools struct{}

func (Tools) Aliases() []string

func (Tools) Description() string

func (Tools) Name() string

func (Tools) Run(ctx context.Context, _ string, host command.Host) (command.Result, error)

func (Tools) Type() command.Type

type Usage struct{}

func (Usage) Aliases() []string

func (Usage) Description() string

func (Usage) Name() string

func (Usage) Run(ctx context.Context, args string, host command.Host) (command.Result, error)

func (Usage) Type() command.Type

type Version struct{}
    Version implements the /version slash command. It prints the running
    binary's version and checks GitHub Releases for a newer tag. Pass "no-check"
    to skip the network round-trip.

func (Version) Aliases() []string

func (Version) Description() string

func (Version) Name() string

func (Version) Run(ctx context.Context, args string, _ command.Host) (command.Result, error)

func (Version) Type() command.Type

```

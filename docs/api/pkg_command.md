# `github.com/ricardo/anthrogo/pkg/command`

```go
package command // import "github.com/ricardo/anthrogo/pkg/command"


TYPES

type Command interface {
	Name() string
	Aliases() []string
	Description() string
	Type() Type
	Run(ctx context.Context, args string, host Host) (Result, error)
}

type ExecRequest struct {
	Cmd        *exec.Cmd
	OnComplete func(err error) string // returns a chat-appendable status message
}
    ExecRequest describes an editor / subprocess launch that the TUI should
    handle via tea.ExecProcess. After the process exits, OnComplete (if non-nil)
    is invoked on the bubbletea goroutine with the exit error (or nil).

type Host interface {
	Engine() *query.Engine
	Permissions() *permissions.Context
	Tools() *tool.Registry
	Session() *session.Store
	Messages() []message.Message
	ReplaceMessages([]message.Message)
	ResetSession() error
	AppendUIMessage(string)
	ClaudeMd() string
	Quit()
	Cwd() string
	Registry() *Registry
	MCP() *mcp.Manager
	Skills() *skill.Registry
	Subagents() *subagent.Registry
	// Plugins returns the *plugin.Registry. Typed as any to break the
	// command ↔ plugin import cycle. Callers in pkg/command/builtins
	// import pkg/plugin directly and type-assert the value.
	Plugins() any
}

type Registry struct {
	// Has unexported fields.
}

func NewRegistry() *Registry

func (r *Registry) All() []Command

func (r *Registry) Fuzzy(q string) []Command

func (r *Registry) Lookup(input string) (Command, bool)

func (r *Registry) Register(c Command)

type Result struct {
	Text       string
	SubmitText string
	// ExecCmd, when non-nil, is a subprocess the surface (TUI) should run via
	// bubbletea's tea.ExecProcess wrapper. Headless surfaces run the *exec.Cmd
	// directly with inherited stdio. Both surfaces call OnComplete after exit.
	ExecCmd *ExecRequest
}

type Type string

const (
	TypeLocal       Type = "local"
	TypeLocalPrompt Type = "local-prompt"
	TypeSubmit      Type = "submit"
)
```

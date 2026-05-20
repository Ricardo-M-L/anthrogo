package command

import (
	"context"
	"os/exec"

	"github.com/ricardo/anthrogo/internal/mcp"
	"github.com/ricardo/anthrogo/internal/session"
	"github.com/ricardo/anthrogo/pkg/message"
	"github.com/ricardo/anthrogo/pkg/permissions"
	"github.com/ricardo/anthrogo/pkg/query"
	"github.com/ricardo/anthrogo/pkg/skill"
	"github.com/ricardo/anthrogo/pkg/subagent"
	"github.com/ricardo/anthrogo/pkg/tool"
)

type Type string

const (
	TypeLocal       Type = "local"
	TypeLocalPrompt Type = "local-prompt"
	TypeSubmit      Type = "submit"
)

type Command interface {
	Name() string
	Aliases() []string
	Description() string
	Type() Type
	Run(ctx context.Context, args string, host Host) (Result, error)
}

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

// ExecRequest describes an editor / subprocess launch that the TUI should
// handle via tea.ExecProcess. After the process exits, OnComplete (if non-nil)
// is invoked on the bubbletea goroutine with the exit error (or nil).
type ExecRequest struct {
	Cmd        *exec.Cmd
	OnComplete func(err error) string // returns a chat-appendable status message
}

type Result struct {
	Text       string
	SubmitText string
	// ExecCmd, when non-nil, is a subprocess the surface (TUI) should run via
	// bubbletea's tea.ExecProcess wrapper. Headless surfaces run the *exec.Cmd
	// directly with inherited stdio. Both surfaces call OnComplete after exit.
	ExecCmd *ExecRequest
}

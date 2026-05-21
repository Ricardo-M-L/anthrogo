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

// AgentTask describes a subagent dispatch that a builtin command wants the
// engine to perform on its behalf. The surface (TUI or headless) detects a
// non-nil AgentTask in the Result and calls engine.RunSubagent with the
// provided parameters before rendering the final text.
type AgentTask struct {
	// Description is a short human-readable label shown in the TUI prefix.
	Description string
	// Prompt is the full self-contained prompt for the subagent.
	Prompt string
	// SubagentType references a registered subagent.Spec by name.
	SubagentType string
	// ToolAllowlist, when non-empty, overrides the spec's tool_allowlist for
	// this particular invocation. If the named spec already has an allowlist,
	// the per-call list wins.
	ToolAllowlist []string
}

type Result struct {
	Text       string
	SubmitText string
	// ExecCmd, when non-nil, is a subprocess the surface (TUI) should run via
	// bubbletea's tea.ExecProcess wrapper. Headless surfaces run the *exec.Cmd
	// directly with inherited stdio. Both surfaces call OnComplete after exit.
	ExecCmd *ExecRequest
	// AgentTask, when non-nil, instructs the surface to dispatch a subagent
	// via engine.RunSubagent after the command returns. The subagent's final
	// text replaces Result.Text in the rendered output.
	AgentTask *AgentTask
}

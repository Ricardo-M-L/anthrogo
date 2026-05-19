package command

import (
	"context"

	"github.com/ricardo/anthrogo/internal/mcp"
	"github.com/ricardo/anthrogo/internal/session"
	"github.com/ricardo/anthrogo/pkg/message"
	"github.com/ricardo/anthrogo/pkg/permissions"
	"github.com/ricardo/anthrogo/pkg/query"
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
}

type Result struct {
	Text       string
	SubmitText string
}

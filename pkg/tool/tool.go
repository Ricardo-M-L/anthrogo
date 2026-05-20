package tool

import (
	"context"

	"github.com/ricardo/anthrogo/pkg/message"
	"github.com/ricardo/anthrogo/pkg/permissions"
)

// Tool is the M1 surface. Smaller than upstream's src/Tool.ts; expanded in
// M3 (MCP), M4 (hooks/skills), M5 (subagents).
type Tool interface {
	Name() string
	Description(ctx context.Context) string
	Schema() map[string]any
	UserFacingName(input map[string]any) string
	IsReadOnly() bool
	IsConcurrencySafe() bool
	Permission(ctx context.Context, input map[string]any) permissions.Decision
	Call(ctx context.Context, input map[string]any, tcx *Context) (Result, error)
}

// DefaultPermission satisfies the Permission method by deferring to the gate
// (returns BehaviorAsk). Embed it in tools that don't have tool-specific gating.
type DefaultPermission struct{}

func (DefaultPermission) Permission(context.Context, map[string]any) permissions.Decision {
	return permissions.Decision{Behavior: permissions.BehaviorAsk}
}

// PromptKind discriminates the modal variants.
type PromptKind string

const (
	PromptToolPermission PromptKind = "tool_permission"
	PromptQuestion       PromptKind = "question"
	PromptElicitForm     PromptKind = "elicit_form" // M6.3: MCP elicitation form
)

// PromptOption is one selectable answer for a PromptQuestion.
type PromptOption struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

// PromptRequest is what a tool (or the engine) asks the surface to display.
type PromptRequest struct {
	Kind         PromptKind
	ToolName     string
	ToolInput    map[string]any
	InputSummary string
	Question     string
	Options      []PromptOption
	// PromptElicitForm fields
	Message string         // server-supplied prompt text
	Schema  map[string]any // JSON schema the server provided
}

// PromptResponse is the user's reply.
type PromptResponse struct {
	Allow         bool
	Remember      bool   // upgrade to AlwaysAllow rule
	Reason        string
	SelectedLabel string // set for PromptQuestion
	Notes         string // optional free-text from user
	// PromptElicitForm fields
	Action   string         // "accept" | "decline" | "cancel"
	FormData map[string]any // parsed JSON submitted by user (when Action=="accept")
}

// Context flows through a turn — engine builds it once per turn, hands it to
// every Tool.Call. Surface (TUI / headless) injects callbacks.
type Context struct {
	Cwd         string
	Messages    []message.Message
	Permissions *permissions.Context
	AbortContext context.Context

	// SubagentPrefixChain carries the outer Task description chain for nested
	// subagent runs. Each entry is an ancestor Task's description. The inner
	// Task prepends these to build the full "[Task: outer → inner]" prefix.
	SubagentPrefixChain []string

	// Surface-injected; nil-safe.
	RequestPrompt   func(source string, req PromptRequest) (PromptResponse, error)
	AppendUIMessage func(msg string)
	SendOSNotify    func(msg, kind string)
	SetToolProgress func(toolUseID string, p any)
}

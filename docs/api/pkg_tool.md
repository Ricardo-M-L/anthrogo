# `github.com/ricardo/anthrogo/pkg/tool`

```go
package tool // import "github.com/ricardo/anthrogo/pkg/tool"


FUNCTIONS

func MCPToolName(server, tool string) string
    MCPToolName composes an MCP tool name from a server and tool name, fitting
    the Anthropic 64-char limit. When either piece overflows its per-piece
    budget, both are truncated and an 8-char sha256 hex suffix is appended for
    collision resistance.


TYPES

type AskUserQuestion struct{ DefaultPermission }

func (AskUserQuestion) Call(_ context.Context, input map[string]any, tcx *Context) (Result, error)

func (AskUserQuestion) Description(context.Context) string

func (AskUserQuestion) IsConcurrencySafe() bool

func (AskUserQuestion) IsReadOnly() bool

func (AskUserQuestion) Name() string

func (AskUserQuestion) Schema() map[string]any

func (AskUserQuestion) UserFacingName(map[string]any) string

type BackgroundCancel struct {
	DefaultPermission
	Manager *bgtasks.Manager
}
    BackgroundCancel cancels a running background task.

func (b *BackgroundCancel) Call(_ context.Context, input map[string]any, _ *Context) (Result, error)

func (*BackgroundCancel) Description(context.Context) string

func (*BackgroundCancel) IsConcurrencySafe() bool

func (*BackgroundCancel) IsReadOnly() bool

func (*BackgroundCancel) Name() string

func (*BackgroundCancel) Schema() map[string]any

func (*BackgroundCancel) UserFacingName(_ map[string]any) string

type BackgroundLaunch struct {
	DefaultPermission
	Manager *bgtasks.Manager
}
    BackgroundLaunch launches a shell command as a background task.

func (b *BackgroundLaunch) Call(_ context.Context, input map[string]any, _ *Context) (Result, error)

func (*BackgroundLaunch) Description(context.Context) string

func (*BackgroundLaunch) IsConcurrencySafe() bool

func (*BackgroundLaunch) IsReadOnly() bool

func (*BackgroundLaunch) Name() string

func (*BackgroundLaunch) Schema() map[string]any

func (*BackgroundLaunch) UserFacingName(input map[string]any) string

type BackgroundOutput struct {
	DefaultPermission
	Manager *bgtasks.Manager
}
    BackgroundOutput fetches captured stdout and stderr for a task.

func (b *BackgroundOutput) Call(_ context.Context, input map[string]any, _ *Context) (Result, error)

func (*BackgroundOutput) Description(context.Context) string

func (*BackgroundOutput) IsConcurrencySafe() bool

func (*BackgroundOutput) IsReadOnly() bool

func (*BackgroundOutput) Name() string

func (*BackgroundOutput) Schema() map[string]any

func (*BackgroundOutput) UserFacingName(_ map[string]any) string

type BackgroundStatus struct {
	DefaultPermission
	Manager *bgtasks.Manager
}
    BackgroundStatus returns status of one task or lists all tasks.

func (b *BackgroundStatus) Call(_ context.Context, input map[string]any, _ *Context) (Result, error)

func (*BackgroundStatus) Description(context.Context) string

func (*BackgroundStatus) IsConcurrencySafe() bool

func (*BackgroundStatus) IsReadOnly() bool

func (*BackgroundStatus) Name() string

func (*BackgroundStatus) Schema() map[string]any

func (*BackgroundStatus) UserFacingName(_ map[string]any) string

type Bash struct{ DefaultPermission }
    Bash executes a shell command. M1 has no sandbox, no AST security,
    no background tasks — those are deferred to M5 alongside the upstream
    bash{Permissions,Security,Sandbox} modules.

    M10.2 adds opt-in lightweight sandboxing via sandbox:true (see Schema).

func (Bash) Call(parent context.Context, input map[string]any, tcx *Context) (Result, error)

func (Bash) Description(context.Context) string

func (Bash) IsConcurrencySafe() bool

func (Bash) IsReadOnly() bool

func (Bash) Name() string

func (Bash) Schema() map[string]any

func (Bash) UserFacingName(input map[string]any) string

type ContainerExec struct{ DefaultPermission }
    ContainerExec runs a command inside a docker or podman container. M10.7:
    provides real OS-level isolation vs M10.2 Bash sandbox's env scrubbing.
    M11.7: adds pull_policy, gpu, user, workdir, and separate stdout/stderr
    capture.

func (*ContainerExec) Call(ctx context.Context, input map[string]any, tcx *Context) (Result, error)

func (ContainerExec) Description(context.Context) string

func (ContainerExec) IsConcurrencySafe() bool

func (ContainerExec) IsReadOnly() bool

func (ContainerExec) Name() string

func (ContainerExec) Schema() map[string]any

func (ContainerExec) UserFacingName(input map[string]any) string

type Context struct {
	Cwd          string
	Messages     []message.Message
	Permissions  *permissions.Context
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
    Context flows through a turn — engine builds it once per turn, hands it to
    every Tool.Call. Surface (TUI / headless) injects callbacks.

type DefaultPermission struct{}
    DefaultPermission satisfies the Permission method by deferring to the gate
    (returns BehaviorAsk). Embed it in tools that don't have tool-specific
    gating.

func (DefaultPermission) Permission(context.Context, map[string]any) permissions.Decision

type Diff struct{ DefaultPermission }
    Diff wraps `git diff` for the working tree or staged area.

func (Diff) Call(_ context.Context, input map[string]any, tcx *Context) (Result, error)

func (Diff) Description(context.Context) string

func (Diff) IsConcurrencySafe() bool

func (Diff) IsReadOnly() bool

func (Diff) Name() string

func (Diff) Schema() map[string]any

func (Diff) UserFacingName(input map[string]any) string

type Edit struct{ DefaultPermission }

func (Edit) Call(_ context.Context, input map[string]any, _ *Context) (Result, error)

func (Edit) Description(context.Context) string

func (Edit) IsConcurrencySafe() bool

func (Edit) IsReadOnly() bool

func (Edit) Name() string

func (Edit) Schema() map[string]any

func (Edit) UserFacingName(input map[string]any) string

type EnterPlanMode struct{ DefaultPermission }

func (EnterPlanMode) Call(_ context.Context, _ map[string]any, tcx *Context) (Result, error)

func (EnterPlanMode) Description(context.Context) string

func (EnterPlanMode) IsConcurrencySafe() bool

func (EnterPlanMode) IsReadOnly() bool

func (EnterPlanMode) Name() string

func (EnterPlanMode) Schema() map[string]any

func (EnterPlanMode) UserFacingName(map[string]any) string

type ExitPlanMode struct{ DefaultPermission }

func (ExitPlanMode) Call(_ context.Context, _ map[string]any, tcx *Context) (Result, error)

func (ExitPlanMode) Description(context.Context) string

func (ExitPlanMode) IsConcurrencySafe() bool

func (ExitPlanMode) IsReadOnly() bool

func (ExitPlanMode) Name() string

func (ExitPlanMode) Schema() map[string]any

func (ExitPlanMode) UserFacingName(map[string]any) string

type Format struct{ DefaultPermission }
    Format formats one or more source files using the language-appropriate
    formatter.

func (Format) Call(_ context.Context, input map[string]any, tcx *Context) (Result, error)

func (Format) Description(context.Context) string

func (Format) IsConcurrencySafe() bool

func (Format) IsReadOnly() bool

func (Format) Name() string

func (Format) Schema() map[string]any

func (Format) UserFacingName(input map[string]any) string

type Git struct{ DefaultPermission }
    Git exposes a read-only subset of git subcommands.

func (Git) Call(_ context.Context, input map[string]any, tcx *Context) (Result, error)

func (Git) Description(context.Context) string

func (Git) IsConcurrencySafe() bool

func (Git) IsReadOnly() bool

func (Git) Name() string

func (Git) Schema() map[string]any

func (Git) UserFacingName(input map[string]any) string

type Glob struct{ DefaultPermission }

func (Glob) Call(_ context.Context, input map[string]any, tcx *Context) (Result, error)

func (Glob) Description(context.Context) string

func (Glob) IsConcurrencySafe() bool

func (Glob) IsReadOnly() bool

func (Glob) Name() string

func (Glob) Schema() map[string]any

func (Glob) UserFacingName(input map[string]any) string

type Grep struct{ DefaultPermission }

func (Grep) Call(ctx context.Context, input map[string]any, tcx *Context) (Result, error)

func (Grep) Description(context.Context) string

func (Grep) IsConcurrencySafe() bool

func (Grep) IsReadOnly() bool

func (Grep) Name() string

func (Grep) Schema() map[string]any

func (Grep) UserFacingName(input map[string]any) string

type HTTPRequest struct{ DefaultPermission }
    HTTPRequest is a general-purpose HTTP client tool (curl-like). Unlike
    WebFetch (GET-only + HTML→markdown), HTTPRequest supports all common
    HTTP verbs, raw body in/out, configurable timeout and response size cap,
    and optional file-save of the response body.

func (*HTTPRequest) Call(ctx context.Context, input map[string]any, _ *Context) (Result, error)

func (*HTTPRequest) Description(context.Context) string

func (*HTTPRequest) IsConcurrencySafe() bool

func (*HTTPRequest) IsReadOnly() bool
    IsReadOnly returns false conservatively; the engine treats per-call.

func (*HTTPRequest) Name() string

func (*HTTPRequest) Schema() map[string]any

func (*HTTPRequest) UserFacingName(input map[string]any) string

type MCPAdapter struct {
	DefaultPermission

	// Has unexported fields.
}
    MCPAdapter wraps one MCP server's tool descriptor as a tool.Tool.

func NewMCPAdapter(serverName string, descriptor *sdk.Tool, invoker MCPInvoker) *MCPAdapter

func (a *MCPAdapter) Call(ctx context.Context, input map[string]any, _ *Context) (Result, error)

func (a *MCPAdapter) Description(_ context.Context) string

func (a *MCPAdapter) IsConcurrencySafe() bool

func (a *MCPAdapter) IsReadOnly() bool

func (a *MCPAdapter) Name() string

func (a *MCPAdapter) Schema() map[string]any

func (a *MCPAdapter) UserFacingName(map[string]any) string

type MCPInvoker func(ctx context.Context, args map[string]any) (*sdk.CallToolResult, error)
    MCPInvoker is the callback the manager hands the adapter so the adapter
    doesn't import internal/mcp.

type MCPResource struct {
	DefaultPermission
	// Has unexported fields.
}
    MCPResource is a built-in tool for listing or reading MCP server resources.

func NewMCPResource(m MCPResourceManager) *MCPResource
    NewMCPResource constructs an MCPResource tool backed by the given manager.

func (t *MCPResource) Call(ctx context.Context, input map[string]any, _ *Context) (Result, error)

func (*MCPResource) Description(context.Context) string

func (*MCPResource) IsConcurrencySafe() bool

func (*MCPResource) IsReadOnly() bool

func (*MCPResource) Name() string

func (*MCPResource) Schema() map[string]any

func (*MCPResource) UserFacingName(input map[string]any) string

type MCPResourceManager interface {
	AllResources(ctx context.Context) map[string][]*sdk.Resource
	ReadResource(ctx context.Context, server, uri string) (*sdk.ReadResourceResult, error)
}
    MCPResourceManager is the slice of internal/mcp.Manager that this tool
    needs. Declared here so pkg/tool doesn't import internal/mcp.

type NotebookEdit struct{ DefaultPermission }

func (NotebookEdit) Call(_ context.Context, input map[string]any, _ *Context) (Result, error)

func (NotebookEdit) Description(context.Context) string

func (NotebookEdit) IsConcurrencySafe() bool

func (NotebookEdit) IsReadOnly() bool

func (NotebookEdit) Name() string

func (NotebookEdit) Schema() map[string]any

func (NotebookEdit) UserFacingName(input map[string]any) string

type PromptKind string
    PromptKind discriminates the modal variants.

const (
	PromptToolPermission PromptKind = "tool_permission"
	PromptQuestion       PromptKind = "question"
	PromptElicitForm     PromptKind = "elicit_form" // M6.3: MCP elicitation form
)
type PromptOption struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}
    PromptOption is one selectable answer for a PromptQuestion.

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
    PromptRequest is what a tool (or the engine) asks the surface to display.

type PromptResponse struct {
	Allow         bool
	Remember      bool // upgrade to AlwaysAllow rule
	Reason        string
	SelectedLabel string // set for PromptQuestion
	Notes         string // optional free-text from user
	// PromptElicitForm fields
	Action   string         // "accept" | "decline" | "cancel"
	FormData map[string]any // parsed JSON submitted by user (when Action=="accept")
}
    PromptResponse is the user's reply.

type Read struct{ DefaultPermission }
    Read is the M1 file reader. Mirrors src/tools/FileReadTool but without
    notebook/image branches (deferred to M2/M3).

func (Read) Call(ctx context.Context, input map[string]any, _ *Context) (Result, error)

func (Read) Description(context.Context) string

func (Read) IsConcurrencySafe() bool

func (Read) IsReadOnly() bool

func (Read) Name() string

func (Read) Schema() map[string]any

func (Read) UserFacingName(input map[string]any) string

type References struct{ DefaultPermission }
    References finds all usages (word-boundary occurrences) of a symbol name
    across the directory tree.

func (References) Call(ctx context.Context, input map[string]any, tcx *Context) (Result, error)

func (References) Description(context.Context) string

func (References) IsConcurrencySafe() bool

func (References) IsReadOnly() bool

func (References) Name() string

func (References) Schema() map[string]any

func (References) UserFacingName(input map[string]any) string

type Registry struct {
	// Has unexported fields.
}
    Registry stores Tools in registration order so the API-side tool list is
    stable run-to-run (important for prompt caching).

func NewRegistry() *Registry

func (r *Registry) All() []Tool

func (r *Registry) Get(name string) (Tool, bool)

func (r *Registry) Register(t Tool)

func (r *Registry) RemoveByPrefix(prefix string) int
    RemoveByPrefix unregisters every tool whose Name() starts with prefix.
    Returns the number removed.

type Result struct {
	Type    ResultType
	Text    string               // canonical payload
	Image   *message.ImageSource // for ResultImage
	ForLLM  string               // what the model sees in tool_result
	ForUser string               // optional user-facing summary
	Data    map[string]any       // structured fields the TUI can read
	IsError bool
}
    Result is what a Tool.Call returns. The TUI consumes ForUser; the engine
    pushes ForLLM (or a fallback to Text) back to the model as a tool_result
    block.

func (r Result) ModelText() string
    ModelText returns the string to send back to the model. Falls back to Text
    when ForLLM is empty.

type ResultType string
    ResultType discriminates the payload variant.

const (
	ResultText       ResultType = "text"
	ResultImage      ResultType = "image"
	ResultStructured ResultType = "structured"
)
type SQLQuery struct{ DefaultPermission }
    SQLQuery runs a SQL query against postgres, mysql, or sqlite.
    SELECT/EXPLAIN/SHOW/DESCRIBE auto-allow; mutating queries go through the
    gate.

func (*SQLQuery) Call(ctx context.Context, input map[string]any, _ *Context) (Result, error)

func (*SQLQuery) Description(context.Context) string

func (*SQLQuery) IsConcurrencySafe() bool

func (*SQLQuery) IsReadOnly() bool

func (*SQLQuery) Name() string

func (*SQLQuery) Permission(_ context.Context, input map[string]any) permissions.Decision

func (*SQLQuery) Schema() map[string]any

func (*SQLQuery) UserFacingName(input map[string]any) string

type Skill struct {
	DefaultPermission
	// Has unexported fields.
}
    Skill is a model-invoked tool that returns the body of a registered skill.

func NewSkill(r *skill.Registry) *Skill

func (s *Skill) Call(ctx context.Context, input map[string]any, _ *Context) (Result, error)

func (*Skill) Description(context.Context) string

func (*Skill) IsConcurrencySafe() bool

func (*Skill) IsReadOnly() bool

func (*Skill) Name() string

func (*Skill) Schema() map[string]any

func (*Skill) UserFacingName(input map[string]any) string

type SpeechToText struct{ DefaultPermission }
    SpeechToText transcribes an audio file using the whisper CLI.

func (*SpeechToText) Call(ctx context.Context, input map[string]any, _ *Context) (Result, error)

func (*SpeechToText) Description(context.Context) string

func (*SpeechToText) IsConcurrencySafe() bool

func (*SpeechToText) IsReadOnly() bool

func (*SpeechToText) Name() string

func (*SpeechToText) Schema() map[string]any

func (*SpeechToText) UserFacingName(input map[string]any) string

type SymbolSearch struct{ DefaultPermission }
    SymbolSearch finds a symbol's definition site by name. Go files are parsed
    via go/parser; other languages use regex heuristics.

func (SymbolSearch) Call(ctx context.Context, input map[string]any, tcx *Context) (Result, error)

func (SymbolSearch) Description(context.Context) string

func (SymbolSearch) IsConcurrencySafe() bool

func (SymbolSearch) IsReadOnly() bool

func (SymbolSearch) Name() string

func (SymbolSearch) Schema() map[string]any

func (SymbolSearch) UserFacingName(input map[string]any) string

type Task struct {
	DefaultPermission

	// Has unexported fields.
}
    Task is a built-in tool that spawns a subagent engine to perform a
    self-contained multi-step task and returns its final assistant text.

func NewTask(reg *subagent.Registry, runner func(context.Context, TaskOptions) (string, error)) *Task
    NewTask creates a Task tool wired with the given subagent registry and
    runner. The runner is typically engine.RunSubagent, injected to avoid an
    import cycle.

func (t *Task) Call(ctx context.Context, input map[string]any, tcx *Context) (Result, error)
    Call invokes the runner with the parsed inputs.

func (t *Task) Description(_ context.Context) string
    Description lists available subagent types so the model knows what to pass.

func (*Task) IsConcurrencySafe() bool
    IsConcurrencySafe returns true — multiple Task tool_use blocks in one
    assistant turn run concurrently via the engine's parallel dispatch path.

func (*Task) IsReadOnly() bool
    IsReadOnly returns false — a subagent can call write tools.

func (*Task) Name() string
    Name implements Tool.

func (*Task) Schema() map[string]any
    Schema returns the JSON schema for Task inputs.

func (*Task) UserFacingName(input map[string]any) string
    UserFacingName returns "Task: <description>" for display in the TUI.

type TaskOptions struct {
	Description  string
	Prompt       string
	SubagentType string
	// OnDelta, if non-nil, is invoked for each text delta from the subagent stream.
	// Called from a background goroutine; implementations must be thread-safe.
	OnDelta func(string)
	// PrefixChain carries ancestor Task descriptions for nested prefix display.
	// Passed through to RunSubagent as SubagentOptions.PrefixChain.
	PrefixChain []string
}
    TaskOptions carries the parsed input for a Task tool call.

type TextToSpeech struct{ DefaultPermission }
    TextToSpeech synthesizes speech from text using a platform system
    synthesizer.

func (*TextToSpeech) Call(ctx context.Context, input map[string]any, _ *Context) (Result, error)

func (*TextToSpeech) Description(context.Context) string

func (*TextToSpeech) IsConcurrencySafe() bool

func (*TextToSpeech) IsReadOnly() bool

func (*TextToSpeech) Name() string

func (*TextToSpeech) Schema() map[string]any

func (*TextToSpeech) UserFacingName(input map[string]any) string

type Todo struct {
	Content    string `json:"content"`
	Status     string `json:"status"` // pending | in_progress | completed
	ActiveForm string `json:"activeForm,omitempty"`
}
    Todo mirrors the upstream TodoWriteTool item shape.

type TodoWrite struct {
	DefaultPermission

	// Has unexported fields.
}
    TodoWrite replaces the tool's internal list on each call. M1 keeps a single
    session-scoped list; M2 will scope per-session-id when sessions persist.

func (t *TodoWrite) Call(_ context.Context, input map[string]any, _ *Context) (Result, error)

func (*TodoWrite) Description(context.Context) string

func (*TodoWrite) IsConcurrencySafe() bool

func (*TodoWrite) IsReadOnly() bool

func (t *TodoWrite) List() []Todo

func (*TodoWrite) Name() string

func (*TodoWrite) Schema() map[string]any

func (*TodoWrite) UserFacingName(map[string]any) string

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
    Tool is the M1 surface. Smaller than upstream's src/Tool.ts; expanded in M3
    (MCP), M4 (hooks/skills), M5 (subagents).

type WebFetch struct {
	DefaultPermission

	// Has unexported fields.
}

func NewWebFetch() *WebFetch

func (w *WebFetch) Call(ctx context.Context, input map[string]any, _ *Context) (Result, error)

func (*WebFetch) Description(context.Context) string

func (*WebFetch) IsConcurrencySafe() bool

func (*WebFetch) IsReadOnly() bool

func (*WebFetch) Name() string

func (*WebFetch) Schema() map[string]any

func (*WebFetch) UserFacingName(input map[string]any) string

type WebSearch struct {
	DefaultPermission
	// Has unexported fields.
}
    WebSearch is the tool that queries a search provider.

func NewWebSearch(cfg WebSearchConfig) *WebSearch
    NewWebSearch creates a WebSearch with the given config.

func (w *WebSearch) Call(ctx context.Context, input map[string]any, _ *Context) (Result, error)
    Call dispatches to the configured backend and returns JSON results.

func (*WebSearch) Description(context.Context) string

func (*WebSearch) IsConcurrencySafe() bool

func (*WebSearch) IsReadOnly() bool

func (*WebSearch) Name() string

func (*WebSearch) Schema() map[string]any

func (*WebSearch) UserFacingName(i map[string]any) string

type WebSearchConfig struct {
	Backend  string
	APIKey   string
	Endpoint string // backend-specific: CSE ID for google, base URL for bing/tavily
	URL      string // optional full URL override (for testing or self-hosted)
}
    WebSearchConfig holds configuration for the WebSearch tool.

type Write struct{ DefaultPermission }

func (Write) Call(_ context.Context, input map[string]any, _ *Context) (Result, error)

func (Write) Description(context.Context) string

func (Write) IsConcurrencySafe() bool

func (Write) IsReadOnly() bool

func (Write) Name() string

func (Write) Schema() map[string]any

func (Write) UserFacingName(input map[string]any) string

```

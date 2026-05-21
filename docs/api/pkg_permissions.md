# `github.com/ricardo/anthrogo/pkg/permissions`

```go
package permissions // import "github.com/ricardo/anthrogo/pkg/permissions"


FUNCTIONS

func IsReadOnlyBashCommand(cmd string) bool
    IsReadOnlyBashCommand checks whether a bash command starts with one
    of the allowlisted read-only prefixes and contains no redirect / pipe
    metacharacters.

func IsWriteTool(name string) bool
    IsWriteTool reports whether a tool name is in the plan-mode write list.
    All mcp__* tools are treated as write tools.


TYPES

type AdditionalDir struct {
	Path string
}
    AdditionalDir is a working directory that's been opted into via --add-dir.

type Behavior string
    Behavior is the gate's verdict.

const (
	BehaviorAllow Behavior = "allow"
	BehaviorDeny  Behavior = "deny"
	BehaviorAsk   Behavior = "ask"
)
type Context struct {
	Mode                         Mode
	AdditionalWorkingDirectories map[string]AdditionalDir
	AlwaysAllowRules             RulesBySource
	AlwaysDenyRules              RulesBySource
	AlwaysAskRules               RulesBySource
	IsBypassAvailable            bool
	ShouldAvoidPrompts           bool
	PrePlanMode                  Mode

	// HookDecide is consulted by Decide before any rule lookup. nil-safe.
	HookDecide func(toolName string, input map[string]any) HookOutcome
}
    Context holds permission state for one conversation. Mirrors
    ToolPermissionContext (src/Tool.ts:123).

func Empty() *Context
    Empty returns a Context with all rule maps initialised, in default mode.

func (c *Context) Clone() *Context
    Clone returns a shallow copy of c suitable for handing to a subagent.
    RulesBySource maps and HookDecide are shared by reference (treated as
    immutable). Mode, PrePlanMode, ShouldAvoidPrompts, and IsBypassAvailable are
    copied so subagent toggles don't affect the parent.

type Decision struct {
	Behavior      Behavior
	Reason        string
	ModifiedInput map[string]any
	SuggestedRule *Rule
}
    Decision is the gate's output for one tool invocation.

func Decide(c *Context, tool string, input map[string]any) Decision
    Decide evaluates a tool invocation against the context's rules.

    Order (mirrors src/utils/permissions evaluation):
     1. bypassPermissions mode → allow always.
     2. deny rules win unconditionally.
     3. acceptEdits mode allows Write/Edit/NotebookEdit by default.
     4. allow rules → allow.
     5. ask rules → ask (unless ShouldAvoidPrompts).
     6. fallback → ask (default mode) / deny (ShouldAvoidPrompts).

type HookOutcome struct {
	Pass          bool
	Allow         bool
	Deny          bool
	Reason        string
	ModifiedInput map[string]any
}
    HookOutcome is the result returned by HookDecide.

type Mode string
    Mode controls how the gate evaluates tool calls. Mirrors
    src/types/permissions.ts PermissionMode.

const (
	ModeDefault           Mode = "default"
	ModePlan              Mode = "plan"
	ModeAcceptEdits       Mode = "acceptEdits"
	ModeBypassPermissions Mode = "bypassPermissions"
)
type Rule struct {
	Tool    string `yaml:"tool" json:"tool"`
	Pattern string `yaml:"match,omitempty" json:"match,omitempty"`
	Source  Source `yaml:"-" json:"-"`
}
    Rule represents one allow/deny/ask entry.

        {tool: "Bash", match: "git status*"}
        {tool: "Read", match: "/tmp/**"}
        {tool: "Read"}  // no match → applies to any input for this tool

func (r Rule) Match(tool string, input map[string]any) bool
    Match returns true if this rule applies to a given tool invocation. Matching
    strategy (M1):
      - Tool name must match exactly.
      - If Match is empty, the rule applies to any input.
      - If the input has a "path" or "file_path" field, doublestar-glob it.
      - If the input has a "command" field, prefix-match (`*` = trailing
        wildcard).
      - Anything else falls through to false (conservative).

type RulesBySource map[Source][]Rule
    RulesBySource groups rules by where they were loaded from.

type Source string
    Source labels where the rule came from (settings.json scope analogue).

const (
	SourceUser    Source = "user"
	SourceProject Source = "project"
	SourceManaged Source = "managed"
	SourceCLI     Source = "cli"
)
```

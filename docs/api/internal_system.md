# `github.com/ricardo/anthrogo/internal/system`

```go
package system // import "github.com/ricardo/anthrogo/internal/system"


FUNCTIONS

func BuildSystemPrompt(opts Options) string
    BuildSystemPrompt produces the prompt sent in `system` to the API. It's the
    Go analogue of upstream's `fetchSystemPromptParts` + `getSystemContext` +
    `getUserContext` joined.

func GitStatusSnapshot(cwd string) (string, error)
func LoadClaudeMd(start, stopAt string) (string, error)
    LoadClaudeMd walks from `start` upward, stopping at (and including)
    `stopAt`. Returns the concatenation of every CLAUDE.md found, root-first,
    separated by a header that names the source file.

    `stopAt` is typically the user's home directory; pass an empty string to
    walk all the way to the filesystem root.


TYPES

type Options struct {
	ToolNames    []string
	ClaudeMd     string
	GitStatus    string
	CurrentDate  string
	Cwd          string
	PlanModeOn   bool
	Skills       []skill.Skill
	Subagents    []subagent.Spec
	MCPResources map[string][]*sdk.Resource
	UserOverlay  string // raw text appended verbatim at end of system prompt (or empty)
}
    Options are everything BuildSystemPrompt needs. The CLI / TUI gathers these
    (via LoadClaudeMd, GitStatusSnapshot, etc.) and hands them off.

```

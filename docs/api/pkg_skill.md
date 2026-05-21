# `github.com/ricardo/anthrogo/pkg/skill`

```go
package skill // import "github.com/ricardo/anthrogo/pkg/skill"


FUNCTIONS

func SplitFrontmatter(raw []byte) (frontmatterBytes, body []byte, ok bool)
    SplitFrontmatter separates the leading "---\n...\n---\n" block.


TYPES

type Registry struct {
	// Has unexported fields.
}

func NewRegistry(list []Skill) *Registry

func (r *Registry) Add(s Skill) bool
    Add inserts s into the registry if no skill with that name exists. Returns
    true if added, false if a same-named skill was already present (in which
    case s is discarded).

func (r *Registry) Get(name string) (Skill, bool)

func (r *Registry) List() []Skill

func (r *Registry) Reload(homeRoot, cwdRoot string) ([]string, error)

type Skill struct {
	Name        string
	Description string
	BasePath    string // absolute path to the skill's directory
	Body        string // markdown after frontmatter
	Source      string // "home" | "cwd"
}
    Skill is one parsed SKILL.md.

func LoadAll(homeRoot, cwdRoot string) ([]Skill, []string, error)
    LoadAll scans homeRoot and cwdRoot for skill directories. cwd-level skills
    replace home-level skills with the same name. Returns the merged skill list,
    per-skill warnings, and only returns a top-level error for unrecoverable IO.

```

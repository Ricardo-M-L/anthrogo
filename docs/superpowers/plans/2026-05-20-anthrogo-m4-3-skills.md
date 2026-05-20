# M4.3 Skills Implementation Plan

> Execute via subagent-driven-development. Checkbox tasks.

**Goal:** Port upstream's Skills mechanism. Markdown files with frontmatter (`name`, `description`) loaded from `~/.anthrogo/skills/<n>/SKILL.md` + `<cwd>/.anthrogo/skills/<n>/SKILL.md`. Listed in system prompt; invoked by model via a new built-in `Skill` tool that returns the skill body as tool_result text.

**Architecture:** New `pkg/skill/` package (Skill struct + Loader + Registry); new `pkg/tool/skill.go` tool referencing the registry; existing `BuildSystemPrompt` gets a Skills option; new `/skills` slash command; `command.Host` gains `Skills()` accessor.

---

## Task 1: `pkg/skill/` package

**Files:**
- Create: `pkg/skill/skill.go`
- Create: `pkg/skill/loader.go`
- Create: `pkg/skill/registry.go`
- Create: `pkg/skill/loader_test.go`
- Create: `pkg/skill/registry_test.go`
- Create: `pkg/skill/testdata/...` (6 sample skill directories)

- [ ] **Step 1.1: Skill struct**

`pkg/skill/skill.go`:

```go
package skill

// Skill is one parsed SKILL.md.
type Skill struct {
	Name        string
	Description string
	BasePath    string // absolute path to the skill's directory
	Body        string // markdown after frontmatter
	Source      string // "home" | "cwd"
}
```

- [ ] **Step 1.2: Loader**

`pkg/skill/loader.go`:

```go
package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

const maxBodyBytes = 1 << 20

var nameRE = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)

type frontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// LoadAll scans homeRoot and cwdRoot for skill directories. cwd-level skills
// replace home-level skills with the same name. Returns the merged skill list,
// per-skill warnings, and only returns a top-level error for unrecoverable IO.
func LoadAll(homeRoot, cwdRoot string) ([]Skill, []string, error) {
	var warnings []string
	home, w1 := loadDir(homeRoot, "home")
	warnings = append(warnings, w1...)
	cwd, w2 := loadDir(cwdRoot, "cwd")
	warnings = append(warnings, w2...)

	merged := map[string]Skill{}
	for _, s := range home {
		merged[s.Name] = s
	}
	for _, s := range cwd {
		if _, exists := merged[s.Name]; exists {
			warnings = append(warnings, fmt.Sprintf("skill %q in cwd overrides home version", s.Name))
		}
		merged[s.Name] = s
	}
	out := make([]Skill, 0, len(merged))
	for _, s := range merged {
		out = append(out, s)
	}
	return out, warnings, nil
}

func loadDir(root, source string) ([]Skill, []string) {
	if root == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, nil // missing root is OK
	}
	var skills []Skill
	var warnings []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if !nameRE.MatchString(name) {
			warnings = append(warnings, fmt.Sprintf("skill dir %q has invalid name", name))
			continue
		}
		base := filepath.Join(root, name)
		path := filepath.Join(base, "SKILL.md")
		raw, err := os.ReadFile(path)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("skill %q: no SKILL.md", name))
			continue
		}
		fm, body, ok := splitFrontmatter(raw)
		if !ok {
			warnings = append(warnings, fmt.Sprintf("skill %q: missing or malformed frontmatter", name))
			continue
		}
		var meta frontmatter
		if err := yaml.Unmarshal(fm, &meta); err != nil {
			warnings = append(warnings, fmt.Sprintf("skill %q: bad YAML: %v", name, err))
			continue
		}
		if meta.Name == "" || meta.Description == "" {
			warnings = append(warnings, fmt.Sprintf("skill %q: empty name or description", name))
			continue
		}
		if meta.Name != name {
			warnings = append(warnings, fmt.Sprintf("skill %q: frontmatter name %q doesn't match directory", name, meta.Name))
			continue
		}
		if len(body) > maxBodyBytes {
			warnings = append(warnings, fmt.Sprintf("skill %q: body truncated to %d bytes", name, maxBodyBytes))
			body = body[:maxBodyBytes]
		}
		abs, _ := filepath.Abs(base)
		skills = append(skills, Skill{
			Name:        name,
			Description: meta.Description,
			BasePath:    abs,
			Body:        string(body),
			Source:      source,
		})
	}
	return skills, warnings
}

// splitFrontmatter separates the leading "---\n...\n---\n" block.
func splitFrontmatter(raw []byte) (frontmatterBytes, body []byte, ok bool) {
	s := string(raw)
	if !strings.HasPrefix(s, "---\n") && !strings.HasPrefix(s, "---\r\n") {
		return nil, nil, false
	}
	start := strings.Index(s, "\n") + 1
	end := strings.Index(s[start:], "\n---")
	if end < 0 {
		return nil, nil, false
	}
	fmEnd := start + end
	bodyStart := fmEnd + len("\n---")
	if bodyStart < len(s) && (s[bodyStart] == '\n' || s[bodyStart] == '\r') {
		bodyStart++
	}
	if bodyStart < len(s) && s[bodyStart] == '\n' {
		bodyStart++
	}
	return []byte(s[start:fmEnd]), []byte(s[bodyStart:]), true
}
```

The `splitFrontmatter` may need iteration — if your testdata uses `\r\n`, handle both. Spot-check by writing one of the testdata SKILL.md files with `\n` and validating the parse works.

- [ ] **Step 1.3: Registry**

`pkg/skill/registry.go`:

```go
package skill

import (
	"sort"
	"sync"
)

type Registry struct {
	mu     sync.RWMutex
	skills map[string]Skill
}

func NewRegistry(list []Skill) *Registry {
	r := &Registry{skills: map[string]Skill{}}
	for _, s := range list {
		r.skills[s.Name] = s
	}
	return r
}

func (r *Registry) Get(name string) (Skill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.skills[name]
	return s, ok
}

func (r *Registry) List() []Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Skill, 0, len(r.skills))
	for _, s := range r.skills {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (r *Registry) Reload(homeRoot, cwdRoot string) ([]string, error) {
	skills, warnings, err := LoadAll(homeRoot, cwdRoot)
	if err != nil {
		return warnings, err
	}
	r.mu.Lock()
	r.skills = map[string]Skill{}
	for _, s := range skills {
		r.skills[s.Name] = s
	}
	r.mu.Unlock()
	return warnings, nil
}
```

- [ ] **Step 1.4: testdata**

Create these directories under `pkg/skill/testdata/`:

```
testdata/
├── valid-home/
│   └── git-flow/
│       └── SKILL.md
├── valid-cwd-overrides/
│   └── git-flow/
│       └── SKILL.md          # different body, same name
├── bad-no-frontmatter/
│   └── git-flow/
│       └── SKILL.md          # no --- markers
├── bad-name-mismatch/
│   └── git-flow/
│       └── SKILL.md          # frontmatter name: gitflow
├── bad-no-skill-md/
│   └── empty/                # dir with no SKILL.md
└── bad-invalid-dirname/
    └── BadName/              # uppercase invalid
```

Valid SKILL.md template:
```markdown
---
name: git-flow
description: Use when starting a new feature branch off main.
---

# git-flow

Body of the skill.
```

- [ ] **Step 1.5: loader_test.go**

Cover:
- `TestLoadAll_ValidHomeOnly`
- `TestLoadAll_CwdOverridesHome`
- `TestLoadAll_BadFrontmatterSkippedWithWarning`
- `TestLoadAll_NameMismatchSkipped`
- `TestLoadAll_MissingSkillMdSkipped`
- `TestLoadAll_InvalidDirNameSkipped`
- `TestLoadAll_BothRootsMissing_NoError`
- `TestSplitFrontmatter_HandlesCRLF` (write a small CRLF SKILL.md and parse it)

- [ ] **Step 1.6: registry_test.go**

Cover:
- `TestRegistry_GetReturnsFalseForMissing`
- `TestRegistry_ListSortedByName`
- `TestRegistry_Reload_ReplacesAtomically`

- [ ] **Step 1.7: Gate**

```bash
go test ./pkg/skill/... -count=1
git add pkg/skill/
```

Expect: all PASS.

---

## Task 2: `Skill` tool

**Files:**
- Create: `pkg/tool/skill.go`
- Create: `pkg/tool/skill_test.go`

- [ ] **Step 2.1: Skill tool**

`pkg/tool/skill.go`:

```go
package tool

import (
	"context"
	"fmt"

	"github.com/ricardo/anthrogo/pkg/permissions"
	"github.com/ricardo/anthrogo/pkg/skill"
)

// Skill is a model-invoked tool that returns the body of a registered skill.
type Skill struct {
	registry *skill.Registry
}

func NewSkill(r *skill.Registry) *Skill { return &Skill{registry: r} }

func (*Skill) Name() string                       { return "Skill" }
func (*Skill) Description(context.Context) string { return skillDescription }
func (*Skill) UserFacingName(input map[string]any) string {
	if s, _ := input["skill"].(string); s != "" {
		return "Skill: " + s
	}
	return "Skill"
}
func (*Skill) IsReadOnly() bool        { return true }
func (*Skill) IsConcurrencySafe() bool { return true }

func (*Skill) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"skill": map[string]any{"type": "string", "description": "Name of the skill to invoke."},
			"args":  map[string]any{"type": "string", "description": "Optional free-text arguments."},
		},
		"required": []string{"skill"},
	}
}

// Permission allows Skill unconditionally — it only returns prepared markdown.
// Any side effects happen through other tools the model invokes afterward.
func (*Skill) Permission(context.Context, map[string]any) permissions.Decision {
	return permissions.Decision{Behavior: permissions.BehaviorAllow}
}

func (s *Skill) Call(ctx context.Context, input map[string]any, _ *Context) (Result, error) {
	name, _ := input["skill"].(string)
	if name == "" {
		msg := "skill: missing 'skill' field"
		return Result{Type: ResultText, Text: msg, ForLLM: msg, IsError: true}, nil
	}
	if s.registry == nil {
		msg := "skill registry not configured"
		return Result{Type: ResultText, Text: msg, ForLLM: msg, IsError: true}, nil
	}
	sk, ok := s.registry.Get(name)
	if !ok {
		msg := fmt.Sprintf("Skill not found: %s", name)
		return Result{Type: ResultText, Text: msg, ForLLM: msg, IsError: true}, nil
	}
	return Result{
		Type:   ResultText,
		Text:   sk.Body,
		ForLLM: sk.Body,
		Data:   map[string]any{"name": sk.Name, "base_path": sk.BasePath, "source": sk.Source},
	}, nil
}

const skillDescription = `Invoke a registered Skill by name. The tool returns the skill's full instructions; you then follow them, using other tools (Read, Bash, etc.) as the skill directs. Available skills are listed in the system prompt.`
```

- [ ] **Step 2.2: skill_test.go**

```go
package tool

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/pkg/skill"
)

func TestSkill_ReturnsBody(t *testing.T) {
	r := skill.NewRegistry([]skill.Skill{
		{Name: "git-flow", Description: "x", Body: "DO THE THING", BasePath: "/p", Source: "home"},
	})
	tool := NewSkill(r)
	res, err := tool.Call(context.Background(), map[string]any{"skill": "git-flow"}, nil)
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Equal(t, "DO THE THING", res.ForLLM)
	require.Equal(t, "git-flow", res.Data["name"])
}

func TestSkill_UnknownReturnsError(t *testing.T) {
	r := skill.NewRegistry(nil)
	tool := NewSkill(r)
	res, _ := tool.Call(context.Background(), map[string]any{"skill": "nope"}, nil)
	require.True(t, res.IsError)
	require.Contains(t, res.ForLLM, "not found")
}

func TestSkill_MissingFieldReturnsError(t *testing.T) {
	tool := NewSkill(skill.NewRegistry(nil))
	res, _ := tool.Call(context.Background(), map[string]any{}, nil)
	require.True(t, res.IsError)
}

func TestSkill_PermissionAllow(t *testing.T) {
	tool := NewSkill(skill.NewRegistry(nil))
	d := tool.Permission(context.Background(), nil)
	require.Equal(t, "allow", string(d.Behavior))
}
```

Verify `permissions.BehaviorAllow` actual value — `grep -n "BehaviorAllow" pkg/permissions/`. If it's a string const "allow", the test above works; if it's an int, adapt.

- [ ] **Step 2.3: Gate**

```bash
go test ./pkg/tool/... -count=1 -run TestSkill
git add pkg/tool/skill.go pkg/tool/skill_test.go
```

---

## Task 3: BuildSystemPrompt + system option

**Files:**
- Modify: `internal/system/prompt.go`
- Modify: `internal/system/prompt_test.go`

- [ ] **Step 3.1: Add Skills option**

In `internal/system/prompt.go` `Options`:

```go
import "github.com/ricardo/anthrogo/pkg/skill"

type Options struct {
    /* existing */
    Skills []skill.Skill
}
```

In `BuildSystemPrompt`, after the `Available tools:` line + mcp-prefix mention block, add:

```go
if len(opts.Skills) > 0 {
    b.WriteString("\nAvailable skills (invoke via the Skill tool, e.g. {\"skill\":\"git-flow\"}):\n")
    for _, sk := range opts.Skills {
        fmt.Fprintf(&b, "- %s: %s\n", sk.Name, sk.Description)
    }
}
```

(Use `pkg/skill.Skill` type — that's fine, system can depend on pkg/skill since pkg/skill has no internal/system dep.)

- [ ] **Step 3.2: Tests**

```go
func TestBuildSystemPrompt_ListsSkillsWhenPresent(t *testing.T) {
    got := BuildSystemPrompt(Options{
        Skills: []skill.Skill{
            {Name: "a", Description: "use when X"},
            {Name: "b", Description: "use when Y"},
        },
    })
    require.Contains(t, got, "Available skills")
    require.Contains(t, got, "- a: use when X")
    require.Contains(t, got, "- b: use when Y")
}

func TestBuildSystemPrompt_OmitsSkillsBlockWhenEmpty(t *testing.T) {
    got := BuildSystemPrompt(Options{})
    require.NotContains(t, got, "Available skills")
}
```

Add `import "github.com/ricardo/anthrogo/pkg/skill"` to the test file.

- [ ] **Step 3.3: Gate**

```bash
go test ./internal/system/... -count=1
git add internal/system/
```

---

## Task 4: `/skills` builtin

**Files:**
- Modify: `pkg/command/command.go` (Host gains `Skills() *skill.Registry`)
- Modify: `pkg/command/builtins/builtins_test.go` (fakeHost.skills field + method)
- Modify: `internal/tui/app.go` (App.Skills accessor)
- Modify: `internal/headless/runner.go` — not needed for /skills (headless mode doesn't run slash commands)
- Create: `pkg/command/builtins/skills.go`
- Create: `pkg/command/builtins/skills_test.go`

- [ ] **Step 4.1: Host accessor**

In `pkg/command/command.go`:

```go
import "github.com/ricardo/anthrogo/pkg/skill"

type Host interface {
    /* existing */
    Skills() *skill.Registry
}
```

In `internal/tui/app.go`:

```go
type Options struct {
    /* existing */
    Skills *skill.Registry
}

func (a *App) Skills() *skill.Registry { return a.opts.Skills }
```

In `pkg/command/builtins/builtins_test.go` fakeHost: add `skills *skill.Registry` field + method `func (f *fakeHost) Skills() *skill.Registry { return f.skills }`. Add import.

- [ ] **Step 4.2: /skills builtin**

`pkg/command/builtins/skills.go`:

```go
package builtins

import (
	"context"
	"fmt"
	"strings"

	"github.com/ricardo/anthrogo/pkg/command"
	"github.com/ricardo/anthrogo/pkg/skill"
)

type Skills struct {
	HomeRoot string
	CwdRoot  string
}

func (Skills) Name() string        { return "/skills" }
func (Skills) Aliases() []string   { return nil }
func (Skills) Description() string { return "List loaded skills (subcommands: show <name>, reload)" }
func (Skills) Type() command.Type  { return command.TypeLocal }

func (s Skills) Run(ctx context.Context, args string, host command.Host) (command.Result, error) {
	reg := host.Skills()
	if reg == nil {
		return command.Result{Text: "no skill registry configured"}, nil
	}
	args = strings.TrimSpace(args)
	switch {
	case args == "":
		return listSkills(reg), nil
	case args == "reload":
		warnings, err := reg.Reload(s.HomeRoot, s.CwdRoot)
		if err != nil {
			return command.Result{}, err
		}
		out := fmt.Sprintf("reloaded skills (now %d)", len(reg.List()))
		if len(warnings) > 0 {
			out += "\nwarnings:\n" + strings.Join(warnings, "\n")
		}
		return command.Result{Text: out}, nil
	case strings.HasPrefix(args, "show "):
		name := strings.TrimSpace(strings.TrimPrefix(args, "show"))
		return showSkill(reg, name), nil
	default:
		return command.Result{Text: "usage: /skills [show <name> | reload]"}, nil
	}
}

func listSkills(reg *skill.Registry) command.Result {
	list := reg.List()
	if len(list) == 0 {
		return command.Result{Text: "(no skills loaded)"}
	}
	var b strings.Builder
	for _, sk := range list {
		fmt.Fprintf(&b, "%-25s  [%s]  %s\n", sk.Name, sk.Source, sk.Description)
	}
	return command.Result{Text: b.String()}
}

func showSkill(reg *skill.Registry, name string) command.Result {
	sk, ok := reg.Get(name)
	if !ok {
		return command.Result{Text: "skill not found: " + name}
	}
	return command.Result{Text: fmt.Sprintf("# %s\n[source: %s, base: %s]\n\n%s", sk.Name, sk.Source, sk.BasePath, sk.Body)}
}
```

- [ ] **Step 4.3: Tests**

`pkg/command/builtins/skills_test.go`:

```go
package builtins

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/pkg/skill"
)

func TestSkills_EmptyList(t *testing.T) {
	h := newFakeHost()
	h.skills = skill.NewRegistry(nil)
	res, _ := Skills{}.Run(context.Background(), "", h)
	require.Contains(t, res.Text, "no skills")
}

func TestSkills_ListSorted(t *testing.T) {
	h := newFakeHost()
	h.skills = skill.NewRegistry([]skill.Skill{
		{Name: "b", Description: "B", Source: "home"},
		{Name: "a", Description: "A", Source: "cwd"},
	})
	res, _ := Skills{}.Run(context.Background(), "", h)
	// a before b
	require.Less(t, indexOf(res.Text, "a "), indexOf(res.Text, "b "))
	require.Contains(t, res.Text, "[home]")
	require.Contains(t, res.Text, "[cwd]")
}

func TestSkills_ShowKnown(t *testing.T) {
	h := newFakeHost()
	h.skills = skill.NewRegistry([]skill.Skill{
		{Name: "git-flow", Description: "use it", Body: "BODY", Source: "home"},
	})
	res, _ := Skills{}.Run(context.Background(), "show git-flow", h)
	require.Contains(t, res.Text, "BODY")
	require.Contains(t, res.Text, "git-flow")
}

func TestSkills_ShowUnknown(t *testing.T) {
	h := newFakeHost()
	h.skills = skill.NewRegistry(nil)
	res, _ := Skills{}.Run(context.Background(), "show nope", h)
	require.Contains(t, res.Text, "skill not found")
}

func TestSkills_UnknownSubcommand(t *testing.T) {
	h := newFakeHost()
	h.skills = skill.NewRegistry(nil)
	res, _ := Skills{}.Run(context.Background(), "garbage", h)
	require.Contains(t, res.Text, "usage:")
}

func indexOf(s, sub string) int {
	return strings.Index(s, sub)
}
```

Add `"strings"` import for the indexOf helper.

- [ ] **Step 4.4: Gate**

```bash
go test ./pkg/command/... -count=1
git add pkg/command/
```

---

## Task 5: CLI wiring + paths + docs + version

**Files:**
- Modify: `internal/config/paths.go` — add `SkillsDir(home string)` helper
- Modify: `cmd/anthrogo/main.go` — load skills, register Skill tool, pass Skills option, register /skills builtin
- Modify: `internal/version/version.go` → 0.4.2-dev
- Modify: `CHANGELOG.md`, `README.md`

- [ ] **Step 5.1: paths.go**

Add to `internal/config/paths.go`:

```go
func SkillsDir(home string) string {
    return filepath.Join(home, ".anthrogo", "skills")
}
```

(Adapt if there's an existing `AnthrogoHome` helper that already handles `$ANTHROGO_HOME`.)

- [ ] **Step 5.2: main.go wiring**

After config load + cwd resolve:

```go
homeSkillsRoot := config.SkillsDir(os.Getenv("HOME"))
cwdSkillsRoot := filepath.Join(cwd, ".anthrogo", "skills")
loaded, warnings, _ := skill.LoadAll(homeSkillsRoot, cwdSkillsRoot)
for _, w := range warnings {
    fmt.Fprintln(os.Stderr, "skills:", w)
}
skillReg := skill.NewRegistry(loaded)
```

Inject:
- `tools.Register(tool.NewSkill(skillReg))` after `registerTools(cfg)`
- In `system.BuildSystemPrompt(...)` options: add `Skills: skillReg.List()`
- In `cmds := registerCommands(...)`: add `cmds.Register(builtins.Skills{HomeRoot: homeSkillsRoot, CwdRoot: cwdSkillsRoot})`
- In `tui.New(tui.Options{...})`: add `Skills: skillReg`

If `registerCommands` doesn't currently take any args, add a single `skillsHome, skillsCwd string` param and thread through.

- [ ] **Step 5.3: Version**

`internal/version/version.go`:
```go
var Version = "0.4.2-dev"
```

- [ ] **Step 5.4: CHANGELOG**

Prepend after `# Changelog`:

```markdown
## [0.4.2-dev] — 2026-05-20

M4.3 — Skills (markdown + frontmatter, model-invoked).

### Added
- `pkg/skill/` package: Skill struct + Loader + Registry, scans `~/.anthrogo/skills/<n>/SKILL.md` and `<cwd>/.anthrogo/skills/<n>/SKILL.md`.
- Built-in `Skill` tool: model invokes a registered skill by name; tool returns the skill's markdown body as tool_result text. Allow-by-default (no side effect — model uses other gated tools to act on skill instructions).
- BuildSystemPrompt lists available skills as `- name: description` after the tool list.
- `/skills` slash command: bare lists, `show <name>` prints body, `reload` re-scans both roots.
- Frontmatter validation: name must match `^[a-z][a-z0-9-]{0,63}$`, match directory name, non-empty description; SKILL.md body > 1 MiB truncated with warning.

### Changed
- `command.Host` gains `Skills() *skill.Registry`.
- `tui.Options` gains `Skills *skill.Registry`.
- `system.Options` gains `Skills []skill.Skill`.

### Known issues / deferred
- Plugin-bundled skills + namespacing (`plugin:skill-name`) land in M4.4.
- Skill packaging / install commands (`/skills install <url>`) are deferred.
- Skill versioning + dependency declarations are deferred.
```

- [ ] **Step 5.5: README**

Add a "Skills" section between "Hooks" and "Compaction":

```markdown
## Skills

A Skill is a markdown file the model can invoke on demand. Layout:

```
~/.anthrogo/skills/<name>/
├── SKILL.md                # required, with frontmatter (name + description)
├── scripts/                # optional, model reads via the Read tool
└── references/             # optional
```

`SKILL.md`:

```markdown
---
name: git-flow
description: Use when starting a new feature branch off main.
---

# git-flow

When the user asks to start a new branch, do X then Y.
```

anthrogo lists every loaded skill in the system prompt (name + description). The model picks one, calls the `Skill` tool with `{"skill": "git-flow"}`, and gets the full markdown back. From there it follows the instructions, using Read / Bash / Write etc. as the skill dictates — all gated by the existing permission rules.

`/skills` lists them, `/skills show <name>` prints one, `/skills reload` re-scans.

Project-level `<cwd>/.anthrogo/skills/<name>/SKILL.md` overrides a same-named home skill.
```

- [ ] **Step 5.6: Stage**

```bash
git add cmd/anthrogo/main.go internal/config/paths.go internal/version/version.go CHANGELOG.md README.md
```

---

## Task 6: Acceptance

```bash
go build ./...
go vet ./...
go test ./...
go test -race -count=2 ./pkg/skill ./pkg/tool ./pkg/command/builtins ./internal/system ./pkg/command ./internal/tui ./internal/mcp ./internal/hooks ./pkg/query ./pkg/compact ./pkg/permissions
for i in 1 2 3; do go clean -testcache; go test ./... 2>&1 | grep -E "FAIL|^FAIL" || echo "run $i clean"; done
./bin/anthrogo --version
```

All clean, 3× uncached clean, version `0.4.2-dev`.

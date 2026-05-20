# anthrogo M4.3 — Skills (markdown + frontmatter, model-invoked)

**Status:** approved (self-authorized)
**Date:** 2026-05-20
**Predecessor:** M4.2 (`docs/superpowers/specs/2026-05-20-anthrogo-m4-2-compact-design.md`)

## 1. Goal

Port upstream's "Skills" mechanism. A Skill is a markdown file describing a capability the model can invoke on demand. anthrogo lists available skills in the system prompt (name + when-to-use description). When the model wants to use one, it calls a built-in `Skill` tool with the skill's name; anthrogo returns the skill's full SKILL.md body as the tool result, after which the model follows the skill's instructions.

This is **prompt-injection on demand**, not a code-execution mechanism. Side effects from a skill happen because the model uses other tools (Bash, Read, Write) per the skill's instructions, and those tools have their own permission gates.

## 2. Scope

- `pkg/skill/` package: Skill type, loader, registry.
- New built-in tool `Skill` registered in cmd/anthrogo (alongside the 13 existing M1-M3 tools).
- `internal/system.BuildSystemPrompt` opts gain a `Skills []skill.Skill` field; output adds a "Available skills" stanza when non-empty.
- New `/skills` slash command (list / show / reload).
- Layout: `~/.anthrogo/skills/<name>/SKILL.md` (home) + `<cwd>/.anthrogo/skills/<name>/SKILL.md` (project; project wins on same name).
- Skill body is plain markdown; frontmatter is YAML between `---` markers (`name`, `description`).
- Subfiles (any `.md`, scripts, `references/`) under the skill directory are accessible to the model via the Read tool (no special skill-internal API).

**Out of scope:** plugin-bundled skills (M4.4 — plugins contribute skills); namespacing (`plugin-name:skill-name`); skill versioning; skill packaging / install commands.

## 3. Skill format

```
~/.anthrogo/skills/git-flow/
├── SKILL.md             # required
├── scripts/
│   └── new-branch.sh
└── references/
    └── workflow.md
```

`SKILL.md`:

```markdown
---
name: git-flow
description: Use when starting a new feature branch off main, including release-branch naming conventions.
---

# git-flow

When the user asks to start a new feature branch:

1. Read scripts/new-branch.sh
2. Confirm the branch name with the user
3. Run the script

See references/workflow.md for the full team workflow.
```

Skill name validation:
- Must match `^[a-z][a-z0-9-]{0,63}$`
- Skill name must equal the directory name
- Frontmatter `name` must match the directory name; mismatch = warning + skip

## 4. Loader behavior

- Scan `~/.anthrogo/skills/` and `<cwd>/.anthrogo/skills/`.
- For each immediate subdirectory, look for `SKILL.md`. If missing → skip with warning.
- Parse frontmatter; if missing/malformed → skip with warning.
- Validate `name` and `description` non-empty; skip if not.
- Project-level skills with the same name as a home-level skill replace the home version (warning logged).
- Body (everything after the closing `---`) is loaded into memory at startup (typical SKILL.md is small, < 50 KB).
- If body exceeds 1 MiB → load just the first 1 MiB + warning.

## 5. Code organization

```
pkg/skill/
├── skill.go           # Skill struct
├── loader.go          # LoadAll(homeRoot, cwdRoot) ([]Skill, []string warnings, error)
├── registry.go        # Registry struct + NewRegistry(skills) + Get/List/Reload
├── skill_test.go
├── loader_test.go
├── registry_test.go
└── testdata/
    ├── valid-home/
    │   └── git-flow/SKILL.md
    ├── valid-cwd/
    │   └── git-flow/SKILL.md       # overrides home
    ├── bad-no-frontmatter/
    │   └── x/SKILL.md
    ├── bad-name-mismatch/
    │   └── x/SKILL.md              # frontmatter says name: y
    └── bad-no-skill-md/
        └── x/                       # no SKILL.md
```

```go
// pkg/skill/skill.go
type Skill struct {
    Name        string
    Description string
    BasePath    string  // absolute path to the skill's directory
    Body        string  // markdown after frontmatter
    Source      string  // "home" | "cwd"
}
```

```go
// pkg/skill/registry.go
type Registry struct {
    mu     sync.RWMutex
    skills map[string]Skill
}

func NewRegistry(list []Skill) *Registry
func (r *Registry) Get(name string) (Skill, bool)
func (r *Registry) List() []Skill   // sorted by name for stable prompt caching
func (r *Registry) Reload(homeRoot, cwdRoot string) ([]string, error)  // returns warnings
```

```go
// pkg/skill/loader.go
func LoadAll(homeRoot, cwdRoot string) (skills []Skill, warnings []string, err error)
```

## 6. System prompt injection

Modify `internal/system/prompt.go`:
- `Options` gains `Skills []skill.Skill` field.
- After "Available tools:" line, if `len(opts.Skills) > 0`, emit:

```
Available skills (use the Skill tool to invoke; pass {"skill": "<name>"}):
- git-flow: Use when starting a new feature branch off main, including release-branch naming conventions.
- inspect-pr: Use when summarizing or reviewing a GitHub pull request.
```

Stable sort by name to keep prompt caching warm.

If skills list is empty, no stanza emitted.

## 7. `Skill` tool

New file `pkg/tool/skill.go`. Embeds `DefaultPermission`. Schema:

```json
{
  "type": "object",
  "properties": {
    "skill": {"type": "string", "description": "Name of the skill to invoke."},
    "args": {"type": "string", "description": "Optional free-text arguments for the skill (rare)."}
  },
  "required": ["skill"]
}
```

`Permission()` returns `BehaviorAllow` directly. Rationale: Skill returns only the prepared markdown text — no filesystem / shell side effect. Any side effects the model triggers afterward go through other tools' gates.

`Call()` looks up the skill by name. On hit:
- result.Type = ResultText
- result.Text = the skill body
- result.ForLLM = the skill body (the LLM sees the markdown directly)
- result.Data = `{"name": "<skill-name>", "base_path": "<absolute path>", "source": "home"|"cwd"}`

On miss: result.IsError = true, ForLLM = "Skill not found: <name>".

`UserFacingName(input)` returns `"Skill: <name>"` for the TUI permission banner / chat header.

Construction: `tool.NewSkill(registry *skill.Registry) *tool.Skill` — the tool holds a reference to the registry so reloads are reflected automatically.

## 8. CLI wiring

`cmd/anthrogo/main.go`:
1. Load skills early (after config load, before tools registration):
   ```go
   homeRoot := config.SkillsDir(os.Getenv("HOME"))
   cwdRoot  := filepath.Join(cwd, ".anthrogo", "skills")
   skills, warnings, err := skill.LoadAll(homeRoot, cwdRoot)
   for _, w := range warnings { fmt.Fprintln(os.Stderr, "skills:", w) }
   skillReg := skill.NewRegistry(skills)
   ```
2. Register the `Skill` tool: `tools.Register(tool.NewSkill(skillReg))`.
3. Pass `skillReg.List()` to BuildSystemPrompt:
   ```go
   systemPrompt := system.BuildSystemPrompt(system.Options{ /* existing */, Skills: skillReg.List() })
   ```
4. `host.Skills() *skill.Registry` — add to `command.Host` interface so the `/skills` slash command can query.

`config.SkillsDir(home string) string` returns `filepath.Join(home, ".anthrogo", "skills")`. Add to `internal/config/paths.go`.

## 9. `/skills` slash command

New `pkg/command/builtins/skills.go`:

```
/skills                    # list all loaded skills with name + description
/skills show <name>        # print the full skill body
/skills reload             # re-scan home + cwd, replace registry contents
```

## 10. Error handling

| Case | Handling |
|---|---|
| `~/.anthrogo/skills/` missing | OK, no skills loaded |
| Skill dir has no SKILL.md | Skip + warning |
| SKILL.md has no frontmatter | Skip + warning |
| Frontmatter is malformed YAML | Skip + warning |
| `name` empty or doesn't match dir | Skip + warning |
| `description` empty | Skip + warning |
| Body > 1 MiB | Truncate to 1 MiB + warning |
| Same name in home + cwd | cwd wins; warning |
| `Skill` tool called with unknown name | result.IsError = true with helpful text |
| Project-level skills dir doesn't exist | OK, only home-level loaded |

## 11. Testing

- `pkg/skill/loader_test.go`:
  - Valid home-only skill
  - Valid cwd-only skill
  - cwd overrides home (warning emitted)
  - Bad frontmatter skipped (warning)
  - Name mismatch skipped (warning)
  - Missing SKILL.md skipped (warning)
  - Body truncation at 1 MiB
- `pkg/skill/registry_test.go`:
  - List sorted by name
  - Get returns false for missing
  - Reload replaces atomic
- `pkg/tool/skill_test.go`:
  - Call with known name returns body
  - Call with unknown name returns IsError
  - Permission() returns Allow
- `pkg/command/builtins/skills_test.go`:
  - List / show / reload all paths

## 12. CHANGELOG / version

- Bump to `0.4.2-dev`.
- README: new "Skills" section between "Hooks" and "Compaction".

## 13. Acceptance

- `go build/vet/test/-race` clean
- 3× uncached full-repo sweep clean
- `./bin/anthrogo --version` → `0.4.2-dev`

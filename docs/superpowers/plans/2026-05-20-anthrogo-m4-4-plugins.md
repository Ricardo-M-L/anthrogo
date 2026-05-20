# M4.4 Plugins Implementation Plan

> Execute via subagent-driven-development.

**Goal:** Add a `pkg/plugin/` package that loads "plugins" (self-contained directories under `~/.anthrogo/plugins/` and `<cwd>/.anthrogo/plugins/`), each manifest-described with optional commands / skills / hooks / mcpServers contributions. Inject contributions into the existing 4 registries at startup.

**Architecture:** Pure loader — no IO inside any registry lock. Plugin contributions absorbed into existing types (no new tool kind). Order of registration in main.go matters (plugins merge BEFORE hookMgr is constructed so hooks see the combined config).

---

## Task 1: `pkg/plugin/` package

**Files:**
- Create: `pkg/plugin/plugin.go`
- Create: `pkg/plugin/manifest.go`
- Create: `pkg/plugin/command.go`
- Create: `pkg/plugin/loader.go`
- Create: `pkg/plugin/registry.go`
- Create: `pkg/plugin/*_test.go`
- Create: `pkg/plugin/testdata/...`

- [ ] **Step 1.1: plugin.go**

```go
package plugin

import (
    "github.com/ricardo/anthrogo/internal/mcp"
    "github.com/ricardo/anthrogo/internal/hooks"
    "github.com/ricardo/anthrogo/pkg/command"
    "github.com/ricardo/anthrogo/pkg/skill"
)

type Plugin struct {
    Name     string
    Manifest Manifest
    BasePath string
    Source   string // "home" | "cwd"

    Commands   []command.Command
    Skills     []skill.Skill
    Hooks      hooks.Config
    MCPServers map[string]mcp.MCPServerConfig
}
```

- [ ] **Step 1.2: manifest.go**

```go
package plugin

import (
    "github.com/ricardo/anthrogo/internal/hooks"
    "github.com/ricardo/anthrogo/internal/mcp"
)

type Manifest struct {
    Name        string                          `yaml:"name"`
    Version     string                          `yaml:"version,omitempty"`
    Description string                          `yaml:"description,omitempty"`
    Author      string                          `yaml:"author,omitempty"`
    Commands    []CommandSpec                   `yaml:"commands,omitempty"`
    Skills      []SkillRef                      `yaml:"skills,omitempty"`
    Hooks       hooks.Config                    `yaml:"hooks,omitempty"`
    MCPServers  map[string]mcp.MCPServerConfig  `yaml:"mcpServers,omitempty"`
}

type CommandSpec struct {
    Name        string   `yaml:"name"`
    Aliases     []string `yaml:"aliases,omitempty"`
    Description string   `yaml:"description,omitempty"`
    Type        string   `yaml:"type"`
    Body        string   `yaml:"body"`
}

type SkillRef struct {
    Dir string `yaml:"dir"`
}
```

- [ ] **Step 1.3: command.go**

```go
package plugin

import (
    "context"
    "strings"

    "github.com/ricardo/anthrogo/pkg/command"
)

type DynamicCommand struct {
    spec      CommandSpec
    pluginDir string
}

func (d DynamicCommand) Name() string        { return d.spec.Name }
func (d DynamicCommand) Aliases() []string   { return d.spec.Aliases }
func (d DynamicCommand) Description() string { return d.spec.Description + " (plugin)" }
func (d DynamicCommand) Type() command.Type  { return command.Type(d.spec.Type) }

func (d DynamicCommand) Run(_ context.Context, args string, _ command.Host) (command.Result, error) {
    body := d.spec.Body
    if args = strings.TrimSpace(args); args != "" {
        body += "\n\n" + args
    }
    switch command.Type(d.spec.Type) {
    case command.TypeLocalPrompt, command.TypeSubmit:
        return command.Result{SubmitText: body}, nil
    case command.TypeLocal:
        return command.Result{Text: body}, nil
    default:
        return command.Result{Text: "plugin command has unknown type: " + d.spec.Type}, nil
    }
}
```

- [ ] **Step 1.4: loader.go**

```go
package plugin

import (
    "fmt"
    "os"
    "path/filepath"
    "regexp"
    "strings"

    "gopkg.in/yaml.v3"

    "github.com/ricardo/anthrogo/internal/hooks"
    "github.com/ricardo/anthrogo/internal/mcp"
    "github.com/ricardo/anthrogo/pkg/command"
    "github.com/ricardo/anthrogo/pkg/skill"
)

var nameRE = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)

func LoadAll(homeRoot, cwdRoot string) ([]Plugin, []string, error) {
    var warnings []string
    home, w1 := loadDir(homeRoot, "home")
    warnings = append(warnings, w1...)
    cwd, w2 := loadDir(cwdRoot, "cwd")
    warnings = append(warnings, w2...)

    merged := map[string]Plugin{}
    for _, p := range home {
        merged[p.Name] = p
    }
    for _, p := range cwd {
        if _, exists := merged[p.Name]; exists {
            warnings = append(warnings, fmt.Sprintf("plugin %q in cwd overrides home version", p.Name))
        }
        merged[p.Name] = p
    }
    out := make([]Plugin, 0, len(merged))
    for _, p := range merged {
        out = append(out, p)
    }
    return out, warnings, nil
}

func loadDir(root, source string) ([]Plugin, []string) {
    if root == "" {
        return nil, nil
    }
    entries, err := os.ReadDir(root)
    if err != nil {
        return nil, nil
    }
    var plugins []Plugin
    var warnings []string
    for _, e := range entries {
        if !e.IsDir() {
            continue
        }
        dirname := e.Name()
        base := filepath.Join(root, dirname)
        absBase, _ := filepath.Abs(base)
        manifestPath := filepath.Join(base, "plugin.yaml")
        raw, err := os.ReadFile(manifestPath)
        if err != nil {
            warnings = append(warnings, fmt.Sprintf("plugin dir %q: no plugin.yaml", dirname))
            continue
        }
        var m Manifest
        if err := yaml.Unmarshal(raw, &m); err != nil {
            warnings = append(warnings, fmt.Sprintf("plugin %q: bad YAML: %v", dirname, err))
            continue
        }
        if !nameRE.MatchString(m.Name) {
            warnings = append(warnings, fmt.Sprintf("plugin %q: invalid manifest name %q", dirname, m.Name))
            continue
        }
        if m.Name != dirname {
            warnings = append(warnings, fmt.Sprintf("plugin %q: manifest name %q doesn't match directory", dirname, m.Name))
            continue
        }

        p := Plugin{Name: m.Name, Manifest: m, BasePath: absBase, Source: source}

        // Resolve commands
        for _, cs := range m.Commands {
            if cs.Name == "" || !strings.HasPrefix(cs.Name, "/") {
                warnings = append(warnings, fmt.Sprintf("plugin %q: skipping command with bad name %q", m.Name, cs.Name))
                continue
            }
            if cs.Body == "" {
                warnings = append(warnings, fmt.Sprintf("plugin %q: skipping command %q (empty body)", m.Name, cs.Name))
                continue
            }
            p.Commands = append(p.Commands, DynamicCommand{spec: cs, pluginDir: absBase})
        }

        // Resolve skills
        for _, sr := range m.Skills {
            skillRoot := filepath.Join(absBase, sr.Dir)
            // The skill dir should be of form <root>/<skill-name>. We point
            // the skill loader at the parent of the skill dir so its existing
            // logic picks up the SKILL.md.
            parent := filepath.Dir(skillRoot)
            // LoadAll wants two roots; we use one root and an empty cwd.
            // But that scans ALL subdirs under parent — that's wrong if
            // parent contains other unrelated dirs. Instead, hand-craft a
            // Skill by reading the specific dir.
            sk, ok, warn := loadOneSkill(skillRoot, source)
            if warn != "" {
                warnings = append(warnings, fmt.Sprintf("plugin %q skill %q: %s", m.Name, sr.Dir, warn))
            }
            if ok {
                p.Skills = append(p.Skills, sk)
            }
            _ = parent // unused; the helper does the file read directly
        }

        // Resolve hooks paths
        absHooks := absolutizeHooks(m.Hooks, absBase)
        p.Hooks = absHooks

        // Namespace MCP server keys with "<plugin>:" prefix
        if len(m.MCPServers) > 0 {
            p.MCPServers = make(map[string]mcp.MCPServerConfig, len(m.MCPServers))
            for k, v := range m.MCPServers {
                p.MCPServers[m.Name+":"+k] = v
            }
        }

        plugins = append(plugins, p)
    }
    return plugins, warnings
}

// loadOneSkill loads a single skill directory directly (without using
// skill.LoadAll). Returns false + warning if SKILL.md missing / malformed.
func loadOneSkill(skillDir, source string) (skill.Skill, bool, string) {
    skillMD := filepath.Join(skillDir, "SKILL.md")
    raw, err := os.ReadFile(skillMD)
    if err != nil {
        return skill.Skill{}, false, "no SKILL.md at " + skillMD
    }
    // We hand-parse the frontmatter rather than importing pkg/skill's
    // internal splitFrontmatter. Re-implement the tiny logic here.
    s := string(raw)
    if !strings.HasPrefix(s, "---\n") && !strings.HasPrefix(s, "---\r\n") {
        return skill.Skill{}, false, "missing frontmatter"
    }
    start := strings.Index(s, "\n") + 1
    end := strings.Index(s[start:], "\n---")
    if end < 0 {
        return skill.Skill{}, false, "unterminated frontmatter"
    }
    fmEnd := start + end
    bodyStart := fmEnd + len("\n---")
    for bodyStart < len(s) && (s[bodyStart] == '\n' || s[bodyStart] == '\r') {
        bodyStart++
    }
    var meta struct {
        Name        string `yaml:"name"`
        Description string `yaml:"description"`
    }
    if err := yaml.Unmarshal([]byte(s[start:fmEnd]), &meta); err != nil {
        return skill.Skill{}, false, "bad YAML: " + err.Error()
    }
    if meta.Name == "" || meta.Description == "" {
        return skill.Skill{}, false, "empty name or description"
    }
    base, _ := filepath.Abs(skillDir)
    return skill.Skill{
        Name:        meta.Name,
        Description: meta.Description,
        BasePath:    base,
        Body:        s[bodyStart:],
        Source:      source,
    }, true, ""
}

func absolutizeHooks(c hooks.Config, base string) hooks.Config {
    fix := func(list []hooks.Spec) []hooks.Spec {
        out := make([]hooks.Spec, 0, len(list))
        for _, s := range list {
            if s.Command != "" && !filepath.IsAbs(s.Command) {
                s.Command = filepath.Join(base, s.Command)
            }
            out = append(out, s)
        }
        return out
    }
    c.PreToolUse = fix(c.PreToolUse)
    c.PostToolUse = fix(c.PostToolUse)
    c.UserPromptSubmit = fix(c.UserPromptSubmit)
    c.Stop = fix(c.Stop)
    c.SubagentStop = fix(c.SubagentStop)
    c.Notification = fix(c.Notification)
    c.PreCompact = fix(c.PreCompact)
    c.SessionStart = fix(c.SessionStart)
    c.SessionEnd = fix(c.SessionEnd)
    return c
}
```

- [ ] **Step 1.5: registry.go**

```go
package plugin

import (
    "fmt"
    "io"
    "os"
    "path/filepath"
    "sort"
    "sync"
)

type Registry struct {
    mu      sync.RWMutex
    plugins map[string]Plugin
}

func NewRegistry(list []Plugin) *Registry {
    r := &Registry{plugins: map[string]Plugin{}}
    for _, p := range list {
        r.plugins[p.Name] = p
    }
    return r
}

func (r *Registry) Get(name string) (Plugin, bool) {
    r.mu.RLock()
    defer r.mu.RUnlock()
    p, ok := r.plugins[name]
    return p, ok
}

func (r *Registry) List() []Plugin {
    r.mu.RLock()
    defer r.mu.RUnlock()
    out := make([]Plugin, 0, len(r.plugins))
    for _, p := range r.plugins {
        out = append(out, p)
    }
    sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
    return out
}

func (r *Registry) Reload(homeRoot, cwdRoot string) ([]string, error) {
    plugins, warnings, err := LoadAll(homeRoot, cwdRoot)
    if err != nil {
        return warnings, err
    }
    r.mu.Lock()
    r.plugins = map[string]Plugin{}
    for _, p := range plugins {
        r.plugins[p.Name] = p
    }
    r.mu.Unlock()
    return warnings, nil
}

// Install copies srcDir into destRoot/<plugin-name>/. Validates that srcDir
// contains plugin.yaml. Returns the parsed Plugin on success.
func (r *Registry) Install(srcDir, destRoot string) (Plugin, error) {
    raw, err := os.ReadFile(filepath.Join(srcDir, "plugin.yaml"))
    if err != nil {
        return Plugin{}, fmt.Errorf("install: no plugin.yaml in %s", srcDir)
    }
    var m Manifest
    if err := unmarshalYAML(raw, &m); err != nil {
        return Plugin{}, fmt.Errorf("install: bad manifest: %w", err)
    }
    if !nameRE.MatchString(m.Name) {
        return Plugin{}, fmt.Errorf("install: invalid plugin name %q", m.Name)
    }
    dest := filepath.Join(destRoot, m.Name)
    if _, err := os.Stat(dest); err == nil {
        return Plugin{}, fmt.Errorf("install: plugin %q already exists at %s", m.Name, dest)
    }
    if err := os.MkdirAll(destRoot, 0o755); err != nil {
        return Plugin{}, err
    }
    if err := copyDir(srcDir, dest); err != nil {
        return Plugin{}, err
    }
    plugins, _, _ := LoadAll(destRoot, "")
    var got Plugin
    for _, p := range plugins {
        if p.Name == m.Name {
            got = p
            break
        }
    }
    r.mu.Lock()
    r.plugins[got.Name] = got
    r.mu.Unlock()
    return got, nil
}

// Remove deletes the home-rooted plugin <name> and unregisters it.
func (r *Registry) Remove(name, homeRoot string) error {
    target := filepath.Join(homeRoot, name)
    info, err := os.Stat(target)
    if err != nil {
        return fmt.Errorf("remove: %w", err)
    }
    if !info.IsDir() {
        return fmt.Errorf("remove: %s is not a directory", target)
    }
    if err := os.RemoveAll(target); err != nil {
        return err
    }
    r.mu.Lock()
    delete(r.plugins, name)
    r.mu.Unlock()
    return nil
}

// copyDir copies src to dst recursively. Preserves file mode bits.
func copyDir(src, dst string) error {
    return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
        if err != nil { return err }
        rel, _ := filepath.Rel(src, path)
        target := filepath.Join(dst, rel)
        if info.IsDir() {
            return os.MkdirAll(target, info.Mode())
        }
        in, err := os.Open(path)
        if err != nil { return err }
        defer in.Close()
        out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
        if err != nil { return err }
        defer out.Close()
        _, err = io.Copy(out, in)
        return err
    })
}

// unmarshalYAML is a thin wrapper so we don't import yaml in this file's top.
func unmarshalYAML(raw []byte, dst any) error {
    return yamlUnmarshal(raw, dst)
}
```

Add a small forwarder file or just import yaml directly. (Simpler: drop `unmarshalYAML` indirection and use `yaml.Unmarshal` here directly. Adjust as you prefer.)

- [ ] **Step 1.6: testdata**

```
pkg/plugin/testdata/
├── valid-home/
│   └── git-tools/
│       ├── plugin.yaml         # has commands, skills (1 dir), hooks (1 spec), mcpServers (1 entry)
│       ├── commands/           # (empty — yaml inline)
│       ├── skills/
│       │   └── git-flow/
│       │       └── SKILL.md
│       └── hooks/
│           └── audit.sh        # executable
├── bad-no-manifest/
│   └── ghost/                  # empty dir
├── bad-name-mismatch/
│   └── x/
│       └── plugin.yaml         # name: not-x
└── bad-skill-ref/
    └── y/
        ├── plugin.yaml         # skills.dir points to skills/nope
        └── skills/             # empty
```

`valid-home/git-tools/plugin.yaml`:

```yaml
name: git-tools
version: 0.1.0
description: branch helpers
commands:
  - name: /new-branch
    type: local-prompt
    description: Start a new feature branch
    body: |
      Create a new branch off main using the team naming convention.
skills:
  - dir: skills/git-flow
hooks:
  PreToolUse:
    - matcher: "Bash"
      command: hooks/audit.sh
      timeout: 10s
mcpServers:
  fs:
    command: /bin/echo
    args: ["mcp-fake"]
```

`skills/git-flow/SKILL.md`:

```markdown
---
name: git-flow
description: Use when starting a feature branch
---

# git-flow body
```

`hooks/audit.sh`:

```bash
#!/usr/bin/env bash
cat > /dev/null
exit 0
```

(chmod +x)

- [ ] **Step 1.7: tests**

`pkg/plugin/loader_test.go` covers each scenario above (valid; bad-no-manifest; bad-name-mismatch; bad-skill-ref). Assert Plugin's Commands/Skills/Hooks/MCPServers populated correctly for valid plugin; assert hooks command path is absolute (joined with BasePath); assert MCPServers keys are `git-tools:fs`.

`pkg/plugin/registry_test.go` covers List sorted, Reload atomic, Install (copy into a t.TempDir and re-load), Remove.

`pkg/plugin/command_test.go` covers DynamicCommand.Run for local-prompt, local, and unknown types.

- [ ] **Step 1.8: Gate + stage**

```bash
go test ./pkg/plugin/... -count=1
git add pkg/plugin/
```

---

## Task 2: `/plugin` builtin

**Files:**
- Create: `pkg/command/builtins/plugin.go`
- Create: `pkg/command/builtins/plugin_test.go`
- Modify: `pkg/command/command.go` (Host.Plugins accessor)
- Modify: `internal/tui/app.go` (App.Plugins, Options.Plugins)
- Modify: `pkg/command/builtins/builtins_test.go` (fakeHost.plugins field + method)

- [ ] **Step 2.1: Host accessor**

`pkg/command/command.go` add:
```go
import "github.com/ricardo/anthrogo/pkg/plugin"
// ...
Plugins() *plugin.Registry
```

tui Options gains `Plugins *plugin.Registry`; App has `Plugins()`.
fakeHost: add `plugins *plugin.Registry` field + `Plugins() *plugin.Registry { return f.plugins }`.

- [ ] **Step 2.2: `/plugin` builtin**

`pkg/command/builtins/plugin.go`:

```go
package builtins

import (
    "context"
    "fmt"
    "strings"

    "github.com/ricardo/anthrogo/pkg/command"
)

type Plugin struct {
    HomeRoot string
    CwdRoot  string
}

func (Plugin) Name() string        { return "/plugin" }
func (Plugin) Aliases() []string   { return nil }
func (Plugin) Description() string { return "Manage installed plugins (subcommands: info <name>, reload, install <path>, remove <name>)" }
func (Plugin) Type() command.Type  { return command.TypeLocal }

func (p Plugin) Run(ctx context.Context, args string, host command.Host) (command.Result, error) {
    reg := host.Plugins()
    if reg == nil {
        return command.Result{Text: "no plugin registry configured"}, nil
    }
    args = strings.TrimSpace(args)
    switch {
    case args == "":
        return listPlugins(reg), nil
    case args == "reload":
        warnings, err := reg.Reload(p.HomeRoot, p.CwdRoot)
        if err != nil {
            return command.Result{}, err
        }
        out := fmt.Sprintf("reloaded plugins (now %d)", len(reg.List()))
        if len(warnings) > 0 {
            out += "\nwarnings:\n" + strings.Join(warnings, "\n")
        }
        out += "\n\nnote: anthrogo must restart for command / skill / MCP-server / hook contributions to take effect at runtime; reload only refreshes the registry view."
        return command.Result{Text: out}, nil
    case strings.HasPrefix(args, "info "):
        name := strings.TrimSpace(strings.TrimPrefix(args, "info "))
        return infoPlugin(reg, name), nil
    case strings.HasPrefix(args, "install "):
        src := strings.TrimSpace(strings.TrimPrefix(args, "install "))
        plg, err := reg.Install(src, p.HomeRoot)
        if err != nil {
            return command.Result{Text: "install failed: " + err.Error()}, nil
        }
        return command.Result{Text: fmt.Sprintf("installed %s (%d commands, %d skills, %d MCP servers)", plg.Name, len(plg.Commands), len(plg.Skills), len(plg.MCPServers))}, nil
    case strings.HasPrefix(args, "remove "):
        name := strings.TrimSpace(strings.TrimPrefix(args, "remove "))
        if err := reg.Remove(name, p.HomeRoot); err != nil {
            return command.Result{Text: "remove failed: " + err.Error()}, nil
        }
        return command.Result{Text: "removed " + name}, nil
    default:
        return command.Result{Text: "usage: /plugin [info <name> | reload | install <path> | remove <name>]"}, nil
    }
}

func listPlugins(reg *plugin.Registry) command.Result {
    list := reg.List()
    if len(list) == 0 {
        return command.Result{Text: "(no plugins installed)"}
    }
    var b strings.Builder
    for _, p := range list {
        fmt.Fprintf(&b, "%-20s  v%-8s  [%s]  %s\n", p.Name, p.Manifest.Version, p.Source, p.Manifest.Description)
    }
    return command.Result{Text: b.String()}
}

func infoPlugin(reg *plugin.Registry, name string) command.Result {
    p, ok := reg.Get(name)
    if !ok {
        return command.Result{Text: "plugin not found: " + name}
    }
    var b strings.Builder
    fmt.Fprintf(&b, "%s v%s\n", p.Name, p.Manifest.Version)
    fmt.Fprintf(&b, "source:       %s\n", p.Source)
    fmt.Fprintf(&b, "base:         %s\n", p.BasePath)
    fmt.Fprintf(&b, "description:  %s\n", p.Manifest.Description)
    fmt.Fprintf(&b, "commands:     %d\n", len(p.Commands))
    fmt.Fprintf(&b, "skills:       %d\n", len(p.Skills))
    fmt.Fprintf(&b, "mcp servers:  %d\n", len(p.MCPServers))
    hookCount := len(p.Hooks.PreToolUse) + len(p.Hooks.PostToolUse) + len(p.Hooks.UserPromptSubmit) +
        len(p.Hooks.Stop) + len(p.Hooks.SubagentStop) + len(p.Hooks.Notification) +
        len(p.Hooks.PreCompact) + len(p.Hooks.SessionStart) + len(p.Hooks.SessionEnd)
    fmt.Fprintf(&b, "hooks:        %d\n", hookCount)
    return command.Result{Text: b.String()}
}
```

Add `"github.com/ricardo/anthrogo/pkg/plugin"` import.

- [ ] **Step 2.3: Tests**

Cover all 5 paths (list / info known / info unknown / reload / install / remove / unknown subcommand). Use `t.TempDir()` for install / remove paths so files are isolated.

- [ ] **Step 2.4: Gate**

```bash
go test ./pkg/command/... -count=1
git add pkg/command/
```

---

## Task 3: skill.Registry.Add + cmd/anthrogo wiring

**Files:**
- Modify: `pkg/skill/registry.go` — add `Add(s Skill)` method
- Modify: `cmd/anthrogo/main.go` — load plugins, merge into existing registries
- Modify: `internal/version/version.go`, `CHANGELOG.md`, `README.md`

- [ ] **Step 3.1: skill.Add**

```go
// Add inserts or replaces a single Skill in the registry. Used by plugin
// loader to contribute skills after initial NewRegistry.
func (r *Registry) Add(s Skill) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.skills[s.Name] = s
}
```

- [ ] **Step 3.2: main.go wiring**

After `hookMgr := ...` setup is currently positioned, **move it AFTER plugin loading** so hookMgr sees the combined hook config. New ordering in main.go:

1. Load config
2. Build perms + cwd + sess
3. Load skills (existing)
4. **Load plugins** (new)
5. **Merge plugin contributions into cfg.Hooks, skillReg, cmds, mcpMgr** (new)
6. Build hookMgr from the now-final cfg.Hooks
7. Register Skill tool
8. Register builtins (including new `builtins.Plugin{HomeRoot, CwdRoot}`)

Concrete code block after current "load skills" block:

```go
homePluginsRoot := filepath.Join(os.Getenv("HOME"), ".anthrogo", "plugins")
cwdPluginsRoot := filepath.Join(cwd, ".anthrogo", "plugins")
plugins, pwarn, _ := plugin.LoadAll(homePluginsRoot, cwdPluginsRoot)
for _, w := range pwarn {
    fmt.Fprintln(os.Stderr, "plugins:", w)
}
pluginReg := plugin.NewRegistry(plugins)

// Merge plugin contributions
for _, p := range plugins {
    for _, s := range p.Skills {
        skillReg.Add(s)
    }
    cfg.Hooks = cfg.Hooks.AppendOverlay(p.Hooks)
}
for _, w := range cfg.Hooks.Validate() {
    fmt.Fprintln(os.Stderr, "hooks (after plugins):", w)
}
```

After mcpMgr is constructed (or before), merge plugin MCP servers:

```go
for _, p := range plugins {
    for name, mcfg := range p.MCPServers {
        mcpMgr.AddServer(name, mcfg)
    }
}
```

After cmds := registerCommands(...) builds the registry, register plugin commands:

```go
for _, p := range plugins {
    for _, c := range p.Commands {
        cmds.Register(c)  // last-writer-wins; warning was already emitted in loader if duplicate
    }
}
cmds.Register(builtins.Plugin{HomeRoot: homePluginsRoot, CwdRoot: cwdPluginsRoot})
```

For TUI: pass `Plugins: pluginReg` in `tui.New(tui.Options{...})`.

Adjust file ordering — ensure plugins load is before all consumers.

- [ ] **Step 3.3: Version**

```go
var Version = "0.4.3-dev"
```

- [ ] **Step 3.4: CHANGELOG**

Prepend after `# Changelog`:

```markdown
## [0.4.3-dev] — 2026-05-20

M4.4 — Plugins (third-party content bundles).

### Added
- `pkg/plugin/` package: Plugin struct + Manifest parser + DynamicCommand + Loader + Registry.
- Layout: `~/.anthrogo/plugins/<name>/plugin.yaml` (home) + `<cwd>/.anthrogo/plugins/<name>/plugin.yaml` (project; overrides home).
- Manifest contributes 4 kinds: commands (with type local/local-prompt/submit + body), skills (by directory ref), hooks (path-resolved relative to plugin root, then merged via `hooks.Config.AppendOverlay`), mcpServers (keys namespaced `<plugin>:<name>` to avoid collisions).
- `/plugin` slash command: list, info <name>, reload, install <path>, remove <name>.
- `command.Host.Plugins()` accessor; `tui.Options.Plugins`.
- `skill.Registry.Add(Skill)` for plugin contributions.
- Loader emits warnings + skips on: missing/malformed plugin.yaml, name regex / mismatch, broken skill dir refs.

### Changed
- Startup order: plugin loading happens AFTER skills/perms/config but BEFORE `hookMgr` construction so manager sees the combined hook config.
- Plugin MCP server keys carry plugin namespace prefix.

### Known issues / deferred
- Remote install (git/npm) — M5.
- Sandbox per-plugin processes — long-term.
- Plugin reload doesn't rebuild model's system prompt, doesn't restart MCP / hook manager; restart anthrogo to surface contribution changes.
- No plugin dependency declarations / version pinning.
```

- [ ] **Step 3.5: README**

Add "Plugins" section between "Skills" and "MCP servers":

```markdown
## Plugins

A Plugin is a directory bundling one or more of: slash commands, skills, hook configurations, MCP server configurations. Install by copying into `~/.anthrogo/plugins/` (or `/plugin install <local-path>`):

```
~/.anthrogo/plugins/git-tools/
├── plugin.yaml         # manifest
├── commands/           # (inlined in plugin.yaml — directory optional)
├── skills/
│   └── git-flow/SKILL.md
└── hooks/audit.sh
```

`plugin.yaml`:

```yaml
name: git-tools
version: 0.1.0
description: Branch + PR helpers
commands:
  - name: /new-branch
    type: local-prompt
    body: |
      Start a new feature branch off main.
skills:
  - dir: skills/git-flow
hooks:
  PreToolUse:
    - matcher: Bash
      command: hooks/audit.sh
mcpServers:
  fs:
    command: npx
    args: ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
```

Project-level `<cwd>/.anthrogo/plugins/<name>/` overrides home-level plugin of the same name.

Manage with `/plugin` (list), `/plugin info <name>`, `/plugin reload`, `/plugin install <local-path>`, `/plugin remove <name>`. After install/remove anthrogo must be restarted for commands / skills / MCP-server / hook contributions to take effect at runtime.

**Trust:** Plugins execute shell commands (via hooks), spawn subprocesses (via MCP), and inject text into the model's prompt (via skills + commands). **Installing a plugin = trusting its author.** Every action still flows through anthrogo's existing permission gate, but the model's reasoning is fully influenceable by anything the plugin chooses to inject.
```

- [ ] **Step 3.6: Stage**

```bash
git add cmd/anthrogo/main.go pkg/skill/registry.go internal/version/version.go CHANGELOG.md README.md
```

---

## Task 4: Acceptance

```bash
go build ./...
go vet ./...
go test ./...
go test -race -count=2 ./pkg/plugin ./pkg/skill ./pkg/command ./pkg/command/builtins ./internal/hooks ./internal/mcp ./internal/system ./pkg/tool ./pkg/query ./pkg/permissions
for i in 1 2 3; do go clean -testcache; go test ./... 2>&1 | grep -E "FAIL|^FAIL" || echo "run $i clean"; done
make build && ./bin/anthrogo --version
```

All clean, version `anthrogo 0.4.3-dev`.

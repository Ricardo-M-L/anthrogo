# anthrogo M4.4 — Plugins (third-party content bundles)

**Status:** approved (self-authorized)
**Date:** 2026-05-20
**Predecessor:** M4.3 (`docs/superpowers/specs/2026-05-20-anthrogo-m4-3-skills-design.md`)

## 1. Goal

Port upstream's "Plugin" concept. A Plugin is a self-contained directory that bundles commands / skills / hook configurations / MCP server configurations behind one `plugin.yaml` manifest. Users install plugins by copying / symlinking directories into `~/.anthrogo/plugins/` (or invoking `/plugin install <local-path>`). On startup, anthrogo scans the plugin roots and merges each plugin's contributions into the existing registries.

## 2. Scope

**In:** local-directory plugin loading; manifest parsing; injection into 4 existing registries (commands, skills, hooks, mcpServers); `/plugin` slash command; per-plugin enable/disable via project-level overlay.

**Out:** remote install (git / npm); sandboxing of plugin code; dependency resolution; semver enforcement; plugin marketplace; versioning / upgrade workflows.

## 3. Plugin layout

```
~/.anthrogo/plugins/git-tools/
├── plugin.yaml           # required manifest
├── commands/             # optional
│   ├── new-branch.yaml
│   └── pr-summary.yaml
├── skills/               # optional
│   └── git-flow/
│       └── SKILL.md
├── hooks/                # optional
│   └── audit.sh
└── mcp/                  # optional
    └── README.md         # only docs; servers are in plugin.yaml
```

`plugin.yaml`:

```yaml
name: git-tools
version: 0.1.0
description: Branch + PR helpers
author: foo@example.com

commands:
  - name: /new-branch
    type: local-prompt
    description: Create a new feature branch
    body: |
      Create a new branch off main following the team naming convention.
      Use scripts/new-branch.sh as the reference.

skills:
  - dir: skills/git-flow          # path relative to plugin root

hooks:
  PreToolUse:
    - matcher: "Bash"
      command: hooks/audit.sh     # path relative to plugin root
      timeout: 10s

mcpServers:
  fs-helper:
    command: npx
    args: ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
```

## 4. Manifest schema

```go
type Manifest struct {
    Name        string             `yaml:"name"`
    Version     string             `yaml:"version,omitempty"`
    Description string             `yaml:"description,omitempty"`
    Author      string             `yaml:"author,omitempty"`
    Commands    []CommandSpec      `yaml:"commands,omitempty"`
    Skills      []SkillRef         `yaml:"skills,omitempty"`
    Hooks       hooks.Config       `yaml:"hooks,omitempty"`
    MCPServers  map[string]mcp.MCPServerConfig `yaml:"mcpServers,omitempty"`
}

type CommandSpec struct {
    Name        string   `yaml:"name"`
    Aliases     []string `yaml:"aliases,omitempty"`
    Description string   `yaml:"description,omitempty"`
    Type        string   `yaml:"type"` // "local" | "local-prompt" | "submit"
    Body        string   `yaml:"body"`
}

type SkillRef struct {
    Dir string `yaml:"dir"` // path relative to plugin root pointing to a skill dir
}
```

**Validation:**
- `name` must match `^[a-z][a-z0-9-]{0,63}$`; must equal directory name (warn + skip on mismatch).
- `version` optional but should look like a version if present (no enforcement in M4.4).
- `commands[*].name` must start with `/`; type must be valid command.Type; body required.
- `skills[*].dir` must be a real subdirectory of the plugin root containing SKILL.md.
- `hooks` paths resolved relative to the plugin root before merging into the global hook config.
- `mcpServers` keys prefixed with `<plugin>:` when registered to avoid collision (so plugin git-tools's `fs-helper` server becomes `git-tools:fs-helper`).

## 5. Code organization

```
pkg/plugin/
├── plugin.go          # Plugin struct + Contributions snapshot
├── manifest.go        # Manifest schema + parse
├── command.go         # DynamicCommand implementing command.Command from CommandSpec
├── loader.go          # LoadAll(homeRoot, cwdRoot) ([]Plugin, []warnings, error)
├── registry.go        # Registry + Get / List / Reload / Install(dir) / Remove(name)
├── *_test.go
└── testdata/
    ├── valid-home/
    │   └── git-tools/
    │       ├── plugin.yaml
    │       ├── commands/...
    │       ├── skills/git-flow/SKILL.md
    │       └── hooks/audit.sh
    ├── bad-no-manifest/
    │   └── ghost/
    ├── bad-name-mismatch/
    │   └── x/plugin.yaml
    └── bad-skill-ref/
        └── y/plugin.yaml   # skills.dir points nowhere
```

```go
// pkg/plugin/plugin.go
type Plugin struct {
    Name       string
    Manifest   Manifest
    BasePath   string
    Source     string  // "home" | "cwd"

    // Resolved contributions — populated by loader after path resolution.
    Commands   []command.Command   // DynamicCommands ready to register
    Skills     []skill.Skill       // already namespaced or just Name=skill name (no namespacing in M4.4)
    Hooks      hooks.Config        // paths absolutized
    MCPServers map[string]mcp.MCPServerConfig  // keys prefixed "<plugin>:<key>"
}
```

```go
// pkg/plugin/registry.go
type Registry struct {
    mu      sync.RWMutex
    plugins map[string]Plugin
}

func NewRegistry(plugins []Plugin) *Registry
func (r *Registry) Get(name string) (Plugin, bool)
func (r *Registry) List() []Plugin                // sorted
func (r *Registry) Reload(homeRoot, cwdRoot string) ([]string, error)
func (r *Registry) Install(srcDir, destRoot string) (Plugin, error)  // copy srcDir into destRoot
func (r *Registry) Remove(name string) error
```

```go
// pkg/plugin/command.go
type DynamicCommand struct {
    spec      CommandSpec
    pluginDir string  // for error messages
}

func (d DynamicCommand) Name() string         { return d.spec.Name }
func (d DynamicCommand) Aliases() []string    { return d.spec.Aliases }
func (d DynamicCommand) Description() string  { return d.spec.Description }
func (d DynamicCommand) Type() command.Type   { return command.Type(d.spec.Type) }
func (d DynamicCommand) Run(_ context.Context, args string, _ command.Host) (command.Result, error) {
    body := d.spec.Body
    if args != "" {
        body += "\n\n" + strings.TrimSpace(args)
    }
    switch command.Type(d.spec.Type) {
    case command.TypeLocalPrompt:
        return command.Result{SubmitText: body}, nil
    case command.TypeSubmit:
        return command.Result{SubmitText: body}, nil
    case command.TypeLocal:
        return command.Result{Text: body}, nil
    default:
        return command.Result{Text: "(plugin command has unknown type)"}, nil
    }
}
```

## 6. Integration points (cmd/anthrogo/main.go)

After config load + skills load + hookMgr build + mcpMgr build, plug in the plugin pipeline:

```go
homePluginsRoot := filepath.Join(os.Getenv("HOME"), ".anthrogo", "plugins")
cwdPluginsRoot := filepath.Join(cwd, ".anthrogo", "plugins")
plugins, warnings, _ := plugin.LoadAll(homePluginsRoot, cwdPluginsRoot)
for _, w := range warnings { fmt.Fprintln(os.Stderr, "plugins:", w) }
pluginReg := plugin.NewRegistry(plugins)

// Merge contributions:
for _, p := range plugins {
    for _, c := range p.Commands {
        cmds.Register(c)  // may emit "duplicate command" panic — see loader rules
    }
    for _, s := range p.Skills {
        skillReg.Add(s)  // pkg/skill.Registry gains an Add method in M4.4
    }
    cfg.Hooks = cfg.Hooks.AppendOverlay(p.Hooks)
    for n, mcfg := range p.MCPServers {
        mcpMgr.AddServer(n, mcfg)
    }
}
// Re-validate combined hooks
for _, w := range cfg.Hooks.Validate() { fmt.Fprintln(os.Stderr, "hooks (after plugins):", w) }
// hookMgr is constructed AFTER plugin merge so it sees the final cfg.Hooks
hookMgr := hooks.NewManager(cfg.Hooks, ...)
```

Order matters: plugins must merge BEFORE `hooks.NewManager` is built so manager sees the combined hooks list. Re-order main.go accordingly.

For commands: if a plugin tries to register a slash command whose name already exists in the cmds registry, the existing tool.Registry behavior should be either skip + warn, or panic. Currently `command.Registry.Register` panics on duplicate. Plugin loader catches duplicates BEFORE calling Register and emits a warning instead.

For skills: project-level skills already override home-level; plugin-contributed skills behave the same — they're added with their plugin's BasePath as the skill BasePath, but if a name conflict exists, the project-level wins, plugin loses. (Plugin-skills are second-class to user-defined skills.)

## 7. `/plugin` slash command

```
/plugin                  # list loaded plugins
/plugin info <name>      # show manifest summary
/plugin reload           # re-scan plugin roots
/plugin install <path>   # copy <path>/ into ~/.anthrogo/plugins/<name>/, then implicit reload
/plugin remove <name>    # delete ~/.anthrogo/plugins/<name>/ (with confirmation? — M4.4 = no confirm; plugin install is local-only, low-risk)
```

`/plugin install <path>` validates the source contains plugin.yaml before copying. Refuses if a same-named plugin already exists in the destination root.

`/plugin remove <name>` only removes from the *home* root (not project-level — cwd-rooted plugins are managed by the user's repo).

After install / remove, `/plugin reload` happens implicitly. Loader warnings printed.

Note: like `/skills reload`, `/plugin reload` doesn't rebuild the model's system prompt or restart the MCP servers / hook manager. So new commands/skills won't surface to the model until restart, and removed MCP servers will still be running. We print a clear warning on every reload.

## 8. Security note

Plugins execute shell commands (via hooks) and inject text into the model's prompt (via commands + skills). **Installing a plugin = trusting its author.** README + CHANGELOG must say this prominently.

The existing permission gate still applies to every action: a malicious plugin can't actually `rm -rf /` without going through Bash's gate. But:
- Plugin-contributed `local-prompt` commands run with the user's implicit "submit this prompt" gesture; if the user types `/foo`, they're consenting.
- Plugin hooks fire on every event, unsandboxed, in the user's shell.
- Plugin MCP servers spawn subprocesses that anthrogo trusts via the existing MCP stdio protocol.

Worth flagging in the README's "Trust" section.

## 9. Tests

- `pkg/plugin/loader_test.go`:
  - Valid home plugin loads with all 4 contribution types
  - cwd plugin overrides home (with warning)
  - Missing plugin.yaml → skip + warning
  - Name mismatch → skip + warning
  - skills.dir doesn't exist → skill ref ignored + warning, other parts of plugin still load
  - Plugin hooks path resolved relative to plugin root
- `pkg/plugin/registry_test.go`:
  - List sorted
  - Reload rebuilds atomically
  - Install copies dir and refuses duplicates
  - Remove deletes home-rooted plugin
- `pkg/plugin/command_test.go`:
  - DynamicCommand of type local-prompt returns SubmitText
  - DynamicCommand of type local returns Text
  - args appended to body
- `pkg/command/builtins/plugin_test.go`:
  - /plugin list / info / reload / install / remove paths

## 10. CHANGELOG / version

- Bump to `0.4.3-dev`.
- README adds a "Plugins" section between "Skills" and "MCP servers".

## 11. Acceptance

- `go build/vet/test/-race` clean
- 3× uncached full-repo sweep clean
- `./bin/anthrogo --version` → `0.4.3-dev`

## 12. Deferred

- Remote install (git / npm) — M5
- Sandbox per-plugin processes — long-term
- Plugin enable/disable toggle (current model: presence in plugins dir = enabled)
- Plugin dependency declarations
- Plugin runtime reload of MCP managers / system prompt
- Plugin marketplace / discovery

# Plugins

A Plugin bundles one or more of: slash commands, skills, hook configurations, and MCP server configurations into a single installable unit. Plugins live in `~/.anthrogo/plugins/` (home) or `<cwd>/.anthrogo/plugins/` (project-local; overrides same-named home plugin).

## Directory layout

```
~/.anthrogo/plugins/git-tools/
├── plugin.yaml         # required manifest
├── skills/
│   └── git-flow/SKILL.md
└── hooks/audit.sh
```

## plugin.yaml manifest

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

Plugin-contributed MCP server keys are prefixed with `<plugin-name>:` at runtime to prevent collisions. The `git-tools` example above surfaces tools as `mcp__git-tools:fs__read_file`. Use `/mcp` to inspect.

## Installing plugins

`/plugin install` accepts three source forms:

```
# Local path (M4.4)
/plugin install ~/my-plugins/git-tools

# HTTPS URL pointing to a .tar.gz or .zip archive (M11.3)
/plugin install https://example.com/plugins/git-tools.tar.gz
/plugin install https://example.com/plugins/git-tools.zip

# git+https:// or git+ssh:// — clones depth=1 (M11.3)
/plugin install git+https://github.com/foo/anthrogo-plugin-git.git
/plugin install git+https://github.com/foo/anthrogo-plugin-git.git@v1.0
```

For URL and git installs, `plugin.yaml` may be at the archive root **or** inside a single top-level directory (the common tarball convention). Zip-slip attacks are rejected during extraction.

## Managing plugins

```
/plugin              # list installed plugins
/plugin info <name>  # show manifest details
/plugin reload       # rescan plugins directory (hot-reload)
/plugin remove <name># remove a plugin
```

After install/remove, restart anthrogo for all contributions (commands, skills, MCP servers, hooks) to take effect at runtime. `/plugin reload` hot-reloads the skills and hook configuration but not MCP servers (those need a full restart).

## Precedence

Project-level `<cwd>/.anthrogo/plugins/<name>/` overrides a same-named home plugin. Override is exact-match on directory name.

## Trust

Plugins execute shell commands (via hooks), spawn subprocesses (via MCP servers), and inject text into the model's prompt (via skills + commands). **Installing a plugin = trusting its author.** Every action still flows through anthrogo's permission gate, but the model's reasoning is fully influenceable by anything the plugin injects. Only install plugins from HTTPS sources you trust; prefer signed archives when available.

Archives are downloaded and extracted before any validation. A future milestone will add Sigstore / GPG signature verification.

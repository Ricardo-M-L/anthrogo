# anthrogo

A Go port of Anthropic's Claude Code CLI, reconstructed from the
source-mapped `@anthropic-ai/claude-code@2.1.88` package.

> **Status**: M5.3 complete (v0.5.2-dev). Concurrent subagents, per-subagent permission isolation, and user-defined YAML types landed. See `docs/superpowers/specs/` for design docs.

## Why

The upstream CLI is TypeScript + Bun + React/Ink. `anthrogo` re-expresses the
same architecture in Go, preserving the shapes of `Tool`, `QueryEngine`,
`PermissionContext`, `ToolUseContext`, MCP client, hooks, skills, plugins —
while replacing the Ink UI with a Bubble Tea front-end.

This project is **not** a 1:1 line transliteration: feature flags become
Go build tags, Zod becomes JSON schema, React components become Bubble Tea
update/view loops.

## Roadmap

| Milestone | Scope                                                                      | Status   |
|-----------|----------------------------------------------------------------------------|----------|
| M1        | TUI REPL + 7 core tools + Anthropic SDK + permission gate + CLAUDE.md      | shipped  |
| M2        | More tools, session persistence, plan mode, slash-command palette          | shipped  |
| M3        | MCP client                                                                 | shipped  |
| M4        | Hooks, skills, plugins                                                     | shipped  |
| M5.1      | Subagents (Task tool + sub-engine, depth limit, SubagentStop hook)         | shipped  |
| M5.2      | MCP resources + minimal elicitations (decline handler)                    | shipped  |
| M5.3      | Concurrent subagents, isolated perms, user-defined YAML types              | shipped  |
| M6        | OAuth + Bedrock/Vertex + OpenAI-compat / DeepSeek / Kimi / MiniMax / GLM   | planned  |

## Repository layout

```
anthrogo/
├── cmd/anthrogo/        # CLI entry
├── internal/
│   ├── tui/             # Bubble Tea models
│   ├── headless/        # -p stdout path
│   ├── config/          # settings + paths
│   ├── system/          # system prompt + CLAUDE.md walker
│   ├── session/         # conversation state
│   └── mcp/             # MCP stdio client (Manager + Server + LogSink)
├── pkg/
│   ├── message/         # ContentBlock types
│   ├── provider/        # Provider interface + Anthropic impl
│   ├── tool/            # Tool framework + 7 built-ins
│   ├── permissions/     # PermissionContext + gate
│   └── query/           # QueryEngine — owns turn loop
└── docs/superpowers/specs/  # design docs
```

## Building

```bash
make build              # produces ./bin/anthrogo
make test               # go test ./...
./bin/anthrogo --version
```

## Running

Set `ANTHROPIC_API_KEY` in your environment, then:

```bash
./bin/anthrogo                          # interactive REPL (Bubble Tea)
./bin/anthrogo -p "explain main.go"     # headless: prints assistant text, exits
./bin/anthrogo --permission-mode acceptEdits -p "fix the typo in README"
./bin/anthrogo --model claude-haiku-4-5-20251001
./bin/anthrogo --cwd /path/to/project
```

## Configuration

Settings live in `$ANTHROGO_HOME/settings.yaml` (default: `~/.anthrogo/settings.yaml`). Example:

```yaml
mode: default
model: claude-sonnet-4-6
alwaysAllow:
  - tool: Read
  - tool: Glob
  - tool: Grep
  - tool: Bash
    match: "git status*"
  - tool: Bash
    match: "git diff*"
alwaysDeny:
  - tool: Bash
    match: "rm -rf*"
```

`CLAUDE.md` is auto-loaded by walking from cwd up to `$HOME`; merged contents are appended to the system prompt.

## MCP servers

anthrogo can spawn MCP (Model Context Protocol) servers at startup and expose their tools to the model. Add to `~/.anthrogo/settings.yaml`:

```yaml
mcpServers:
  filesystem:
    command: npx
    args: ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  remote-fetch:
    type: streamable
    endpoint: https://example.com/mcp
    max_retries: 3
  legacy-sse:
    type: sse
    endpoint: https://legacy.example.com/mcp
```

`type` defaults to `stdio`. Other values: `sse` (2024-11-05 SSE), `streamable` (newer streamable HTTP).

Tools surface as `mcp__<server>__<tool>` (names exceeding 64 chars get a sha-8 suffix). Inspect status with `/mcp`, view one server's last error with `/mcp status <name>`, restart all servers with `/mcp reload` (removes and re-registers all `mcp__*` tools; the model's system prompt is still built at startup, so restart anthrogo to refresh model awareness of newly-added tools). Server log notifications render dim-styled in the TUI; in headless they go to stderr.

**Plan mode blocks all MCP tool calls** (`mcp__*` tools are treated as write tools). Switch to default mode (`/mode default`) to invoke MCP tools.

> anthrogo registers list-changed handlers for tools and resources. When a server pushes `notifications/tools/list_changed`, anthrogo refreshes its per-server tool cache and logs the event. The model-facing tool registry is NOT auto-rebuilt (would race with in-flight turns); run `/mcp reload` to surface new tools to the next system prompt.

### MCP resources

anthrogo lists resources advertised by Ready servers in the system prompt at startup and provides a built-in `MCPResource` tool the model can use to list or read them:

- **List resources on a server:** `{server: "filesystem"}` — returns a JSON array of `{uri, name, description, mime_type, size}`.
- **Read a resource:** `{server: "filesystem", uri: "file:///tmp/notes.md"}` — returns the resource text (or a blob summary for binary content).

The `MCPResource` tool has a default `alwaysAllow` rule at the CLI level (read-only; deny rules and `PreToolUse` hooks still take precedence).

### Elicitations

When an MCP server sends an `elicitation/create` request, anthrogo records it via the log sink and returns `Action: "decline"`. This advertises the elicitation capability so servers know anthrogo is reachable. To opt out entirely (suppress the capability advertisement), set `elicitation_mode: "disabled"` on the server config:

```yaml
mcpServers:
  filesystem:
    command: npx
    args: ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
    elicitation_mode: "disabled"   # don't advertise elicitation capability
```

Full TUI form-input elicitation handling defers to M5.3.

WebSocket transport and OAuth 2.1 client flow are deferred to M5.3.

## Hooks

anthrogo runs user-defined shell commands at 9 lifecycle events. Add to `~/.anthrogo/settings.yaml`:

```yaml
hooks:
  PreToolUse:
    - matcher: "Bash"
      command: ~/.anthrogo/hooks/audit.sh
      timeout: 30s
    - matcher: "Write|Edit|NotebookEdit"
      command: ~/.anthrogo/hooks/protect-secrets.sh
  PostToolUse:
    - matcher: "Write|Edit"
      command: ~/.anthrogo/hooks/gofmt.sh
  UserPromptSubmit:
    - command: ~/.anthrogo/hooks/inject-cwd.sh
  Stop:
    - command: ~/.anthrogo/hooks/notify-slack.sh
```

Each hook gets one JSON object on stdin describing the event. Exit code 2 blocks the action (PreToolUse → deny, UserPromptSubmit → abort prompt). Exit code 0 + JSON on stdout can `permissionDecision: "allow"|"deny"`, `modifiedInput: {...}` (PreToolUse only), or `additionalContext: "..."` (UserPromptSubmit / PostToolUse).

`matcher` is a Go regexp against the tool name (PreToolUse / PostToolUse only). Project-level `.anthrogo/hooks.yaml` appends to home-level `hooks:` block.

anthrogo doesn't auto-provision the `~/.anthrogo/hooks/` directory — create it and your hook scripts yourself, then `chmod +x` them.

Default timeouts: 30s for sync events, 5–10s for async. Async events (Stop / Notification / Session*) fire on a background goroutine.

Plan-mode hard-lock still overrides hook-allow for write tools.

`PreCompact` fires synchronously before `/compact` runs (M4.2).

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

**Trust:** the body of a SKILL.md becomes part of the prompt sent to the model when invoked. A malicious skill can instruct the model to leak data, exfiltrate files, or trigger side effects — though every action still flows through anthrogo's tool permission gate. Only install skills from sources you trust.

## Plugins

A Plugin is a directory bundling one or more of: slash commands, skills, hook configurations, MCP server configurations. Install by copying into `~/.anthrogo/plugins/` or via `/plugin install <local-path>`:

```
~/.anthrogo/plugins/git-tools/
├── plugin.yaml         # required manifest
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

> Plugin-contributed MCP server keys are prefixed with `<plugin-name>:` at runtime to prevent collisions. So `git-tools`'s `fs:` server surfaces as tools like `mcp__git-tools:fs__read_file`. Use `/mcp` to inspect.

Project-level `<cwd>/.anthrogo/plugins/<name>/` overrides a same-named home plugin.

Manage with `/plugin` (list), `/plugin info <name>`, `/plugin reload`, `/plugin install <local-path>`, `/plugin remove <name>`. After install/remove anthrogo must be restarted for commands / skills / MCP-server / hook contributions to take effect at runtime.

**Trust:** Plugins execute shell commands (via hooks), spawn subprocesses (via MCP), and inject text into the model's prompt (via skills + commands). **Installing a plugin = trusting its author.** Every action still flows through anthrogo's existing permission gate, but the model's reasoning is fully influenceable by anything the plugin injects.

## Subagents

The model can spawn isolated sub-engines via the `Task` tool to perform self-contained multi-step tasks. Unlike skill invocations (which just return markdown), a subagent runs its own tool-use loop and returns its final answer as a tool result.

```
Task({
  "description": "find all TODO comments",
  "prompt":       "Search the codebase for TODO comments. Return a list.",
  "subagent_type": "general-purpose"
})
```

The subagent has no memory of the parent conversation — brief it fully in `prompt`. It inherits the parent's tools (unless the subagent type restricts them via `ToolAllowlist`), permission gate, and hook manager.

**Concurrent dispatch (M5.3):** when the model emits multiple `Task` tool_use blocks in a single turn, the engine runs them concurrently. Tool_result order is preserved. Log/stderr output from concurrent subagents may interleave.

**Permission isolation (M5.3):** each subagent runs with a cloned `permissions.Context`. Mode toggles (e.g. the model entering plan mode inside a subagent) do not leak back to the parent.

**Recursion limit:** nested subagents are allowed up to depth 3 by default (`MaxSubagentDepth`). Calls beyond the limit return an error to the model.

**Plan mode:** `Task` is treated as a write tool, so plan mode blocks it. Switch to default mode (`/mode default`) to invoke subagents.

**SubagentStop hook:** fires after every subagent completes (success or error). Wire it in `hooks.yaml` under `SubagentStop:`.

### Custom subagent types

Drop YAML files into `~/.anthrogo/subagents/` (home, all projects) or `<cwd>/.anthrogo/subagents/` (project-local; overrides home) to define your own types:

```yaml
# ~/.anthrogo/subagents/code-reviewer.yaml
name: code-reviewer
description: Use when reviewing a PR or code change for correctness and style.
system_prompt_suffix: |
  You are a code reviewer. Be specific. Cite file:line. Suggest concrete fixes.
tool_allowlist:
  - Read
  - Grep
  - Glob
  - Bash
```

Rules:
- `name` must be lowercase alphanumeric + hyphens (`^[a-z][a-z0-9-]{0,63}$`) and match the filename stem.
- `description` is required (shown to the model in the Task tool schema).
- `system_prompt_suffix` is optional extra instruction appended to the system prompt for this subagent.
- `tool_allowlist` is optional. Empty = inherit parent's full tool registry.
- The name `general-purpose` is reserved and cannot be overridden.

Use `/subagents` to list loaded types, `/subagents show <name>` to inspect, `/subagents reload` to hot-reload without restarting (note: the system prompt is built at startup, so newly added types won't be advertised to the model until restart).

## Compaction

For long sessions, `/compact` summarizes earlier turns to cut token cost:

```
/compact            # keeps the 10 most-recent messages, summarizes the rest
/compact --keep 20  # keeps 20 most-recent
```

Currently all earlier messages including MCP tool calls are summarized to prose; pair-preserving compaction is a future milestone. `PreCompact` hooks (configured under `hooks.PreCompact`) fire before each compact.

## Tools (M1)

| Tool        | Read-only | What it does                                                       |
|-------------|-----------|--------------------------------------------------------------------|
| Read        | yes       | Read a file with offset/limit, cat-n style line numbers            |
| Write       | no        | Write content to a file path, creating parent dirs                 |
| Edit        | no        | Replace `old_string` with `new_string`; unique match unless `replace_all` |
| Glob        | yes       | doublestar glob, results sorted newest-first by mtime              |
| Grep        | yes       | Go regexp recursive search with `output_mode` and glob filter      |
| Bash        | no        | Run a shell command with `timeout_ms` (default 120000)             |
| TodoWrite   | no        | Maintain a replace-on-write task list                              |

## Permission model

For every tool call the engine consults `permissions.Decide`:

1. `bypassPermissions` mode → allow.
2. `alwaysDeny` rules → deny.
3. `acceptEdits` mode → allow Write/Edit/NotebookEdit by default.
4. `alwaysAllow` rules → allow.
5. `alwaysAsk` rules → ask (prompt the user).
6. Otherwise → ask (TUI shows a modal) / deny (headless when `ShouldAvoidPrompts`).

## License

Source code Anthropic-attributed in the reference repo. This port is for
research and personal use; do not redistribute commercially.

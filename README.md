# anthrogo

A Go port of Anthropic's Claude Code CLI, reconstructed from the
source-mapped `@anthropic-ai/claude-code@2.1.88` package.

> **Status**: M4.1 complete (v0.4.0-dev). Hooks (9 event types) landed. See `docs/superpowers/specs/` for design docs and `docs/superpowers/plans/` for implementation plans.

## Why

The upstream CLI is TypeScript + Bun + React/Ink. `anthrogo` re-expresses the
same architecture in Go, preserving the shapes of `Tool`, `QueryEngine`,
`PermissionContext`, `ToolUseContext`, MCP client, hooks, skills, plugins —
while replacing the Ink UI with a Bubble Tea front-end.

This project is **not** a 1:1 line transliteration: feature flags become
Go build tags, Zod becomes JSON schema, React components become Bubble Tea
update/view loops.

## Roadmap

| Milestone | Scope                                                                      |
|-----------|----------------------------------------------------------------------------|
| M1        | TUI REPL + 7 core tools + Anthropic SDK + permission gate + CLAUDE.md      |
| M2        | More tools, session persistence, plan mode, slash-command palette          |
| M3        | MCP client                                                                 |
| M4        | Hooks, skills, plugins                                                     |
| M5        | Subagents, KAIROS / coordinator, remote sessions                           |
| M6        | OAuth + Bedrock/Vertex + OpenAI-compat / DeepSeek / Kimi / MiniMax / GLM   |

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
  fetch:
    command: npx
    args: ["-y", "@modelcontextprotocol/server-fetch"]
```

Tools surface as `mcp__<server>__<tool>` (names exceeding 64 chars get a sha-8 suffix). Inspect status with `/mcp`, view one server's last error with `/mcp status <name>`, restart all servers with `/mcp reload` (note: tool registry is refreshed at startup only; reloaded servers' tool list changes won't surface until restart — fixed in M4). Server log notifications render dim-styled in the TUI; in headless they go to stderr.

Stdio-spawned servers only. SSE / WebSocket / OAuth / elicitations / resources are deferred to M4–M5.

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

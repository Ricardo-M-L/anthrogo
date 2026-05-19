# Changelog

## [0.3.0-dev] — 2026-05-19

M3 — MCP stdio support.

### Added
- `internal/mcp` package wrapping `github.com/modelcontextprotocol/go-sdk@v1.6.0`.
- `mcpServers:` stanza in `settings.yaml` to spawn N stdio MCP servers at startup.
- Per-server lifecycle: spawn → `initialize` → `tools/list` → ready, with parallel spawn and per-server 10s default timeout.
- `pkg/tool.MCPAdapter` exposes each MCP tool as `mcp__<server>__<tool>` (with sha-8 suffix on truncation, 16-char server / 41-char tool budget, 64-char total limit).
- `Manager` / `Server` / `LogSink` primitives in `internal/mcp/`.
- `/mcp`, `/mcp reload`, `/mcp status <name>` slash commands.
- TUI dim-styled MCP log lines (`[mcp:<server>] <msg>`).
- Embedded fake MCP server at `internal/mcp/testdata/echo-server/` used by integration tests.

### Changed
- `command.Host` gains `MCP() *mcp.Manager`.
- `tui.Options` gains `MCP *mcp.Manager`.
- System prompt notes the `mcp__` prefix when external MCP tools are present.

### Known issues / deferred
- Stdio only (SSE / WebSocket in M4).
- No OAuth (M4).
- No elicitations (M4).
- No `resources/list` / `resources/read` (M5).
- `/compact` still a placeholder (M4 alongside MCP-aware history compaction).
- Plan-mode `IsWriteTool` doesn't classify MCP tools — users must add explicit deny rules for `mcp__*` writes if they need to lock writes through plan mode.
- `/mcp reload` restarts subprocesses but does not refresh the tool registry; tool list changes from reloaded servers don't surface until anthrogo restart.

## [0.2.0-dev] — 2026-05-17

M2 — runnable, resumable, planning-aware.

### Added
- Mid-turn cancel: `Ctrl+C` in the REPL aborts the in-flight turn cleanly instead of exiting the process; idle `Ctrl+C` still quits.
- `Engine.Denials()` tracks every permission denial during a conversation (mirrors upstream `SDKPermissionDenial`).
- `Tool.Permission(ctx, input) permissions.Decision` interface method + `DefaultPermission` embed for tools that defer to the gate.
- `go:embed`-backed verbatim core system prompt + plan-mode addendum at `internal/system/prompts/*.txt`.
- Deterministic Grep output in `count` mode (sorted by file path).
- **JSONL session persistence** at `~/.anthrogo/projects/<cwd-hash>/<uuid>.jsonl` with first-line `session_meta`. `Engine.Config.RecordHook` decouples the engine from `internal/session`.
  - `--resume <id-prefix>` resumes a session by unique prefix.
  - `--continue` resumes the most-recent session for the current cwd.
- 4 new tools:
  - `WebFetch` — HTTP GET + HTML→markdown + 32-entry/15-min LRU cache.
  - `WebSearch` — Brave Search backend (settings.yaml: `webSearch.{backend,apiKey,endpoint}`).
  - `AskUserQuestion` — 2-4 multi-choice prompt routed through the TUI permission modal; errors out in headless.
  - `NotebookEdit` — Jupyter nbformat v4 cell ops (replace / insert_before / insert_after / delete).
- **Hard-lock plan mode**: `EnterPlanMode` / `ExitPlanMode` tools flip `Permissions.Mode`; the gate denies Write/Edit/NotebookEdit and any non-read-only Bash while active.
- TUI plan-mode banner badge + plan-mode addendum injected into the system prompt.
- `pkg/command` framework + 10 built-in slash commands: `/help`, `/tools`, `/memory`, `/cwd`, `/add-dir`, `/clear`, `/compact` (placeholder), `/resume`, `/model`, `/mode`.
- TUI slash palette overlay (Tab/Shift+Tab/Esc, fuzzy match).
- `internal/headless.Options` and `tui.Options` accept `InitialMessages` and `RecordHook` for resume + persistent journaling.

### Changed
- `pkg/tool.PromptRequest` gains `Kind PromptKind` and question fields (`Question`, `Options []PromptOption`).
- `pkg/tool.PromptResponse` gains `SelectedLabel` and `Notes`.
- TUI permission modal handles both tool-permission and question-kind prompts (`y/a/n` vs `1-4`).
- `internal/config.Config` gains `WebSearch WebSearchConfig`.
- `cmd/anthrogo` registers 13 tools (was 7) and 10 commands.

### Known issues / deferred
- `/compact` is a placeholder; real compaction lands in M3 (alongside MCP).
- The plan-mode read-only Bash heuristic is a fixed allowlist; full BashSecurity rewrite lands in M5.
- WebFetch's html-to-markdown is best-effort; JS-heavy pages fall back to raw HTML.
- `/clear`'s in-memory reset is a stub: it clears the engine messages but does not rotate to a new JSONL file yet (M3 alongside compact).
- Headless mode still ignores `KindUsage` events (token accounting only surfaces in REPL via the chat viewport).

## [0.1.0-dev] — 2026-05-17

M1 — first runnable slice.

### Added
- `cmd/anthrogo` CLI entry with `-p/--print` headless mode and Bubble Tea REPL.
- 7 built-in tools: `Bash`, `Read`, `Write`, `Edit`, `Glob`, `Grep`, `TodoWrite`.
- `pkg/provider` Provider abstraction + `anthropic` SDK-backed implementation (live smoke test off by default).
- `pkg/provider/fake` scripted Provider used by engine + headless + TUI tests.
- `pkg/permissions` Mode + Rule matcher + Context + Gate with `deny > allow > ask` precedence; `acceptEdits` and `bypassPermissions` modes.
- `pkg/message` ContentBlock union (text / tool_use / tool_result / image / thinking) with JSON round-trip.
- `pkg/query.Engine` turn loop: stream → tool_use → permission gate → exec → continue, with `SubmitMessage(ctx, prompt) <-chan Event`.
- `internal/system`: CLAUDE.md walker (cwd → `$HOME`, root-first concat), `GitStatusSnapshot`, `BuildSystemPrompt`.
- `internal/config`: `~/.anthrogo/settings.yaml` loader → `permissions.Context`.
- `internal/session`: in-memory `Store` (UUID-tagged).
- `internal/headless.Run`: machine-parseable stdout, tool/error diagnostics on stderr.
- `internal/tui`: Bubble Tea root App with chat viewport, prompt input, permission modal, theme.
- Settings DSL: YAML rules with `tool` + optional `match` (glob for path-like inputs, prefix for `command`).

### Known issues / deferred
- Anthropic SDK is alpha; `provider/anthropic` adapter pins `v0.2.0-alpha.13`.
- `Ctrl+C` in REPL exits the process; mid-turn cancel while staying in the REPL is M2.
- `bash` tool has no sandbox / no AST security scan (M5).
- No MCP, no plugins, no skills, no hooks, no subagents, no Bedrock/Vertex, no OAuth (M3-M6).

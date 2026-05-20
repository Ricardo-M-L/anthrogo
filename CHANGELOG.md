# Changelog

## [0.4.4-dev] — 2026-05-20

M4.5 — MCP debt sweep (3 of 7 deferred items).

### Added
- `tool.Registry.RemoveByPrefix(prefix string) int` — removes every tool whose name starts with prefix; returns count.
- `MCPServerConfig.Type` — defaults to `"stdio"`. New values: `"sse"` (2024-11-05 SSE protocol, uses `Endpoint`), `"streamable"` (newer streamable HTTP, uses `Endpoint` + optional `MaxRetries`).
- `Server.Start` picks transport based on `Type`: `*sdk.CommandTransport`, `*sdk.SSEClientTransport`, or `*sdk.StreamableClientTransport`. Validation: stdio requires `command`, sse/streamable require `endpoint`; bad combinations → `StateFailed` with a clear error.

### Changed
- `/mcp reload` now actually re-registers MCP tools: removes every `mcp__*` from `tool.Registry` then registers from `mgr.AllTools()`. (The model's system prompt is still built at startup; restart to refresh model awareness — `/mcp reload`'s response message mentions this.)
- `permissions.IsWriteTool` now returns true for any tool name starting with `mcp__`. **Plan mode therefore blocks every MCP tool call by default.** To use a specific MCP tool while planning, exit plan mode (`/mode default`).

### Known issues / deferred (M5)
- WebSocket transport (no native SDK support yet)
- OAuth 2.1 client flow for HTTP transports
- Elicitations (server-initiated user prompts via MCP)
- Resources (`resources/list`, `resources/read`)

## [0.4.3-dev] — 2026-05-20

M4.4 — Plugins (third-party content bundles).

### Added
- `pkg/plugin/` package: Plugin struct + Manifest parser + DynamicCommand + Loader + Registry.
- Layout: `~/.anthrogo/plugins/<name>/plugin.yaml` (home) + `<cwd>/.anthrogo/plugins/<name>/plugin.yaml` (project; overrides home).
- Manifest contributes 4 kinds: commands (type local/local-prompt/submit + body), skills (by directory ref), hooks (path-resolved relative to plugin root, then merged via `hooks.Config.AppendOverlay`), mcpServers (keys namespaced `<plugin>:<name>` to avoid collisions).
- `/plugin` slash command: list, info <name>, reload, install <path>, remove <name>.
- `command.Host.Plugins() any` accessor (typed as `any` to break import cycle; callers type-assert to `*plugin.Registry`); `tui.Options.Plugins`.
- `skill.Registry.Add(Skill)` for plugin skill contributions.
- Loader emits warnings + skips on: missing/malformed plugin.yaml, name regex / mismatch, broken skill dir refs.

### Changed
- Startup order: plugin loading happens AFTER skills/perms/config but BEFORE `hookMgr` construction so manager sees the combined hook config.
- Plugin MCP server keys carry plugin namespace prefix (`<plugin>:<key>`).

### Known issues / deferred
- Remote install (git/npm) — M5.
- Sandbox per-plugin processes — long-term.
- Plugin reload doesn't rebuild model's system prompt, doesn't restart MCP / hook manager; restart anthrogo to surface contribution changes.
- No plugin dependency declarations / version pinning.

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

## [0.4.1-dev] — 2026-05-20

M4.2 — Real `/compact` (MCP-aware history compaction).

### Added
- `pkg/compact/` package: pure summarization via existing provider.Provider.
- `query.Engine.Compact(ctx, opts) (Summary, error)` — fires PreCompact hook, calls compact.Run, swaps messages.
- `/compact` now actually summarizes earlier turns (default keep 10 most-recent; `--keep N` to override).
- Compaction algorithm uses assistant-boundary split: `tail` always begins with an assistant message, producing a valid Anthropic API conversation (`[summary_user, tail...]`).
- Session JSONL gains `compact` record kind; replay discards messages before the latest compact.

### Changed
- `query.HookSink` interface gains `FirePreCompact(ctx, trigger)`.
- `tui.PromptHookSink` and `headless.PromptHookSink` extended to match.

### Known issues / deferred
- Auto-compact on token threshold (M5).
- Byte-count proxy used instead of real tokenizer (M6 with multi-provider).
- Older compacts pre-replay (before the latest compact) are not preserved on resume — `--resume` always rebuilds from the most recent compact forward.
- MCP-aware preservation (carrying `mcp__*` tool_use/tool_result pairs through compaction) was descoped from M4.2 due to Anthropic API conversation-validity constraints around orphan tool_use blocks; the summary now covers MCP turns as prose. Real preservation lands in a later milestone.

## [0.4.0-dev] — 2026-05-20

M4.1 — Hooks (9 event types: PreToolUse / PostToolUse / UserPromptSubmit / Stop / SubagentStop / Notification / PreCompact / SessionStart / SessionEnd).

### Added
- `internal/hooks/` package: Config, Event payloads, Runner, Manager, Decision.
- `hooks:` YAML stanza in `settings.yaml` — per-event lists with `matcher` (Go regexp), `command`, and `timeout`.
- JSON-over-stdin / JSON-on-stdout / exit-code-2-blocks protocol matching upstream claude-code@2.1.88.
- Permission gate consults `PreToolUse` hooks before rule lookup; hooks can allow / deny / mutate input.
- Plan-mode hard-lock still overrides hook-allow for write tools.
- `PostToolUse` hooks can append `additionalContext` to tool_result text.
- `UserPromptSubmit` hooks can inject context or abort the prompt (exit 2).
- Async fire-and-forget for `Stop` / `SubagentStop` / `Notification` / `SessionStart` / `SessionEnd`.
- Sync but log-only `PreCompact` (M4.2 wires real /compact).
- TUI dim-styled `[hook:<event>] <msg>` log lines via a separate atomic.Pointer[*App] rail.
- chat AppendServerLog / AppendHookLog concurrent-safe regression tests.
- `Server.Start` state-reset regression test (covers `/mcp reload` re-Start).

### Changed
- `permissions.Context` gains `HookDecide func(toolName, input) HookOutcome` (nil-safe).
- `permissions.Decision` gains `ModifiedInput map[string]any`.
- `query.Config` gains `Hooks query.HookSink` interface (FirePostToolUse, FireStop).
- `tui.Options` and `headless.Options` gain `Hooks PromptHookSink` (6-method superset; same value satisfies both).
- Async hook Fire methods gained a `ctx context.Context` first parameter for interface uniformity.

### Known issues / deferred
- `SubagentStop` payload defined but never fires (no subagents until M5).
- `PreCompact` wired but `/compact` itself is still a placeholder (M4.2 lands real compaction).
- Hook subprocesses run unsandboxed in the user's privilege; sandbox lands with M5 plugins.

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

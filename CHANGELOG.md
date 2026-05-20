# Changelog

## [0.7.9-dev] — 2026-05-20

M7.10 — TUI markdown rendering (glamour).

### Added
- Assistant message text passes through glamour `TermRenderer` on `finishAssistant`. Auto-style picks dark/light from the terminal; code fences, headers, lists, bold/italic, blockquotes, and inline code render properly in the TUI chat viewport.
- During streaming, deltas accumulate as plain text under the "assistant > " prefix; the markdown render happens once at end-of-turn.
- User messages and tool output stay plain (no glamour) — user input is literal, tool output is usually compact JSON/text.
- Backing `chat.lines` reshaped from `[]string` to `[]chatLine{rendered, rawText}` so the streaming-then-finalize re-render can recover the source markdown.

### Known issues / deferred
- No mid-stream re-render (would flicker; one-shot at turn end is the right tradeoff).
- Glamour's word-wrap disabled; bubbletea viewport handles vertical scroll, horizontal overflow may clip on very narrow terminals.
- Tool output stays plain — adding glamour there would slow down repeated short outputs.

## [0.7.8-dev] — 2026-05-20

M7.9 — /sessions export markdown.

### Added
- `/sessions export <id-prefix> [-o file.md]` — renders the matched session JSONL as readable markdown. Per-turn headings (### 👤 User / 🤖 Assistant), per-tool subheadings (#### 🔧 Tool: <name>) with JSON-formatted inputs and code-fenced outputs, compact + subagent + error events as blockquotes. Without `-o`, prints to stdout; with `-o`, writes to file (0o644) and returns "exported <path> (<N> bytes)".
- Soft code-detection heuristic on tool result text (looksLikeCode): wraps in ``` if multi-line and starts with structural chars or contains common keywords; otherwise plain.
- Image blocks rendered as `_[image: <mime>, <N> base64 bytes]_`.

### Known issues / deferred
- No language-specific fence (always plain ```).
- Image data base64 not inlined (would bloat the markdown); future could write images to a sidecar dir.
- No HTML escaping inside content (markdown can mangle if user wrote raw <html>).

## [0.7.7-dev] — 2026-05-20

M7.8 — Image / vision blocks (OpenAI + prompt syntax).

### Added
- `message.ParseUserPrompt(prompt) ([]Block, error)` — recognizes `@image:<path>` tokens anywhere in the user's prompt; loads the file, base64-encodes it, validates MIME (image/png|jpeg|gif|webp), and emits a BlockImage at the same position. Surrounding text becomes BlockText.
- `pkg/provider/openai` now emits multimodal content arrays (`[{type:"text"}, {type:"image_url", image_url:{url: "data:<mime>;base64,..."}}]`) for any message containing an image block. String content path is preserved for text-only messages (no change to existing wire format).
- `Engine.SubmitMessageBlocks(ctx, []message.Block) <-chan Event` — new sibling to `SubmitMessage`; the string variant now delegates to it.
- TUI + headless route user prompts through `ParseUserPrompt` before submission.

### Usage

```
@image:./screenshot.png what's wrong with this UI?
look at @image:/tmp/diagram.png and explain the flow
```

### Known issues / deferred
- Only file paths (no URLs / data URIs in prompt syntax).
- Doesn't support image-only assistant responses or tool images.
- Tool-result content stays string-only (OpenAI doesn't support image content in tool messages).
- No image dimensions / detail level control via prompt syntax (use config later).
- Anthropic provider already supported BlockImage from M1 — no change.

## [0.7.6-dev] — 2026-05-20

M7.7 — /sessions delete.

### Added
- `/sessions delete <id-prefix>` — destructive subcommand. Default is **dry-run**: prints the JSONL path + size, the subagents subdirectory (if any) with file count + total bytes, and instructions to re-run with `--yes`. Adding `--yes` (`/sessions delete --yes <prefix>`) actually removes both the JSONL and the matching `<session-id>/subagents/` directory tree.
- Updated usage message to include the new subcommand.

### Known issues / deferred
- No undo / recycle bin. `--yes` is irreversible.
- Doesn't update any in-memory engine that might be replaying the just-deleted session (rare — typically you delete OLD sessions, not the current one).

## [0.7.5-dev] — 2026-05-20

M7.6 — /sessions replay + search.

### Added
- `/sessions replay <id-prefix>` — renders the matched session JSONL as a one-line-per-record timeline. Covers all 10 record kinds (meta, user, asst, tool, result, compact, subagent, usage, turn-end, error). Text blocks truncated to 200 chars (100 for tool results); newlines collapsed.
- `/sessions search <keyword>` — case-insensitive substring search across every .jsonl in the current cwd's session directory. Returns one line per match showing session id, record kind, and 80 chars of context (40 before + 40 after). Caps at 200 matches.

### Known issues / deferred
- Search is full-file scan; large session histories may be slow. No index.
- No regex syntax in search (substring only).
- Search doesn't recurse into subagent subdirectories.
- Replay doesn't render images, thinking blocks, or raw JSON inputs in full; first-N-chars truncation only.
- No /sessions delete yet.

## [0.7.4-dev] — 2026-05-20

M7.5 — Budget hard caps + /sessions list.

### Added
- `Config.CostLimitUSD float64` (YAML `cost_limit_usd`) + CLI flag `--cost-limit <USD>` — when set AND pricing is configured, anthrogo denies tool calls once the cumulative estimated session cost equals or exceeds the limit. Deny reason includes current cost and the limit.
- Budget enforcement piggybacks on `permissions.Context.HookDecide` — registered as a wrapper around M4.1's existing chain, so user-defined PreToolUse hooks still get to run after the budget check.
- `query.Engine.IsOverBudget() (over bool, current, limit float64)` accessor.
- `/sessions` slash command — `list` (default, sorted newest-first) and `show <id-prefix>` (unambiguous prefix match) for the current cwd's JSONLs.

### Known issues / deferred
- Budget only enforces on tool dispatch, not on assistant text generation cost; a high-cost text-only turn can push past the limit before the next tool runs.
- `/sessions show` only shows metadata, not content. Future milestones add `/sessions replay <id>` and `/sessions search <kw>`.
- Subagent JSONLs (nested under `<session>/subagents/`) are not listed by `/sessions list`.
- No automatic budget reset on `/compact` or new session — set new `--cost-limit` manually after compact if needed.

## [0.7.3-dev] — 2026-05-20

M7.4 — Cost tracking + /cost builtin + pricing config.

### Added
- `pkg/pricing/` package: Table{} with exact + glob lookup for model→Rate; EstimateUSD(rate, inputTokens, outputTokens) helper.
- `Config.Pricing map[string]Pricing` YAML stanza — user supplies per-model `input_per_m` / `output_per_m` USD rates (per million tokens). Keys can be exact model names or globs ("claude-haiku-*").
- `query.Config.Pricing *pricing.Table` — wired through tui + headless options.
- `query.Engine.EstimatedCost() (usd float64, ok bool)` — returns the estimated USD cost of cumulative session usage at the matching model's rate.
- `/cost` slash command — prints session usage + estimated USD.
- TUI status line appends `$<USD>` when pricing is configured and matches.

### Known issues / deferred
- No budget alerts / hard caps. Hitting a high cost still proceeds.
- No per-subagent cost breakdown.
- Glob matching uses filepath.Match (POSIX-style); ** wildcards not supported.
- No automatic price updates — user maintains the table.

## [0.7.2-dev] — 2026-05-20

M7.3 — Cumulative token tracking + /usage builtin.

### Added
- `query.Engine.UsageSinceLastCompact() message.Usage` — usage accumulated since the start of the session OR since the most recent successful Compact, whichever is later. Updated under lock on every EventUsage. Reset by Compact when Skipped==false.
- `/usage` slash command — prints session totals, since-last-compact totals, and (if enabled) the auto-compact threshold + tokens-until-trigger.
- `query.Engine.AutoCompactConfig() (threshold, keep int)` accessor for builtins.
- TUI status line now shows `tok: <Xin>in/<Yout>out (since: <Z>) [⚙ <N>]` — `since` is post-compact accumulation, `⚙` symbol denotes the auto-compact threshold.

### Changed
- Auto-compact threshold now uses cumulative `usageSinceLastCompact.InputTokens + .OutputTokens` instead of just the latest turn's usage. Closes M7.2's known issue.

### Known issues / deferred
- Token count is still provider-reported (no client-side precise tokenizer).
- Compact() called manually via `/compact` also resets usageSinceLastCompact (intended).
- usageSinceLastCompact resets only on successful Compact; if Compact is skipped (e.g. too few messages), the counter keeps accumulating.

## [0.7.1-dev] — 2026-05-20

M7.2 — Automatic /compact on token threshold.

### Added
- `query.Engine` tracks `LastUsage()` from streaming EventUsage events.
- `query.Config.AutoCompactThreshold` (0 = disabled) — when set, at the end of a turn the engine checks whether `lastUsage.InputTokens + lastUsage.OutputTokens >= threshold` and if so synchronously fires Compact() before returning control to the caller. Next user prompt sees the compacted history.
- `query.Config.AutoCompactKeepRecent` overrides the default KeepRecent (10) for auto-fires.
- YAML `auto_compact_threshold` + `auto_compact_keep_recent` settings.
- CLI flag `--auto-compact <N>` overrides the YAML value.
- TUI status line shows current token usage (`tokens: <in> in / <out> out [auto-compact at N]`).

### Known issues / deferred
- Threshold uses the LATEST turn's usage, not cumulative across the session. A single huge turn beyond threshold triggers compact; smaller turns that aggregate to large context do not. Cumulative tracking lands in a later milestone.
- Byte-count proxy is still used inside compact.Run's ApproxBytes; the threshold uses real EventUsage tokens. No precise tokenizer (M7+).
- Auto-compact for subagents not implemented (child engine's auto-compact is disabled regardless of parent's setting — would lose subagent context).

## [0.7.0-dev] — 2026-05-20

M7.1 — OpenAI-compatible provider (DeepSeek/Kimi/MiniMax/GLM).

### Added
- `pkg/provider/openai/` — generic OpenAI Chat Completions provider. Direct HTTP+SSE (no client SDK dep). Translates OpenAI stream chunks to anthrogo's EventTextDelta / EventToolUseStart / EventToolInputDelta / EventBlockStop / EventMessageStop / EventUsage. Maps finish_reason: stop→end_turn, tool_calls→tool_use, length→max_tokens.
- `Config.Profiles map[string]Profile{Type, BaseURL, Model, APIKey}` — user defines named provider profiles in settings.yaml. API key supports `env:VARNAME` syntax.
- `Config.Provider` field + `--provider <name>` CLI flag select the active profile. Default still "anthropic" (uses the existing Anthropic SDK).
- Profile's Model overrides Config.Model when active.

### Known issues / deferred
- Anthropic-style thinking blocks and image blocks are silently dropped when sent to OpenAI-compat endpoints (M7+).
- No automatic provider failover or retry.
- No streaming tool input validation (server may send incremental JSON that doesn't parse until the whole arguments string arrives).
- No Bedrock / Vertex (Anthropic via cloud providers) — M7.2.
- No native OpenAI vision support — text-only conversations.

## [0.6.0-dev] — 2026-05-20

M6.6 — KAIROS coordinator (minimal cross-process subagent).

### Added
- `pkg/kairos/` package: minimal SSE-based RPC for cross-process subagent dispatch.
  - Server: `POST /kairos/run` with `{subagent_type, prompt, description?}`; streams `event: text` deltas, ends with `event: done` (final text) or `event: error`. Optional Bearer auth via `KAIROS_AUTH_TOKEN` env on the worker side.
  - Client: `DispatchRemote(ctx, endpoint, token, type, description, prompt) (string, error)` consumes SSE stream and returns accumulated/final text.
- `subagent.Spec.Remote *RemoteSpec` (YAML `remote: {endpoint, auth_token}`) — auth_token supports `env:VARNAME` syntax. When set, `Engine.RunSubagent` dispatches via HTTP instead of spawning a local child Engine. Hooks (SubagentStop) still fire locally with success/error reason.
- `--kairos-serve <addr>` CLI flag — anthrogo runs as a worker that services subagent dispatches on demand using its own provider + tools + permissions. The worker excludes Remote subagent types from its registry to prevent multi-hop redirect.

This completes the M6 group: hooks/compact/skills/plugins (M4), subagents/MCP-resources-elicit/concurrent-isolated-yaml (M5), list_changed/independent-JSONL/form-elicit/websocket/oauth/kairos (M6).

### Known issues / deferred
- No remote hooks, no remote permission context, no remote tool execution — the worker uses its own.
- No bidirectional streaming (client can't cancel or feed back to the running subagent).
- No remote JSONL upload back to the caller.
- No multi-hop (worker rejects remote types in its registry).
- No connection pooling / retry / circuit-breaker.
- Worker uses the calling process's HOME / model / permissions, not the client's — pin your worker config carefully.
- No TLS termination (use a reverse proxy if needed); KAIROS_AUTH_TOKEN over plain HTTP leaks.

## [0.5.7-dev] — 2026-05-20

M6.5 — OAuth 2.1 client flow for MCP HTTP transports.

### Added
- `internal/oauth/` package: PKCE challenge generator + `Token{AccessToken, RefreshToken, ExpiresAt, Scopes}` + `FetchToken`/`SaveToken`/`LoadToken`.
- Authorization code + PKCE flow with local loopback callback server (default port 8765, configurable via `redirect_port`). Browser launch via `open` / `xdg-open` / `rundll32`.
- Automatic refresh-token grant on expiry; full re-auth on refresh failure.
- Tokens cached at `~/.anthrogo/oauth/<server-name>.json` (0600 mode, 0700 dir).
- `MCPServerConfig.OAuth *OAuthConfig` — when set + Type is sse/streamable/websocket, anthrogo fetches a token before transport construction and injects `Authorization: Bearer <token>` via a custom HTTP RoundTripper (or HTTPHeader for websocket).
- 30s clock-skew margin on expiry checks.

### Known issues / deferred
- No device-code / client-credentials grants (only PKCE authorization code).
- No token revocation endpoint integration.
- Browser-launch failure is silent (user must read stderr for the URL).
- Static `redirect_port`; collision with another anthrogo instance returns a setup error.
- No keychain integration (file mode 0600 only).

## [0.5.6-dev] — 2026-05-20

M6.4 — WebSocket MCP transport.

### Added
- `internal/mcp.WebSocketClientTransport{Endpoint, HTTPHeader}` — satisfies `sdk.Transport`. Frames each JSON-RPC message as one text-mode websocket message. 16 MiB read limit.
- `MCPServerConfig.Type: "websocket"` — uses `Endpoint` (`ws://` or `wss://`).
- Validation: websocket type requires non-empty Endpoint; missing endpoint → StateFailed with clear error.
- New dep: `github.com/coder/websocket` v1.8.14 (single-file, minimal-dep websocket library; formerly nhooyr.io/websocket).

### Known issues / deferred
- No automatic reconnect on transport failure (matches stdio/SSE/streamable behavior).
- No subprotocol negotiation (sends none; servers requiring a specific subprotocol won't accept the dial).
- No HTTPHeader configuration via YAML (`HTTPHeader` field exists on the struct for programmatic use; YAML-driven custom headers ship in a later milestone).

## [0.5.5-dev] — 2026-05-20

M6.3 — Real TUI form elicitation handler.

### Added
- `tool.PromptKind` gains `PromptElicitForm`; `PromptRequest` gains `Message` + `Schema map[string]any`; `PromptResponse` gains `Action` ("accept"|"decline"|"cancel") + `FormData map[string]any`.
- TUI permission modal renders an elicitation form with the server's message, the requested JSON schema, and a single-line text buffer. User types a JSON object, presses Enter to submit (accept) or Esc to cancel.
- `mcp.Manager.SetElicitationHandler(fn)` injection point; `NewServer(...)` signature extended with an `ElicitFn` callback (4th arg, nil-safe).
- `cmd/anthrogo` wires the manager handler to the TUI's `RequestPrompt` path; headless mode still declines (no TUI to render).
- `tui.App.RequestPrompt` extracted as a public method; engine wires it directly.
- Empty buffer + Enter → decline; invalid JSON + Enter → decline with reason.

### Known issues / deferred
- Form is a single-textarea JSON blob; multi-field structured UI (one input per schema property) is deferred.
- No schema validation against the user's submitted JSON; server-side rejection is the only validation.
- No multi-line input (Enter submits; can't type a newline inside the buffer).
- Headless mode always declines.

## [0.5.4-dev] — 2026-05-20

M6.2 — Independent JSONL per subagent.

### Added
- `session.NewSubagent(parent, subagentID)` constructor; subagent JSONL written to `<parent path without .jsonl>/subagents/<subagent-id>.jsonl`.
- `query.Engine.Config.Session *session.Store` — when non-nil, `RunSubagent` mints a UUID for the subagent and routes its `RecordHook` to a freshly-opened subagent Store.
- `session.KindSubagentStart` record + `SubagentRecord{ID, Type, Description}` payload — parent JSONL gains a marker pointing at the spawned subagent file. Replay treats it as informational (no message effect).
- `cmd/anthrogo` threads the parent Session into `query.Config` so the wiring activates automatically.

### Known issues / deferred
- Nested sub-sub-agents (subagent that spawns its own Task) share the immediate subagent's JSONL; per-nest isolation requires per-engine Session threading and is deferred.
- No CLI to inspect a subagent's JSONL; user opens the file path directly. A `/subagents log <id>` builtin lands in a later milestone.
- Compaction does not rewrite or prune subagent JSONLs.

## [0.5.3-dev] — 2026-05-20

M6.1 — MCP list_changed notifications.

### Added
- `internal/mcp.Server` registers `ToolListChangedHandler` and `ResourceListChangedHandler` on `sdk.ClientOptions`.
- On `notifications/tools/list_changed`: refresh the server's tool cache via `ListTools` + log via LogSink. The `tool.Registry` is NOT auto-rebuilt (would race with in-flight turns); user runs `/mcp reload` to surface new tools to the system prompt.
- On `notifications/resources/list_changed`: validate connectivity via `ListResources` + log. anthrogo doesn't cache resources at the server level (Manager.AllResources queries on-demand), so no cache update is needed.
- Echo-server testdata gains a hidden `_emit_list_changed` tool used by the new `TestServer_ToolListChanged_TriggersRefresh` integration test.

### Known issues / deferred (M6+)
- `tool.Registry` doesn't auto-refresh on list_changed; the model's system prompt still lists startup-cached tools until `/mcp reload` or restart.
- Resource subscription (`resources/subscribe`) is not wired.
- Prompt list_changed (`PromptListChangedHandler`) is not wired (anthrogo doesn't surface MCP prompts yet).

## [0.5.2-dev] — 2026-05-20

M5.3 — Subagent polish (concurrent + isolated perms + YAML types).

### Added
- User-defined subagent types via YAML at `~/.anthrogo/subagents/<name>.yaml` (home) and `<cwd>/.anthrogo/subagents/<name>.yaml` (project; overrides home). Each YAML has `name`, `description`, `system_prompt_suffix`, optional `tool_allowlist`. "general-purpose" name is reserved.
- `/subagents` slash command: list, `show <name>`, `reload`.
- `permissions.Context.Clone()` — shallow copy for subagent isolation; subagent Mode toggles no longer leak to parent.
- `pkg/subagent.Registry.Replace(other)` — used by `/subagents reload`.

### Changed
- `Task` tool's `IsConcurrencySafe` now `true`. Multiple Task tool_use blocks in one assistant turn run concurrently via the engine's parallel dispatch path. Per-subagent stderr/log output may interleave; hooks see events from all in-flight subagents merged.
- `query.Engine.RunSubagent` clones the parent's `permissions.Context` for the child.
- `command.Host` gains `Subagents() *subagent.Registry`.
- `pkg/query/loop.go`: when all in-flight tool_use blocks have `IsConcurrencySafe=true`, they are dispatched concurrently in goroutines; tool_result order preserved by indexed slots.

### Known issues / deferred (M6)
- Independent JSONL session per subagent (today: child uses RecordHook=nil, so child messages aren't persisted).
- MCP resources/list_changed notifications + subscription.
- Real TUI form-based elicitation handler (today: declines).
- WebSocket MCP transport.
- OAuth 2.1 client flow.
- KAIROS coordinator / remote sessions.

## [0.5.1-dev] — 2026-05-20

M5.2 — MCP resources + minimal elicitations.

### Added
- `internal/mcp.Server` gains `ListResources(ctx)` (with NextCursor pagination) and `ReadResource(ctx, uri)`.
- `internal/mcp.Manager` gains `AllResources(ctx) map[server][]*Resource` (per-server errors logged via LogSink, not propagated) and `ReadResource(ctx, server, uri)`.
- New `MCPResource` built-in tool: `{server, uri?}` — list or read MCP resources. Read-only; default `alwaysAllow` rule at CLI level.
- System prompt lists every Ready server's resources (capped at 50/server with "... N more" line).
- `MCPServerConfig.ElicitationMode` (`"decline"` default | `"disabled"`). When not disabled, anthrogo registers an ElicitationHandler that records the request via LogSink and returns `Action: "decline"`. This advertises the elicitation capability so servers know anthrogo is reachable; full form-input integration lands in M5.3.

### Changed
- `system.Options` gains `MCPResources map[string][]*sdk.Resource`.
- Version bumped to `0.5.1-dev`.

### Known issues / deferred (M5.3)
- Full TUI form-input elicitation handler (current handler always declines).
- WebSocket transport (no native SDK support).
- OAuth 2.1 client flow.
- Resource list_changed notifications + subscription.

## [0.5.0-dev] — 2026-05-20

M5.1 — Subagents (Task tool + sub-Engine).

### Added
- `Task` built-in tool: model invokes `{description, prompt, subagent_type}` to spawn a sub-engine that runs an isolated multi-step task and returns its final assistant text.
- `pkg/subagent/` package: `Spec` + `Registry`; ships with one "general-purpose" type pre-registered in `DefaultRegistry()`.
- `query.Engine.RunSubagent(ctx, opts) (string, error)`: builds child engine with subagent-specific system prompt suffix and optional tool allowlist, runs one turn, drains stream, fires `SubagentStop` hook.
- Nested depth limit (default 3; configurable via `query.Config.MaxSubagentDepth`); recursion past limit returns a clear error to the model.
- `SubagentStop` hook event from M4.1 now actually fires after every subagent run (success or error).
- System prompt lists available subagent types as `- name: description` in a section after tools/skills.
- Plan mode treats `Task` as a write tool — `IsWriteTool("Task") == true`. Switch to default mode to dispatch subagents while in plan mode.

### Changed
- `query.HookSink` gains `FireSubagentStop(ctx context.Context, reason string)`.
- `tui.PromptHookSink` and `headless.PromptHookSink` both gain `FireSubagentStop` to match.
- `query.Config` gains `SubagentRegistry *subagent.Registry`, `SubagentDepth int`, `MaxSubagentDepth int`.
- `tui.Options` gains `Subagents *subagent.Registry` and `OnEngineReady func(*query.Engine)`.
- `headless.Options` gains `Subagents *subagent.Registry` and `OnEngineReady func(*query.Engine)`.
- `internal/system.Options` gains `Subagents []subagent.Spec`; `BuildSystemPrompt` emits the subagent types section.
- Version bumped to `0.5.0-dev`.

### Known issues / deferred (M5.2 / M5.3)
- Subagents run serially (one Task at a time); concurrent multi-Task dispatch is M5.3.
- Subagents share the parent's permission context, hooks, and JSONL session; independent contexts + per-subagent JSONL are M5.3.
- User-defined subagent types via YAML are M5.3 (M5.1 ships one built-in type).
- WebSocket / OAuth / elicitations / resources MCP debt is M5.2.

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

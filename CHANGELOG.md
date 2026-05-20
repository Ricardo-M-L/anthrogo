# Changelog

## [0.9.8-dev] — 2026-05-21

M9.9 — Model + path + visibility polish.

### Added
- Built-in pricing aliases for Bedrock (`anthropic.claude-*`) and Vertex (`claude-*@*`) variants — `/cost` works out-of-box on cloud-hosted Anthropic models.
- KAIROS worker hook resolver — relative-path hook commands forwarded from clients are skipped on the worker with a warning (path is meaningless on the worker's filesystem). Hooks with absolute paths pass through normally.
- Nested-subagent prefix chain: when a subagent invokes Task, the outer task's description joins via `→` so the parent TUI shows `[Task: research → fetch-status] hello` rather than only the inner prefix. Propagated via new `query.Config.SubagentPrefixChain []string` and `tool.Context.SubagentPrefixChain`.
- `/sessions search --recurse-subagents` now walks multiple levels deep (was: 1 level).
- SymbolSearch reports `kind: method` for receiver-bound functions and includes the receiver in the matched line (e.g., `func (s *Server) Foo()`). `kind=method` filter matches methods only; `kind=func` matches both bare funcs AND methods.

### Known issues / deferred
- Full go/types integration (semantic xref, type signatures) deferred — adds heavy `golang.org/x/tools/go/packages` dep.
- Hook resolver doesn't attempt to relocate relative paths (e.g., search for matching binary on worker's PATH).
- Prefix chain depth uncapped — very deep nests produce long prefixes.

## [0.9.7-dev] — 2026-05-21

M9.8 — Tool & subagent JSONL polish.

### Added
- `Diff.range` field — commit-range diff like `HEAD~3..HEAD` or `main..feature`. Mutually exclusive with `cached` (returns IsError on conflict).
- `Format.paths` field — array of paths formatted in a batch. `paths` array preferred; legacy singular `path` still accepted. Aggregated result reports per-file success/failure: `formatted 2/3: a.go b.go FAILED c.py: <reason>`.
- Per-nest subagent JSONL: a sub-sub-agent (subagent calling Task) now writes to `<parent>/<sub-id>/subagents/<sub-sub-id>.jsonl`. M6.2 deferred this by setting Session=nil on the child Config; M9.8 plumbs childSessionStore through so the recursive layout works.

### Known issues / deferred
- Diff range doesn't combine with `path` filtering for multiple paths (single path only).
- Format aggregated output doesn't surface per-file diff/changed-line counts (just "formatted" / "FAILED").
- /sessions stats and search don't recurse into nested subagent JSONLs by default; --recurse-subagents only goes one level deep.

## [0.9.6-dev] — 2026-05-20

M9.7 — Form UI completion.

### Added
- Arrow keys (Left/Right/Home/End) navigate within the focused field's text. Cursor renders as `█`. Insertions go at cursor position; Backspace removes char BEFORE cursor; Delete removes char AT cursor.
- Multi-line string fields: Ctrl+J inserts a literal `\n` into a string-typed buffer. Plain Enter still submits.
- Enum support: schemas with `enum: [...]` render as a horizontal cycler (`[selected] other1 other2`). Left/Right or Up/Down cycles options; Tab moves to next field; Enter submits the focused selection.
- Schema `default` values pre-fill the field buffer at modal open. String defaults populate text; boolean defaults populate as "true"/"false"; number/integer as their string form; enum defaults match-index into the option list.

### Known issues / deferred
- Multi-line render shows `\n` as literal `\n` in the input row (no visual line break in the modal); the model still receives a real newline at submit.
- Enum is single-select only; multi-select schemas drop to textarea fallback.
- No tooltip / help bubble per field.

## [0.9.5-dev] — 2026-05-20

M9.6 — YAML config polish.

### Added
- `env:VARNAME` expansion now applies to `MCPServerConfig.Headers` values (matching M6.5 OAuth + M5.3 subagent loader pattern).
- `/mcp status <name>` prints the configured headers with sensitive values redacted (`<redacted>` for keys containing authorization/auth/token/key/secret/password/bearer, case-insensitive).
- `Config.SessionSearchCacheSize` (YAML `session_search_cache_size`) — overrides the default M8.12 cap of 64. 0/missing = default.

### Known issues / deferred
- Redaction is heuristic by key name; if a server uses a non-standard key for credentials it'll print in clear. Future could allow explicit `redact: ["X-Custom-Auth"]` list.
- env-prefix expansion happens once at startup; YAML changes during a session aren't picked up.

## [0.9.4-dev] — 2026-05-20

M9.5 — LSP-style code intel tools.

### Added
- `SymbolSearch` tool — finds a symbol's definition by name. `.go` files parsed via `go/parser` for accurate position + kind classification (func/type/var/const). Other languages (`.js`/`.ts`/`.tsx`/`.py`/`.rs`/`.rb`) use language-specific regex heuristics. Returns up to 50 hits as `path:line: matched-line`. `kind` parameter filters; `path` defaults to cwd.
- `References` tool — finds usages of a name via word-boundary regex `\b<name>\b` across the tree. Skips binary files (heuristic: any 0x00 byte in first 512). Returns up to 200 hits as `path:line:col: matched-line`.
- Both tools skip `vendor/`, `node_modules/`, `.git/`, `.anthrogo/`, and respect 1MB per-file size cap.

### Known issues / deferred
- Not a real LSP. No semantic xref (e.g., distinguishing `pkg1.Foo` from `pkg2.Foo` by import path). No type info.
- Go AST resolution mode is lossy (SkipObjectResolution) — accurate for top-level decls, but doesn't catch method receivers, nested types.
- Non-Go regex heuristics produce false positives for shadowed/local names.
- No incremental indexing; each call walks the tree.

## [0.9.3-dev] — 2026-05-20

M9.4 — Subagent real-time stream to parent TUI.

### Added
- `query.SubagentOptions.OnTextDelta func(string)` — fires for every EventTextDelta from the child engine's stream.
- `kairos.ClientOptions.OnTextDelta` — invoked per `event: text` SSE message in remote subagent runs.
- `tool.TaskOptions.OnDelta` — passed through Task tool's runner; cmd/anthrogo wires this to `query.SubagentOptions.OnTextDelta`.
- Task tool's `Call` uses `tcx.AppendUIMessage` with a per-task prefix `[Task: <description>] ` to emit subagent output to the parent TUI as it streams. Deltas are buffered until newline boundaries to avoid scroll spam; remaining buffer flushed when the subagent finishes.
- `tool.deltaBuffer` — internal line-buffering helper; mutex-safe for concurrent delta writes.

### Known issues / deferred
- No interactivity: parent can't interrupt a subagent mid-run (Ctrl+C still cancels the whole turn).
- Buffered per-line; very long lines (no newline for 10KB) cause one big render at end of stream.
- Headless mode appends to its progress stream; no special UI layering.
- For nested subagents (subagent calling Task), only the outermost prefix is shown.

## [0.9.2-dev] — 2026-05-20

M9.3 — Multi-hop KAIROS + remote hook/perm context.

### Added
- `kairos.RunRequest.RemoteContext` — client populates with hops counter, hooks.Config snapshot, and permissions snapshot. Worker reads it to inherit caller's gate + hook config.
- `kairos.PermSnapshot{Mode, AlwaysAllowRules, AlwaysDenyRules, AlwaysAskRules}` — JSON-safe permissions.Context projection.
- `kairos.MaxHops = 2` — workers register their Remote-type subagents only when incoming HopDepth < MaxHops. Outgoing forwards bump HopDepth.
- Client `Engine.RunSubagent` remote branch fills RemoteContext from `Engine.Config.HooksConfig` + `Engine.Config.Permissions`.
- `query.Config.HooksConfig *hooks.Config` — new field; set by cmd/anthrogo when building the engine; forwarded into RemoteContext so KAIROS workers can apply the client's hook rules.
- Worker `cmd/anthrogo --kairos-serve` substitutes the per-request hooks/perms when RemoteContext.Hooks or .Permissions is non-nil; falls back to worker's local settings otherwise.
- `tui.Options.HooksConfig` and `headless.Options.HooksConfig` — both forward the raw hooks.Config into query.Config.

### Known issues / deferred
- Hook commands are passed verbatim — paths relative to client cwd won't resolve on worker. Use absolute paths in hooks meant for KAIROS.
- Hop depth is a number, not a path; a 2-hop chain can't be inspected mid-flight (no audit endpoint).
- Worker still uses its own provider / model / API keys; remote does NOT proxy back to the client's LLM.
- Permission gate's HookDecide isn't serialized (it's a Go func); worker uses its own HookDecide via the rebuilt hook manager.

## [0.9.1-dev] — 2026-05-20

M9.2 — Vertex provider (Anthropic via GCP).

### Added
- `pkg/provider/vertex/` — wraps Anthropic SDK's vertex subpackage. Uses Google application default credentials (`gcloud auth application-default login`, GOOGLE_APPLICATION_CREDENTIALS env, or workload identity).
- Profile fields: `region` (mandatory; e.g. `us-east5`), `project_id` (mandatory).
- Vertex model IDs follow GCP's published name convention: `claude-sonnet-4-6@20260101` (publisher/anthropic models in Model Garden).

### Example

```yaml
provider: vertex-sonnet
profiles:
  vertex-sonnet:
    type: vertex
    model: claude-sonnet-4-6@20260101
    region: us-east5
    project_id: my-gcp-project
```

### Known issues / deferred
- No explicit credential file path option; relies on default chain (GOOGLE_APPLICATION_CREDENTIALS env or `gcloud auth application-default login`).
- Vertex model IDs differ from direct Anthropic API names; pricing lookups won't match without YAML aliases.
- Stream-level testing requires real GCP creds; CI skips.

## [0.9.0-dev] — 2026-05-20

M9.1 — Bedrock provider (Anthropic via AWS).

### Added
- `pkg/provider/bedrock/` — wraps Anthropic SDK's bedrock subpackage. Uses AWS default credential chain (env, ~/.aws/credentials, IAM role). Optional `region` field in the profile (defaults to `AWS_REGION` env or default config).
- `Profile.Region` YAML field (bedrock only).
- New dep: `github.com/aws/aws-sdk-go-v2/config` (pulled in transitively by the Anthropic SDK's bedrock package; upgraded to latest).
- `pkg/provider/anthropic.NewWithOptions(model, opts...)` factored out so bedrock and future Vertex backends reuse the same Stream implementation.

### Example

```yaml
provider: bedrock-sonnet
profiles:
  bedrock-sonnet:
    type: bedrock
    model: anthropic.claude-sonnet-4-6-v1:0
    region: us-west-2
```

### Known issues / deferred
- Bedrock model IDs follow AWS format (`anthropic.claude-sonnet-4-6-v1:0`), different from direct Anthropic API names. Pricing table lookups may not find matches — add aliases to pricing YAML if needed.
- No explicit IAM role assume; relies on default credential chain.
- Stream-level testing requires real AWS credentials; not included in CI.

## [0.8.13-dev] — 2026-05-20

M8.13 — Subagent remote tool execution.

### Added
- `subagent.RemoteSpec.ExecToolsLocally bool` (YAML `exec_tools_locally`). When true, tool calls from the remote subagent execute on the CLIENT process — client's tool registry, client's permission gate.
- `query.Config.ToolDispatcher func(ctx, toolUseID, toolName, input) (Result, error)` — pluggable dispatch hook; when non-nil, replaces local tools.Registry dispatch. Used by both worker (to forward to client) and as a general extension point.
- KAIROS protocol additions: `ToolUseRequest`, `ToolResult` payloads. Worker emits `event: run_id` then `event: tool_use_request` per blocking tool call. Client POSTs to `/kairos/run/<rid>/tool-result` with the result. Worker resumes the engine's stream.
- `kairos.DispatchRemoteWithOptions(ctx, endpoint, req, opts)` — new client API. Existing `DispatchRemote` stays as a thin wrapper.
- `kairos.NewServerWithToolForward(handler, handlerWithForward, authToken)` — server constructor that enables exec-tools-locally mode when `X-Anthrogo-Exec-Tools-Locally: true` header is present.
- `cmd/anthrogo` worker mode auto-detects `X-Anthrogo-Exec-Tools-Locally: true` header and builds a forwarding ToolDispatcher for that request.

### Known issues / deferred
- Bidirectional flow uses SSE-down + separate POST-up; not real bidi. Tool execution latency = client RTT to worker + tool runtime + client RTT back.
- No streaming of large tool results (a 5MB Read result is one POST body).
- Worker process state per in-flight run is kept until SSE close; long-running tools holding the channel keep memory alive.
- Permission gate on the client still runs; if it denies, the worker subagent sees an IsError tool result and may retry, loop, or give up depending on its prompt.

## [0.8.12-dev] — 2026-05-20

M8.12 — LRU index for /sessions search.

### Added
- `session.ReplayCache` — in-memory LRU of parsed `[]Record` keyed by `(path, modtime)`. Default capacity 64 sessions. Modtime changes auto-invalidate. Thread-safe.
- `Sessions{ReplayCache *ReplayCache}` accepts the cache; `search` / `replay` / `stats` / `export` / `show` all consult it first, falling back to direct `Replay` if not set.
- `/sessions reindex` (alias `search-rebuild-index`) — clears the cache; rebuilds on demand from next call.
- `Sessions.deleteSession` invalidates the cache entry after removing a JSONL.

### Known issues / deferred
- Cap is hardcoded at 64; not configurable from YAML yet.
- Cache is process-local; restart anthrogo loses warm state.
- No persistence to disk; future could SQLite-index by hash.

## [0.8.11-dev] — 2026-05-20

M8.11 — Diff / Format / Git built-in tools.

### Added
- `Diff` tool — wraps `git diff`. Options: `path` (file/dir), `cached`, `context` (lines), `stat`. Read-only. Returns the diff text.
- `Format` tool — language-aware formatter dispatch. `.go` → gofmt, `.js/.ts/.tsx/.css/.html/.yaml/.md` → prettier (if on PATH), `.py` → black or ruff, `.rs` → rustfmt. Writes file in place. Errors if formatter not on PATH.
- `Git` tool — read-only subset: status, log, branch, show, blame, remote. Args sanitized against shell metacharacters; destructive subcommands (commit, push, reset, etc.) NOT allowlisted — the model uses Bash with the existing permission gate for those.

### Known issues / deferred
- No support for diff of a specific commit range (use Bash for git diff sha1..sha2).
- Format doesn't queue / batch across multiple files (one path per call).
- Git tool's arg sanitization is a denylist (`;`, `&&`, `||`, `|`, `>`, `<`, backtick); won't catch all shell-injection vectors. Allowlist commands provide the primary safety; users still see the permission ask before each Git call.
- No JSON-formatted git output (porcelain v2) — text only.

## [0.8.10-dev] — 2026-05-20

M8.10 — Real tokenizer (tiktoken-go).

### Added
- `pkg/tokens/` package — `Counter` bound to a model:
  - gpt-4o / o1 / o3 / gpt-5 → o200k_base (tiktoken-go)
  - gpt-4 / gpt-3.5 / text-embedding-3 → cl100k_base
  - deepseek-* / kimi-* / moonshot-* / minimax / abab / glm-* → cl100k_base (close approximation)
  - Anything else (claude-*, unknown) → (len+3)/4 char approximation
- `Counter.CountText(s)` / `CountBlocks([]Block)` / `CountMessages([]Message)`. Per-message overhead ~3 tokens for role tagging (matches OpenAI documented behavior).
- BlockImage skipped (real image-token cost only available via provider EventUsage).
- New dep: `github.com/pkoukk/tiktoken-go`.

### Changed
- `compact.Run` now uses `tokens.Counter` instead of `ApproxBytes` for `OriginalTokens` / `NewTokens` fields. Result text now reports "X → Y tokens" instead of "X → Y bytes".
- `compact.Output.OriginalBytes`/`NewBytes` renamed to `OriginalTokens`/`NewTokens`.
- `query.Summary.OriginalBytes`/`NewBytes` renamed similarly.
- `session.CompactRecord` gains `original_tokens`/`new_tokens` JSON fields. Old `original_bytes`/`new_bytes` fields retained in struct with `omitempty` for legacy JSONL backward compatibility.
- `/compact` builtin format string updated to "X → Y tokens".
- `compact.ApproxBytes` marked deprecated; still callable for tests.

### Known issues / deferred
- Anthropic Claude has no public tokenizer; char/4 is a rough underestimate.
- Image tokens not counted (provider EventUsage is authoritative).
- BPE encoders init lazily on first use; first /compact takes ~5ms longer.

## [0.8.9-dev] — 2026-05-20

M8.9 — Real multi-field form elicitation UI.

### Added
- Multi-field form rendering when MCP server's elicitation schema is a flat object with only primitive properties (string / number / integer / boolean). Each property gets its own input row with type hint + required marker; Tab/Shift-Tab navigates; Enter submits; Esc cancels.
- Type coercion: string → as-is, integer/number → strconv.Atoi/ParseFloat (decline with reason on parse error), boolean → "yes/no/y/n/true/false/1/0" mapped to bool.
- Required-field empty-check: declines with reason; optional empty fields omitted from FormData.
- Schemas with nested objects, arrays, enums fall back to the M6.3 single-textarea JSON path.

### Known issues / deferred
- No enum dropdown UI (enum schemas drop to textarea fallback).
- No multi-line strings in any single field (Enter is reserved for submit).
- Cursor rendering is a static block; no left/right arrow support yet.
- No default values from schema applied to buffer at start.

## [0.8.8-dev] — 2026-05-20

M8.8 — Inline $EDITOR via tea.ExecProcess.

### Added
- `command.Result.ExecCmd *ExecRequest` — slash commands can now request a subprocess be launched (typically a $EDITOR). TUI suspends bubbletea via tea.ExecProcess, runs the cmd, resumes the screen, then appends an OnComplete-returned status to the chat. Headless mode exposes `headless.RunExecRequest` to run the cmd directly with inherited stdio.
- `/system edit` and `/system edit project` now actually open `$EDITOR <overlay-path>` from inside anthrogo without restart. Overlay saves are reported back in the chat.

### Known issues / deferred
- Overlay edits still apply on the NEXT turn (the engine's system prompt for the current session is frozen at startup). Future could re-resolve the overlay on every turn boundary.
- $EDITOR uses inherited stdio; if EDITOR is a complex modal editor that paints, your terminal scroll history during the edit is captured into bubbletea's restoration.
- No editor-quit detection beyond exit status.

## [0.8.7-dev] — 2026-05-20

M8.7 — WebSocket subprotocol + YAML HTTPHeader.

### Added
- `MCPServerConfig.Subprotocols []string` (YAML `subprotocols`) — passed to `WebSocketClientTransport` and on to `websocket.Dial`'s `DialOptions.Subprotocols`. Server's selected subprotocol returned in `Sec-WebSocket-Protocol` response header.
- `MCPServerConfig.Headers map[string]string` (YAML `headers`) — applied to ALL HTTP-based transports (websocket via `DialOptions.HTTPHeader`; sse / streamable via a new `headerInjector` RoundTripper on the configured HTTPClient).
- When both `oauth:` and `headers:` are configured, the `headerInjector` and `bearerInjector` compose so Authorization + custom headers both land on the request.
- `internal/mcp/transport_auth.go` adds `headerInjector` next to the existing `bearerInjector` (M6.5).

### Known issues / deferred
- Subprotocol is websocket-only (sse / streamable don't have a subprotocol layer).
- Headers can't be templated from environment variables in YAML (use literal strings). Future could expand `env:VARNAME` syntax there.
- No header redaction in `/mcp status <name>` output — Authorization values would leak if printed.

## [0.8.6-dev] — 2026-05-20

M8.6 — /sessions search enhancements.

### Added
- `/sessions search --regex <pattern>` — interpret keyword as a Go `regexp.Compile` pattern; invalid regex returns an error before scanning. Without `--regex`, behavior is unchanged (case-insensitive substring).
- `/sessions search --recurse-subagents` — also scan `<session-id>/subagents/*.jsonl`. Matching record IDs are formatted as `<parent>/subagents/<sub>` so the source is clear.
- `/sessions search --since YYYY-MM-DD` / `--until YYYY-MM-DD` — filter records by Timestamp. Inclusive on both ends (until = start of next day).
- Context snippet around match: 40 chars before and after; for regex matches, the matched substring's position drives the window.

### Known issues / deferred
- Only one keyword per invocation (no OR'd patterns).
- Timestamp filter applies per-record (a session with one in-range record + many out-of-range records still scans the whole file).
- No `--limit N` flag yet; cap remains 200.
- Recurse depth is exactly one level (`subagents/`, not `subagents/.../subagents/`).

## [0.8.5-dev] — 2026-05-20

M8.5 — /sessions stats.

### Added
- `/sessions stats` — aggregates metrics across every JSONL in the current cwd's session directory:
  - Total sessions, turns, input/output tokens
  - Estimated total cost in USD (uses built-in default pricing from M8.1)
  - First-seen / latest timestamps
  - Per-model token + cost breakdown
  - Per-day turn count
- `--since YYYY-MM-DD` / `--until YYYY-MM-DD` flags to filter the aggregation.

### Known issues / deferred
- Cost estimate ignores user-supplied pricing overrides (uses built-in defaults only). Wire in user rates later if needed.
- Aggregation is single-pass (linear in number of JSONLs); large session libraries could be slow.
- Per-day count uses local timezone of the timestamps in JSONL (which were written with local time at record time).
- Doesn't recurse into subagent JSONLs.
- Tools/MCP/hook events not counted as "turns" — only KindTurnComplete is a turn.

## [0.8.4-dev] — 2026-05-20

M8.4 — /cost reset + /compact --reset-budget.

### Added
- `Engine.ResetUsage()` — zeroes cumulative session usage and since-last-compact counter under lock.
- `/cost reset` subcommand — calls ResetUsage and reports.
- `/compact --reset-budget` flag — when /compact succeeds (not Skipped), also resets usage so the post-compact session starts fresh against the budget cap.

### Known issues / deferred
- ResetUsage doesn't touch the cost-limit budget gate; it just zeroes the counter. Budget remains armed; user can hit it again as new usage accumulates.
- No "undo" — once reset, prior usage is lost from the in-memory engine. JSONL records still have the prior data.

## [0.8.3-dev] — 2026-05-20

M8.3 — TUI 1Hz status refresh.

### Added
- TUI now schedules a `tea.Tick` once per second. Each tick re-paints the View, so the status line (tokens, since-compact, cost, budget) updates live during long-running turns instead of staying frozen until the next event arrives.
- Tick auto-re-schedules from inside Update; no global state.

### Known issues / deferred
- 1Hz is hardcoded; no flag to change frequency.
- Other parts of the UI re-render on every tick too (cheap, but worth flagging if performance ever matters).
- No tick during permission modal pause — modals own the terminal but cmd loop still runs, so tick continues; render is correct.

## [0.8.2-dev] — 2026-05-20

M8.2 — Per-project system prompt overlay.

### Added
- `<cwd>/.anthrogo/system_overlay.md` — project-level overlay appended AFTER the home overlay. Project text comes later in the system prompt, so model attention naturally prefers it for project-specific instructions.
- `config.ProjectSystemOverlayPath(cwd)` helper.
- `/system show` now prints both layers separately with their paths.
- `/system edit home` / `/system edit project` (no arg defaults to home).
- `/system reset home` / `/system reset project`.

### Known issues / deferred
- Project overlay isn't shared across cwd-shifts within one anthrogo session (overlay paths are resolved once at startup).
- No file watcher; edits don't apply mid-session — restart anthrogo to pick up.
- No git-tracked / repo-shared distinction; the project overlay is just `<cwd>/.anthrogo/system_overlay.md`. Add to .gitignore or commit as a team prompt as you see fit.

## [0.8.1-dev] — 2026-05-20

M8.1 — Built-in default pricing table.

### Added
- `pkg/pricing.DefaultRates()` — built-in per-million-token USD rates for major models: Claude opus-4-7/sonnet-4-6/haiku-4-5 (+ version-stamped variants via glob), OpenAI gpt-5*/gpt-4o/gpt-4o-mini/gpt-4-turbo/o1*/o3*, DeepSeek chat/reasoner, Kimi k2*, MiniMax M2/abab*, GLM 4*/zero-*. Sourced from published pricing as of 2026-05.
- `pkg/pricing.MergeWithUserRates(user)` merges YAML-configured rates onto the built-in defaults; user keys always win on exact collisions.
- `cmd/anthrogo` always builds the pricing table now (built-ins + user merge) — `/cost` works out of the box on any of the listed models without YAML configuration.
- `query.Engine.Model()` accessor for cost-message clarity.

### Changed
- `/cost` "no pricing" message now includes the current model name and points at the YAML override.

### Known issues / deferred
- Built-in rates are static; periodic price changes from providers will drift until the next anthrogo release updates them.
- No region-specific rates (Anthropic Bedrock vs direct, OpenAI Azure vs direct).
- Cache-creation / cache-read pricing not surfaced (treated as zero in cost calc).

## [0.8.0-dev] — 2026-05-20

M8 — /system show / edit / reset (custom system prompt overlay).

### Added
- `~/.anthrogo/system_overlay.md` — optional persistent user overlay appended verbatim to the system prompt sent to the model. Loaded at startup; effective from the first turn onward.
- `/system show` — prints the active system prompt (current turn) followed by the overlay file's content.
- `/system edit` — prints the overlay path and the `$EDITOR <path>` invocation to run outside anthrogo. Creates the overlay file with a seed comment if it doesn't exist.
- `/system reset` — removes the overlay file. Effective next session.
- `internal/config.SystemOverlayPath(home)` helper.
- `query.Engine.SystemPrompt()` accessor.
- `system.Options.UserOverlay string` — empty by default; appended after the existing sections under a "# User overlay" heading.

### Known issues / deferred
- Overlay changes don't apply mid-session; user restarts anthrogo to pick up edits.
- No in-TUI editor (would conflict with bubbletea's terminal ownership). A future milestone could spawn the editor via tea.ExecProcess.
- Only one global overlay; no per-project (`<cwd>/.anthrogo/system_overlay.md`) layering yet.
- `/system show` truncates nothing — large prompts produce a lot of scroll.

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

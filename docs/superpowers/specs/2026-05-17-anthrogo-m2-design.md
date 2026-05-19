# anthrogo M2 Design Spec

> **Date**: 2026-05-17
> **Status**: design pending user approval
> **Owner**: Ricardo
> **Builds on**: M1 (`2026-05-17-anthrogo-design.md`, `2026-05-17-anthrogo-m1.md`)

## 1. Goal

Move anthrogo from "can have a conversation" to "can be re-entered, run plan-then-act workflows, web research, ask questions, and is operated through a slash-command palette." Close out the M1 reviewer's flagged issues at the same time.

## 2. Non-goals

- **MCP** (M3). No external tool servers in M2.
- **Plugin / skill system** (M4). Slash commands are static + a registration interface; M4 will wire dynamic registration.
- **Subagents / AgentTool** (M5).
- **Other providers** (M6).
- **Compact / snip / cost-aware truncation**. M2 keeps full transcripts; M3 introduces compaction once MCP load grows context faster.

## 3. Scope

Four feature buckets plus M1 carry-overs.

### 3.1 New tools (4)

| Tool             | Behavior                                                                        |
|------------------|---------------------------------------------------------------------------------|
| WebFetch         | HTTP GET a URL → return body. Optional markdown conversion via `html-to-markdown`. In-memory LRU cache (process lifetime, capacity 32 entries, TTL 15 min). Limit 5 MB. |
| WebSearch        | Calls a configurable backend (default: Brave Search API). Returns top-N `{title, url, snippet}`. |
| AskUserQuestion  | Pushes a question + 2–4 options through the surface's `RequestPrompt`. Each option has `{label, description}`. Returns `{selected_label, notes}` — `notes` is the optional free-text the user types alongside their selection. In headless mode the tool errors out (interactive-only). |
| NotebookEdit     | `.ipynb` cell edit: replace_cell, insert_cell_before, insert_cell_after, delete_cell. Preserves notebook metadata. |

### 3.2 Session persistence

- Format: **JSONL**, one `Record` per line.
- Path: `~/.anthrogo/projects/<cwd-hash>/<session-uuid>.jsonl`.
  - `<cwd-hash>` = first 12 chars of SHA-256 of the absolute cwd. Provides stable dedup across `cd` symlinks.
- Record types (each `{type, ts, ...payload}`, ts = RFC3339Nano):
  - `session_meta` — first line. Fields: `session_id`, `cwd`, `model`, `permission_mode`, `anthrogo_version`, `created_at`.
  - `user_message` — fields: `content` (message.Block[]).
  - `assistant_message` — fields: `content` (message.Block[]), `stop_reason`.
  - `tool_use_request` — fields: `tool_use_id`, `tool_name`, `tool_input`.
  - `tool_result` — fields: `tool_use_id`, `text`, `is_error`.
  - `turn_complete` — fields: `stop_reason`.
  - `error` — fields: `error`, `during` ("stream" | "tool_exec" | "permission" | "...").
  - `usage` — fields: `input_tokens`, `output_tokens`, `cache_creation_input_tokens`, `cache_read_input_tokens`.
- Resume:
  - `anthrogo --resume <session-uuid>` opens a chooser if multiple candidates match, else loads.
  - `anthrogo --continue` resumes the most-recent session for the current cwd.
  - On resume: rebuild `Engine.messages` by replaying records; rebuild `Usage`; restore `Cwd`. UI replays the transcript into the chat viewport.
- Append-only writes; `fsync` on every `turn_complete`. Best-effort: crash mid-stream loses at most the in-flight assistant deltas.

### 3.3 Plan mode (hard-lock)

- New permission mode: `plan` (already in M1 enum).
- New tools `EnterPlanMode` and `ExitPlanMode` (no-input tools that flip `Permissions.Mode`).
- Gate rule update: in `plan` mode, the gate denies any tool whose name matches `{Write, Edit, NotebookEdit}` AND any `Bash` invocation whose command is NOT recognised as read-only (M2 minimal heuristic: matches a small allowlist of read commands — `ls`, `cat`, `grep`, `rg`, `find`, `git status`, `git diff`, `git log`, `pwd`, `wc`, `stat`, `head`, `tail`).
- `Permissions.PrePlanMode` is stashed when entering and restored on exit.
- TUI: a yellow status badge "PLAN MODE — write tools disabled" overlays the bottom statusline.
- The system prompt gets a plan-mode addendum when active: "You are in plan mode. Do not execute any modifying actions. Investigate, propose a plan, then call ExitPlanMode to ask for permission to execute."

### 3.4 Slash command palette

- New package `pkg/command` with:
  ```go
  type Command interface {
      Name() string             // "/help", "/clear", ...
      Aliases() []string
      Description() string
      Type() Type               // local | local-prompt | submit
      Run(ctx context.Context, args string, host Host) (Result, error)
  }

  type Host interface {       // injected by the surface
      Engine() *query.Engine
      Permissions() *permissions.Context
      Tools() *tool.Registry
      Session() *session.Store
      ReplaceMessages([]message.Message)
      AppendUIMessage(string)
      Quit()
  }
  ```
- 10 built-in commands:
  - `/help` — list all commands
  - `/clear` — start a fresh session (writes a new JSONL file)
  - `/compact <hint?>` — manual placeholder (returns "compact deferred to M3")
  - `/resume <id?>` — open the resume chooser
  - `/model <name?>` — switch active model; with no arg, list
  - `/mode <plan|default|acceptEdits|bypass>` — set permission mode
  - `/cwd <path?>` — print or change working directory
  - `/add-dir <path>` — add to additionalWorkingDirectories
  - `/tools` — list registered tools with descriptions
  - `/memory` — print resolved CLAUDE.md
- TUI integration:
  - Input starting with `/` triggers a palette popup listing matching commands by fuzzy match (subsequence over Name + Aliases).
  - `Tab` accepts the highlighted candidate and inserts its full name into the input, replacing the user's partial.
  - `Tab` cycles forward through candidates if multiple match; `Shift+Tab` cycles backward.
  - `Enter` executes the command currently shown in the input (whether typed or Tab-completed).
  - `Esc` dismisses the palette without changing the input.
  - The palette disappears as soon as the input no longer starts with `/`.
- Slash commands run BEFORE the message hits the engine (they don't go to the model). The CommandType `submit` exists so future commands can wrap a slash invocation into a normal user prompt (e.g. `/init` injects a templated prompt and submits) — none of the M2 builtins use it.

### 3.5 M1 carry-overs (polish)

1. **Mid-turn cancel in REPL**: thread `context.Context` from TUI Update through `SubmitMessage`; `Ctrl+C` cancels the current turn and re-enables the input.
2. **TUI permission modal leak**: close reply channels on quit; flush pending asks.
3. **Verbatim coreSystemPrompt**: extract the 2.1.88 system prompt text from the source map and ship a `coreSystemPrompt_v2_1_88.txt` embed via `//go:embed`.
4. **Grep count mode determinism**: sort file paths before emission.
5. **Engine denials tracking**: add `[]PermissionDenial` accumulator + `Engine.Denials()` getter.
6. **Tool.Permission method**: add to interface, with default impl `func (Default) Permission(...) permissions.Decision { return Decision{Behavior: BehaviorAsk} }` — embed in tools where the gate alone is enough.

## 4. Architecture changes

### 4.1 New files

```
internal/
  command/                  # slash command runtime
    palette.go              # TUI overlay model
  session/
    store.go                # rewritten — JSONL-backed
    record.go               # Record union
    paths.go                # cwd-hash, dir layout
    resume.go               # chooser + --continue resolver
pkg/
  command/                  # slash command interface + builtins
    command.go              # Command, Host, Type, Result
    registry.go             # Registry, Register, Lookup
    builtins/
      help.go
      clear.go
      resume.go
      model.go
      mode.go
      cwd.go
      adddir.go
      tools.go
      memory.go
      compact.go
  permissions/
    plan.go                 # plan-mode gate hook + read-only Bash heuristic
  tool/
    webfetch.go
    webfetch_test.go
    websearch.go
    websearch_test.go
    askuserquestion.go
    askuserquestion_test.go
    notebookedit.go
    notebookedit_test.go
    planmode.go             # EnterPlanMode + ExitPlanMode tools
    planmode_test.go
  query/
    denials.go              # PermissionDenial + Engine.Denials()
internal/system/
  prompts/                  # //go:embed text files
    core_v2_1_88.txt
    plan_mode_addendum.txt
```

### 4.2 Type changes

- `permissions.Context.Mode` already exists; `plan.go` adds:
  ```go
  func IsWriteTool(name string) bool
  func IsReadOnlyBashCommand(cmd string) bool
  ```
  `Decide` consults these when `Mode == ModePlan`.
- `tool.Tool` interface gains `Permission(ctx, input) permissions.Decision`. Existing tools embed `tool.DefaultPermission{}` to keep the old behavior.
- `query.Config` gains `RecordHook func(session.Record)` (optional). The engine calls the hook synchronously inside its goroutine whenever it emits a high-level `Event` (KindAssistantStop → assistant_message; KindToolUseRequest → tool_use_request; KindToolResult → tool_result; KindUsage → usage; KindTurnComplete → turn_complete; KindError → error). The hook is responsible for writing to disk; `internal/session.Store.NewRecordHook()` returns a closure that does this with append-only fsync semantics. Decoupling via callback keeps `pkg/query` free of filesystem awareness — the engine never imports `internal/session`.
- `cmd/anthrogo/main.go` gets `--resume <id>`, `--continue` flags.

### 4.3 Backend config for WebSearch

New optional settings.yaml stanza:

```yaml
webSearch:
  backend: brave          # brave | tavily | googlecse | disabled
  apiKey: ${BRAVE_API_KEY}
  endpoint: https://api.search.brave.com/res/v1/web/search
```

WebSearch tool errors out with a clear message if no backend is configured (so plain users without keys aren't blocked from the rest of M2).

## 5. Data flow — resume

```
anthrogo --resume <uuid>
  │
  ▼
config.Load() + session.Resolve(uuid)
  │
  └─ Opens ~/.anthrogo/projects/<hash>/<uuid>.jsonl
       │
       └─ For each Record:
            session_meta      → set cwd, model
            user_message      → append to messages
            assistant_message → append
            tool_use_request  → append (tool_use block)
            tool_result       → append (user role + tool_result block)
            turn_complete     → mark turn boundary
            error / usage     → restore counters
       │
       └─ Build Engine with restored messages
       │
       └─ TUI replays each into chat viewport (no API call)
```

## 6. Error handling

| Case                                | Behavior                                                                                       |
|-------------------------------------|------------------------------------------------------------------------------------------------|
| Mid-turn `Ctrl+C` in REPL           | Engine ctx cancelled; in-flight Bash subprocess killed; pending tool_use marked as error; input re-enabled |
| Resume file truncated/corrupt       | Surface error in chat with line number; engine starts a fresh session                          |
| WebFetch >5 MB                      | Return IsError with size info; tool_result truncated to 512 KB                                 |
| WebSearch no API key                | IsError with "configure webSearch.apiKey in settings.yaml"                                     |
| Plan mode write attempt             | Synthetic tool_result: "denied: plan mode active — call ExitPlanMode first"                    |
| Notebook JSON invalid               | IsError with parse error + offending byte position                                             |
| AskUserQuestion in headless         | IsError "AskUserQuestion is interactive-only; use --permission-mode acceptEdits or run REPL"   |

## 7. Testing strategy

- Each new tool gets its own `*_test.go` with table-driven cases + a `t.TempDir()` workspace where the filesystem is involved.
- WebFetch/WebSearch tests use `httptest.NewServer` for the backend.
- Session persistence: round-trip test (append → read back via Resume → asserts Messages match).
- Plan mode: gate tests confirm `Write/Edit/NotebookEdit` denied + Bash read-only allowlist.
- Slash command: parser test for `/cmd args`, table-driven dispatch tests against a fake `Host`.
- Mid-turn cancel: engine_test fires a cancel during a fake long stream; asserts Event chan closes and final `KindError` carries `context.Canceled`.

## 8. Phased delivery (within M2)

| Sub-phase | Scope                                                | Approx tasks |
|-----------|------------------------------------------------------|--------------|
| M2.A      | Polish (mid-turn cancel + denials + Tool.Permission + grep determinism + modal leak fix + verbatim prompt) | 6            |
| M2.B      | Session persistence (record, store rewrite, resume, --continue, smoke) | 5            |
| M2.C      | New tools (WebFetch, WebSearch, AskUserQuestion, NotebookEdit) | 8            |
| M2.D      | Plan mode (gate update, EnterPlanMode + ExitPlanMode tools, system prompt, TUI badge) | 4            |
| M2.E      | Slash command palette (pkg/command, 10 builtins, TUI overlay) | 6            |

Total ~29 tasks. Each commits independently. M2.A first to unblock the rest.

## 9. Risks / open questions

1. **WebFetch content extraction**: raw HTML is unfriendly to the model. Plan ships markdown via `JohannesKaufmann/html-to-markdown`. If size/quality is wrong we can swap to `kennygrant/sanitize` + a smaller regex strip in M3.
2. **WebSearch backend**: Brave is the default but the hard dep is just `net/http`. The settings stanza supports swapping backends; no Brave SDK is pulled.
3. **NotebookEdit format drift**: Jupyter nbformat v4 is stable since 2017; we pin to that and reject v3.
4. **Session replay accuracy**: JSONL replay must preserve byte-identical input to the model on resume, or prompt caching breaks. We test this with a round-trip equality check on every supported block type.
5. **Slash command vs prompt collision**: input starting with `/` could be a literal request like "/usr/bin matters". Resolution: only intercept if the token before whitespace exactly matches a registered command name; otherwise let it through as a user message.
6. **Plan-mode Bash heuristic**: a hardcoded allowlist of read commands will miss tools the user installs (e.g., `rg` aliases, custom scripts). Acceptable for M2; M5 BashTool security rewrite replaces it.

## 10. References

- M1 spec: `docs/superpowers/specs/2026-05-17-anthrogo-design.md`
- M1 plan: `docs/superpowers/plans/2026-05-17-anthrogo-m1.md`
- Upstream sources:
  - `restored-src/src/commands.ts` (Slash commands)
  - `restored-src/src/tools/WebFetchTool/`
  - `restored-src/src/tools/WebSearchTool/`
  - `restored-src/src/tools/AskUserQuestionTool/`
  - `restored-src/src/tools/NotebookEditTool/`
  - `restored-src/src/tools/EnterPlanModeTool/`, `ExitPlanModeTool/`
  - `restored-src/src/utils/sessionStorage.ts` (JSONL semantics)
  - `restored-src/src/utils/queryContext.ts` (verbatim system prompt text)

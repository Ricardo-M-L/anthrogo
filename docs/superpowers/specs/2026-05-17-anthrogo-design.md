# anthrogo Design Spec

> Ported-to-Go reconstruction of Anthropic's Claude Code CLI, based on the
> source-mapped TS sources of `@anthropic-ai/claude-code@2.1.88`.
> Reference repo: `opensource-contributions/claude-code-sourcemap/restored-src/src`.

- **Date**: 2026-05-17
- **Status**: M1 design approved, implementation pending
- **Owner**: Ricardo
- **Source-of-truth path**: `~/Documents/公司学习文件/opensource-contributions/claude-code-sourcemap/restored-src/src/`

## 1. Goal

Build a Go implementation of Claude Code that, over time, reaches feature parity
with the upstream TS product (MCP, plugin/skill, hooks, remote sessions, OAuth,
Bedrock/Vertex, vim, voice, KAIROS coordinator, etc.). Faithful architecture is
preferred over premature multi-provider abstraction.

The directory `claude-code-go` already exists as a 12-file proof-of-concept
skeleton; this project (`anthrogo`) supersedes it with a serious, multi-phase port.

## 2. Non-goals (explicit)

- **Not** a multi-provider router. DeepSeek/Kimi/MiniMax/GLM support is reserved
  to M6 behind a `Provider` interface; M1–M5 target Anthropic only.
- **Not** a 1:1 line-by-line transliteration. We re-express the architecture in
  Go idiom (no React/JSX, no `useState`, no Zod), keeping the *shape* of the
  abstractions: Tool / QueryEngine / PermissionContext / ToolUseContext / MCP
  client / hooks / skills / plugins.
- **Not** depending on Bun/Node bundler features (`bun:bundle`, `feature()` flags).
  Feature flags become Go build tags or runtime config.

## 3. Technology choices

| Concern        | Choice                                | Reason                                              |
|----------------|---------------------------------------|-----------------------------------------------------|
| TUI            | `charmbracelet/bubbletea`             | Elm-style update/view aligns with Ink components.   |
|                | `charmbracelet/lipgloss`              | Styling, layout primitives.                         |
|                | `charmbracelet/bubbles`               | Prebuilt textinput/viewport/spinner.                |
|                | `charmbracelet/glamour`               | Markdown rendering in TUI.                          |
| CLI parsing    | `spf13/cobra`                         | Matches commander-js semantics.                     |
| HTTP transport | `net/http` + `r3labs/sse/v2`          | Shared by all providers in M6.                      |
| Anthropic SDK  | `anthropics/anthropic-sdk-go`         | First-party; OAuth/Bedrock/Vertex baked in.         |
| Logging        | `log/slog`                            | Stdlib structured logging.                          |
| Validation     | Stdlib `encoding/json` + hand checks  | No validator dep in M1; revisit only if pain shows. |
| Tracing        | OpenTelemetry (reserved, M3+)         | Mirrors upstream analytics layer.                   |
| Permissions DSL| YAML rules + glob matching            | Replaces upstream JSON `settings.json` rule format. |
| Testing        | `testing` + `testify` if needed       | Stdlib first.                                       |

## 4. Repository layout

```
anthrogo/
├── cmd/
│   └── anthrogo/                # main package; CLI entry, flag wiring, dispatch
│       └── main.go
├── internal/
│   ├── tui/                     # Bubble Tea models (private to the binary)
│   │   ├── app.go               # root model: composes panels
│   │   ├── chat.go              # conversation view, streaming render
│   │   ├── input.go             # prompt input, slash-command palette
│   │   ├── permission.go        # modal dialog for tool permission
│   │   ├── status.go            # statusline
│   │   └── theme.go             # lipgloss themes
│   ├── headless/                # -p stdout / structured-IO path
│   │   └── runner.go
│   ├── config/                  # settings.{json,yaml}, env var precedence
│   │   ├── loader.go
│   │   └── paths.go
│   ├── system/                  # system prompt assembly (mirrors src/context.ts)
│   │   ├── prompt.go            # default Claude Code system prompt builder
│   │   ├── context.go           # git status + CLAUDE.md + currentDate
│   │   └── claudemd.go          # cwd-up walk, dedup, attachment shape
│   ├── session/                 # in-memory M1, file-backed from M2
│   │   └── store.go
│   └── version/
│       └── version.go
├── pkg/
│   ├── message/                 # type defs: User/Assistant/System/Tool blocks
│   │   ├── content.go           # ContentBlock union (text/tool_use/tool_result/image/thinking)
│   │   └── usage.go             # token accounting
│   ├── provider/
│   │   ├── provider.go          # interface: Stream(ctx, req) (<-chan Event, error)
│   │   └── anthropic/           # impl via anthropic-sdk-go
│   │       ├── client.go
│   │       └── stream.go
│   ├── tool/
│   │   ├── tool.go              # Tool interface + ToolDef descriptor
│   │   ├── registry.go          # name → Tool, JSON schema export
│   │   ├── context.go           # ToolUseContext analogue (slim M1, grows later)
│   │   ├── bash.go              # BashTool (with whitelist + sandbox stub)
│   │   ├── read.go              # FileReadTool
│   │   ├── write.go             # FileWriteTool
│   │   ├── edit.go              # FileEditTool (exact string replacement)
│   │   ├── glob.go              # GlobTool (doublestar)
│   │   ├── grep.go              # GrepTool (rg fallback to native)
│   │   └── todowrite.go         # TodoWriteTool (in-mem list)
│   ├── permissions/
│   │   ├── context.go           # PermissionContext (alwaysAllow/Deny/Ask + mode)
│   │   ├── rules.go             # rule matching (tool name + glob args)
│   │   ├── gate.go              # CanUseTool implementation
│   │   └── mode.go              # default/plan/acceptEdits/bypass enum
│   └── query/
│       ├── engine.go            # QueryEngine — owns conversation lifecycle
│       ├── loop.go              # one turn = stream → tool_use? → exec → loop
│       └── event.go             # SDKMessage analogue (assistant/user/system/result)
├── docs/superpowers/specs/
│   └── 2026-05-17-anthrogo-design.md
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

Rationale: `internal/tui` is private to the binary because the TUI is not a
reusable library; `pkg/{provider,tool,permissions,query,message}` are usable by
embedders (e.g. metis, future SDK wrappers). This mirrors how the upstream
project's `src/tools` and `src/services/api` are conceptually reusable while the
Ink layer is application-specific.

## 5. Core abstractions

### 5.1 Tool

Mirrors `src/Tool.ts` (`buildTool` / `ToolDef`), simplified for M1.

```go
package tool

type Tool interface {
    Name() string
    Description(ctx context.Context) (string, error)   // may read context
    Schema() any                                       // JSON schema for the API
    UserFacingName(input map[string]any) string
    Permission(ctx context.Context, input map[string]any) permissions.Decision
    Call(ctx context.Context, input map[string]any, tcx *ToolUseContext) (Result, error)
    IsReadOnly() bool
    IsConcurrencySafe() bool
}

type Result struct {
    Type    ResultType   // text | image | structured
    Text    string
    Image   *ImageBlock
    ForLLM  string       // canonical text seen by the model
    ForUser string       // user-facing summary (TUI render)
    Data    map[string]any
}

type ToolUseContext struct {
    Cwd            string
    Tools          []Tool
    Messages       []message.Message
    AbortContext   context.Context     // engine-scoped cancellation
    ReadFileCache  *file.StateCache
    Permissions    *permissions.Context

    // Surface-injected callbacks. nil-safe — engine code checks before calling.
    RequestPrompt    func(source string, req PromptRequest) (PromptResponse, error)
    AppendUIMessage  func(msg message.UIMessage)
    SendOSNotify     func(msg, kind string)
    SetToolProgress  func(toolUseID string, p Progress)
}
```

`ToolUseContext` is intentionally narrower than upstream's (Tool.ts:158) for
M1 — fields like `setSDKStatus`, `dynamicSkillDirTriggers`, `agentId`,
`queryTracking`, `contentReplacementState` are added in M3/M4/M5 when the
features that need them ship.

We drop the React `renderToolUseMessage` / `renderToolResultMessage` JSX
returns; instead each tool produces a `ForUser` string and structured `Data`,
and the TUI layer owns the rendering.

### 5.2 PermissionContext + Gate

Direct port of `ToolPermissionContext` (Tool.ts:123). Lives in
`pkg/permissions`; `tool.ToolUseContext.Permissions` holds a `*Context`.

```go
type Context struct {
    Mode                         Mode
    AdditionalWorkingDirectories map[string]AdditionalDir
    AlwaysAllowRules             RulesBySource
    AlwaysDenyRules              RulesBySource
    AlwaysAskRules               RulesBySource
    IsBypassAvailable            bool
    ShouldAvoidPrompts           bool
    PrePlanMode                  Mode
}

type Decision struct {
    Behavior      Behavior      // allow | deny | ask
    Reason        string
    UpdatedInput  map[string]any
    SuggestedRule *Rule
}
```

The `gate.go` function `CanUseTool(tool, input, tcx) Decision` is invoked by
`query.engine` before every tool call. `Decision.Behavior == "ask"` is resolved
through `tcx.RequestPrompt(...)` — a callback the surface layer (TUI or
headless) injects when constructing the `ToolUseContext`:

- **REPL surface** wires it to a Bubble Tea modal that suspends input until
  the user picks `allow once / allow always / deny`.
- **Headless surface** (`-p` mode) wires it to an auto-resolver that obeys the
  `ANTHROGO_PERMISSION_MODE` env var or `--permission-mode` flag (`auto`,
  `acceptEdits`, `bypassPermissions`); when none of those allow it, the call
  is denied with a synthetic `tool_result` and the model adapts. Headless
  never prompts on stdin (preserves machine-parseable output).

### 5.3 QueryEngine

Direct port of `src/QueryEngine.ts:184`. Owns one conversation; each
`SubmitMessage(prompt) -> <-chan Event` runs one turn (potentially many
streamed assistant chunks and many tool_use cycles).

```go
type Engine struct {
    cfg        Config
    messages   []message.Message
    abort      context.CancelFunc
    denials    []PermissionDenial
    usage      message.Usage
    readCache  *file.StateCache
}

func (e *Engine) SubmitMessage(ctx context.Context, prompt Prompt) <-chan Event
```

The internal loop:
1. Build system prompt (cached after first turn).
2. Append user message to `messages`.
3. Stream from `provider.Stream(ctx, req)`.
4. On each `text_delta` → emit `Event{Kind: AssistantDelta}`.
5. On `tool_use` complete → permission.Gate → tool.Call → push `tool_result`.
6. If assistant has more `stop_reason == "tool_use"` → next stream call; else
   end turn.

### 5.4 System Prompt

Port of `src/context.ts`. Built lazily and memoized once per turn:

- Default Claude-Code system prompt text (extracted from `src/utils/queryContext.ts`)
- `getSystemContext()` → git status (branch, default branch, short status, log -5, user.name)
- `getUserContext()` → merged CLAUDE.md (cwd walk via `claudemd.go`) + currentDate

### 5.5 Streaming Event Bus

Bubble Tea is single-threaded with messages. The Engine emits `Event`s on a
channel, and the TUI's `Update` consumes them via a `tea.Cmd` that reads the
channel and re-injects each as a `tea.Msg`.

Event kinds: `AssistantDelta`, `AssistantStop`, `ToolUseRequest`,
`ToolResultEmitted`, `PermissionAsk`, `UserCancelled`, `TurnComplete`,
`Error`.

## 6. M1 deliverables

A `go build ./cmd/anthrogo` binary that:

1. Run `anthrogo` (no args) → opens Bubble Tea REPL.
2. Reads `ANTHROPIC_API_KEY` from env (or `~/.anthrogo/settings.yaml`).
3. Walks cwd → parent → ... → home looking for `CLAUDE.md`, merges them.
4. Captures git status (when in a repo) for system context.
5. Streams a turn through `anthropic-sdk-go`, rendering deltas live.
6. Recognises seven tool definitions and exposes them to the model:
   `Bash`, `Read`, `Write`, `Edit`, `Glob`, `Grep`, `TodoWrite`.
7. On `tool_use`: checks permission rules from settings; if `ask`, opens a
   modal showing the tool name + truncated input; on user `y/n/always`,
   executes (or refuses) and feeds `tool_result` back.
8. Run `anthrogo -p "explain main.go"` → headless mode, dumps assistant text
   to stdout, exits when the model emits `end_turn`.
9. `Ctrl+C` cancels the in-flight turn cleanly (abort propagation).
10. Permission rule format (YAML):
    ```yaml
    mode: default
    alwaysAllow:
      - tool: Bash
        match: "git status*"
      - tool: Read
    alwaysDeny:
      - tool: Bash
        match: "rm -rf*"
    ```

Out of M1 scope (deferred): MCP, plugin/skill/hooks, AgentTool subagents,
WebFetch/WebSearch, file session persistence, plan mode UI, vim mode,
remote sessions, compact/snip, KAIROS, voice.

## 7. Data flow (M1 happy path)

```
user types "/list files" + Enter
  │
  ▼
tui/input.go → tea.Cmd{ SubmitMessage(text) }
  │
  ▼
query.Engine.SubmitMessage(ctx, Prompt{ Text: text })
  │
  ├─ system.BuildPrompt() once per turn
  ├─ messages = append(messages, User{...})
  └─ provider.anthropic.Stream(ctx, req) → <-chan SDKEvent
       │
       ├─ text_delta  →  Event{ AssistantDelta(t) } → tui chat append
       ├─ tool_use(complete)
       │    ├─ permissions.Gate(tool, input, tcx) → Decision
       │    │    └─ Decision.ask → tui.permission modal → user y/n
       │    └─ tool.Call() → Result
       │         └─ messages = append(messages, ToolResult)
       └─ stop_reason == "tool_use" → loop back to Stream(...)
       └─ stop_reason == "end_turn" → Event{ TurnComplete }
```

## 8. Error handling

| Class                          | Strategy                                                         |
|--------------------------------|------------------------------------------------------------------|
| 429 / 529 (overloaded)         | Exponential backoff w/ jitter (mirrors `src/services/api/errors`)|
| 401 (auth)                     | Surface "Run `anthrogo auth`" hint, exit headless / dialog REPL  |
| Tool exec failure              | Convert to `tool_result` with `is_error: true`, model retries    |
| Permission deny                | Tool returns synthetic error block; model adapts                 |
| User abort (Ctrl+C)            | Cancel root context, drain in-flight tool, emit `Event{Aborted}` |
| Network drop mid-stream        | Surface partial assistant text + error; allow retry on next turn |
| Bash tool malicious command    | `bashSecurity` port — blocklist + glob check + sandbox flag      |

The upstream's `permissionDenials` tracking on `QueryEngine` is preserved so
that automated agents (M5) can see denial frequency.

## 9. Testing strategy

- **Unit**: tool input validation, permission gate, CLAUDE.md walker, system
  prompt assembly, rule matcher.
- **Integration**: a fake `Provider` (`pkg/provider/fake`) drives the engine
  through canned tool_use sequences; tests assert on the resulting message
  log and tool side-effects in a `t.TempDir()` workspace.
- **End-to-end**: a `testscript`-style harness in `cmd/anthrogo` that pipes
  a scripted user input into headless mode against the fake provider.

No mocks of the database / filesystem in the BashTool integration tests —
they execute in `t.TempDir()`. Following [feedback memory: integration tests
must hit real I/O].

## 10. Phased roadmap

| Phase | Scope                                                                                    |
|-------|------------------------------------------------------------------------------------------|
| M1    | This spec. Bubble Tea REPL + 7 tools + Anthropic SDK + permission + CLAUDE.md + headless |
| M2    | WebFetch / WebSearch / TodoRead / AskUserQuestion / NotebookEdit; session persistence;   |
|       | plan mode (`EnterPlanMode` / `ExitPlanMode`); slash-command palette; settings sync.      |
| M3    | MCP client (stdio + SSE + websocket); dynamic tool registration; MCP elicitations;       |
|       | OAuth for managed MCP; `mcp__*` tool name namespacing.                                   |
| M4    | Hooks (UserPromptSubmit / PreToolUse / PostToolUse / Stop / SessionStart / SessionEnd);  |
|       | skill discovery (`SkillTool`, dir-watched); plugin system (`init`, bundled, third-party).|
| M5    | AgentTool (subagents); team/task tools (Create/Get/Update/Stop/Output/List); worktree    |
|       | tools; KAIROS / coordinator modes; remote-session client (CCR); compact / snip.          |
| M6    | OAuth flow, Bedrock, Vertex; OpenAI-compat / DeepSeek / Kimi / MiniMax / GLM providers   |
|       | (each a `provider.Provider` impl, with shared tool-call adapter).                        |

Each phase ends with a `vX.Y.0` tag, a `CHANGELOG.md` entry, and `make test`
passing.

## 11. Risks / open questions

1. **TUI fidelity**: upstream Ink uses fine-grained component re-renders and
   measure-pass layout (`src/ink/layout`). Bubble Tea's full-frame render
   model is coarser. **Mitigation**: keep the visible viewport small; render
   diffs by re-rendering only the active view.
2. **System prompt drift**: the canonical Claude Code system prompt text
   lives in upstream's `utils/queryContext.ts` and shifts version to version.
   **Mitigation**: pin to 2.1.88 text; track upstream `.tgz` releases in a
   `tools/refresh-prompt.sh` script.
3. **Tool input schemas**: upstream uses Zod; we'd export JSON Schema from Go
   structs (`mark3labs/jsonschema-go` or hand-rolled). Validate against the
   real API to avoid silent schema drift.
4. **Streaming back-pressure**: a long Bash command should not block the TUI
   render loop. **Mitigation**: tool execution runs in goroutine, progress
   events flow through the same Event channel; `Update` is non-blocking.
5. **Cross-platform**: upstream supports macOS / Linux / Windows. Bash tool
   on Windows uses PowerShell wrapper (`src/tools/PowerShellTool`) —
   honestly deferred to M5; M1 = macOS + Linux only.

## 12. References

| File                                                  | Purpose                              |
|-------------------------------------------------------|--------------------------------------|
| `restored-src/src/main.tsx`                           | CLI entry / commander wiring         |
| `restored-src/src/QueryEngine.ts`                     | Turn loop (1295 lines)               |
| `restored-src/src/Tool.ts`                            | Tool / ToolUseContext / Permission   |
| `restored-src/src/tools.ts`                           | Built-in tool registry               |
| `restored-src/src/context.ts`                         | System / user context builder        |
| `restored-src/src/services/api/claude.ts`             | Streaming Anthropic client           |
| `restored-src/src/tools/BashTool/*`                   | Bash sandbox / sed / permissions     |
| `restored-src/src/ink/`                               | Layout engine reference              |

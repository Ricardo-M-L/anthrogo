# anthrogo M3 Design Spec

> **Date**: 2026-05-17
> **Status**: design pending user approval
> **Owner**: Ricardo
> **Builds on**: M2 (`2026-05-17-anthrogo-m2-design.md`, v0.2.0-dev)

## 1. Goal

Connect anthrogo to the MCP ecosystem so it can drive third-party tool servers (filesystem, fetch, mermaid, firecrawl, etc.) the same way Claude Code does. From the model's perspective, MCP tools should be indistinguishable from built-in tools — they appear in the tool list, can be called, return results, and obey the same permission gate.

## 2. Non-goals

- **SSE / WebSocket / HTTP-streaming transports** (M4). Stdio is the most common transport for local dev and covers ~90% of public servers.
- **OAuth-managed MCP** (M4). Anthropic-hosted MCP servers that require OAuth flows are deferred until we have OAuth infrastructure.
- **Elicitations** (M4). Server-initiated user prompts (`elicitation/create`) need UX design.
- **Resources** (M5). `resources/list` + `resources/read` (e.g., for accessing server-managed files) is not part of M3; M3 is tools-only.
- **Prompts** (M5). MCP server-provided prompt templates.
- **Real `/compact`** (M4). Stays as the M2 placeholder.
- **Sampling** (out of scope). Reverse-direction LLM calls from server to client are explicitly not supported.
- **Channel allowlists** (M4). Per-server permission scopes ship after the basic plumbing is stable.

## 3. Scope (locked)

1. **MCP client core** via official `github.com/modelcontextprotocol/go-sdk` (v1.6.0).
2. **Stdio transport** only. Server is spawned as a subprocess; messages are JSON-RPC 2.0 framed by newlines on stdin/stdout.
3. **Per-server lifecycle**: spawn on app start → `initialize` handshake → `tools/list` → tools registered → live for the duration of the process → graceful `notifications/cancelled` + close on quit.
4. **Tool name namespacing**: each MCP tool is exposed as `mcp__<server>__<tool>` in the registry, mirroring upstream Claude Code's convention. Tool-name collisions across servers are prevented by this prefix.
5. **Settings DSL**: new `mcpServers:` stanza in `~/.anthrogo/settings.yaml` listing servers, command + args + env.
6. **Background server logs**: `notifications/message` from a server is rendered into the TUI chat as a faded status line; in headless mode it goes to stderr.
7. **TUI command**: `/mcp` lists servers + their tool counts + connection state; `/mcp reload` re-spawns all servers (M3.E.6).
8. **Errors**: a server that fails to initialize is logged + skipped; anthrogo still runs. Per-server crash mid-session marks its tools as unavailable until reload.

## 4. Architecture

### 4.1 Package layout

```
internal/
  mcp/
    manager.go        # MCPManager: spawn N servers, expose merged tool list
    manager_test.go
    server.go         # one server's lifecycle (subprocess + sdk client)
    server_test.go
    config.go         # MCPServerConfig (yaml shape)
    log_sink.go       # routes server notifications/log → tui callback
  command/builtins/
    mcp.go            # /mcp + /mcp reload
pkg/
  tool/
    mcp_adapter.go    # implements tool.Tool wrapping one mcp tool descriptor
    mcp_adapter_test.go
internal/config/loader.go    # MODIFY: + MCPServers map
internal/system/prompt.go    # MODIFY: prompt mentions MCP tools
internal/tui/app.go          # MODIFY: receive log_sink output
internal/tui/chat.go         # MODIFY: appendServerLog style
cmd/anthrogo/main.go         # MODIFY: bring up MCPManager and merge tools
docs/superpowers/specs/...
```

### 4.2 Public types

```go
// internal/mcp/config.go
type MCPServerConfig struct {
    Command string            `yaml:"command"`
    Args    []string          `yaml:"args"`
    Env     map[string]string `yaml:"env"`
    Cwd     string            `yaml:"cwd,omitempty"`
    Timeout time.Duration     `yaml:"timeout,omitempty"` // init handshake timeout
}

// internal/mcp/manager.go
type Manager struct {
    servers map[string]*Server // keyed by server name (also tool prefix)
    logSink func(name, msg string)
}

type Server struct {
    Name    string
    cfg     MCPServerConfig
    client  *sdk.ClientSession   // from modelcontextprotocol/go-sdk
    tools   []*sdk.Tool          // result of tools/list
    state   State                // initializing | ready | failed | closed
    err     error                // last error for failed state
}

type State string
const (
    StateInit   State = "init"
    StateReady  State = "ready"
    StateFailed State = "failed"
    StateClosed State = "closed"
)

// pkg/tool/mcp_adapter.go
type MCPAdapter struct {
    DefaultPermission
    serverName string
    descriptor *sdk.Tool
    invoker    func(ctx context.Context, args map[string]any) (Result, error)
}
```

### 4.3 Tool name format

`mcp__<server>__<tool>` — e.g., `mcp__filesystem__read_file`. Underscores chosen because they're allowed in JSON Schema property names and don't conflict with any built-in tool. `MCPAdapter.Name()` returns this composite; `tool.Registry.Register(adapter)` stores it like any other tool.

### 4.4 Lifecycle data flow

```
anthrogo boots
  │
  ▼
config.Load() → cfg.MCPServers map[string]MCPServerConfig
  │
  ▼
mcp.NewManager(cfg, logSink) → spawn one *Server per entry, in parallel
  │
  └── for each Server:
       ├─ exec.CommandContext(ctx, cfg.Command, cfg.Args...) → stdin/stdout pipes
       ├─ sdk.NewClient + sdk.NewStdioTransport
       ├─ client.Initialize(timeout=cfg.Timeout || 10s)
       ├─ on success: tools, _ := client.ListTools(); state=ready
       └─ on failure: state=failed, err recorded
  │
  ▼
mgr.AllTools() → []tool.Tool  (one MCPAdapter per ready server's tool)
  │
  ▼
cmd/anthrogo merges built-ins + MCP tools into tool.Registry
  │
  ▼
tui.New(tools, ...) — model sees the unified list
  │
  ▼
... user types a prompt ...
  │
  ▼
engine emits tool_use for "mcp__filesystem__read_file"
  │
  ▼
permissions.Decide runs the gate (rules can target mcp__* like any tool name)
  │
  ▼
adapter.Call → invoker → client.CallTool(name="read_file", args=...)
  │
  ▼
result back through tool_result block
  │
  ▼
on app quit: mgr.Close() — sends notifications/cancelled + closes each subprocess
```

### 4.5 Per-server notifications/log handling

SDK exposes `client.SetNotificationHandler` (or similar). We wire it to `manager.logSink(serverName, message)`. The TUI's `logSink` appends a dimmed line to the chat viewport: `[mcp:filesystem] reading /tmp/foo`. Headless mode writes to stderr.

### 4.6 Settings YAML

```yaml
mcpServers:
  filesystem:
    command: npx
    args: ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  fetch:
    command: npx
    args: ["-y", "@modelcontextprotocol/server-fetch"]
    env:
      USER_AGENT: anthrogo/0.3
```

`internal/config.Config` gains:

```go
MCPServers map[string]mcp.MCPServerConfig `yaml:"mcpServers,omitempty"`
```

The MCPServerConfig type lives in `internal/mcp` (not in `internal/config`) to keep the config-loader from importing the MCP runtime; loader uses the struct as data only. Acceptable since both are `internal/...`.

### 4.7 Permission gate integration

No gate changes needed. Rules already target tools by exact `Name`. Users can add:

```yaml
alwaysAllow:
  - tool: mcp__filesystem__read_file
  - tool: mcp__fetch__fetch
    match: "https://docs.*"
alwaysDeny:
  - tool: mcp__filesystem__write_file
```

Plan-mode hard-lock: `IsWriteTool` (M2) is built-in-tools-only. MCP write-style tools won't be auto-locked in plan mode — the user must add explicit deny rules. **This is intentional**: anthrogo can't know a server's tool semantics; classifying MCP writes is M5 work alongside the BashSecurity rewrite.

### 4.8 `/mcp` slash command

```
/mcp           → list servers with state + tool count
/mcp reload    → close all, re-spawn from current config
/mcp status <server>  → detailed status incl. last error
```

Implementation in `pkg/command/builtins/mcp.go`. Uses `command.Host` interface — adds `MCP() *mcp.Manager` accessor.

### 4.9 System prompt

The system prompt's "Available tools" line already lists every name; MCP tools naturally appear. We add a one-liner noting that `mcp__*` tools are provided by external servers and may have varying reliability.

## 5. Error handling

| Case                                        | Behavior                                                                                  |
|---------------------------------------------|-------------------------------------------------------------------------------------------|
| Server `command` not on PATH                | Manager logs error; server marked failed; other servers continue; binary starts normally  |
| `initialize` handshake timeout (default 10s)| Server marked failed; tools list empty                                                    |
| Server crashes mid-session                  | Adapter calls return synthetic `tool_result{is_error: true, text: "mcp server X exited"}` |
| `tools/call` returns error                  | Pass through as `tool_result{is_error: true}` so the model can adapt                      |
| Server emits a notification we don't grok   | Log to stderr (debug) + skip                                                              |
| Closing on quit fails                       | Best-effort; SIGTERM then SIGKILL after 2s                                                |
| Settings YAML mcpServers section invalid    | Loader returns error; binary exits with helpful message                                   |

## 6. Testing strategy

- **Unit**: `internal/mcp/config_test.go` for YAML round-trip, `pkg/tool/mcp_adapter_test.go` for the adapter shape.
- **Integration**: a fake stdio MCP server is included under `internal/mcp/testdata/` — a Go program in a sub-package that implements minimal initialize + tools/list + tools/call. The manager spawns it, lists tools, and calls one. This proves the SDK path works end-to-end without needing npm or external servers in CI.
- **Smoke (gated)**: optional live test against `npx @modelcontextprotocol/server-everything` when `ANTHROGO_LIVE_MCP=1` is set. Skipped by default to keep `go test ./...` hermetic.

## 7. Phased delivery (within M3)

| Phase | Scope                                                                                | Approx tasks |
|-------|--------------------------------------------------------------------------------------|--------------|
| M3.A  | go-sdk dep + `MCPServerConfig` + YAML loader extension                               | 2            |
| M3.B  | `internal/mcp.Server` lifecycle (spawn / init / list / call / close), unit-tested    | 4            |
| M3.C  | `MCPAdapter` (tool.Tool impl) + adapter tests against fake server                    | 2            |
| M3.D  | `Manager` orchestration + log_sink + parallel spawn                                  | 3            |
| M3.E  | Wire MCP tools into `cmd/anthrogo`, `/mcp` builtin (status + reload), TUI log render | 4            |
| M3.F  | Fake MCP server testdata + end-to-end integration test + acceptance                  | 3            |

Total ~18 tasks. Smaller than M1/M2 because the SDK absorbs the protocol implementation.

## 8. Risks / open questions

1. **SDK API drift**: `modelcontextprotocol/go-sdk` v1.6.0 is the current stable, but the API surface (especially around transport setup and notification handlers) could differ from what the design assumes. Each implementer task that touches `sdk.*` must verify symbol names against `go doc`. Same pattern as M1 anthropic-sdk-go.
2. **Subprocess lifetime on user-interrupt**: if the user `Ctrl+C`s the binary, MCP server children should die. Use `cmd.SysProcAttr` + a parent-pgid kill on shutdown. Tested on macOS + Linux; Windows deferred to M5 alongside PowerShell.
3. **First-spawn cost**: each `npx ...` server costs 1-5 seconds to boot. M3 **blocks startup** waiting for all servers to either reach `ready` or hit the per-server timeout (default 10s, configurable per server). Servers spawn in parallel, so total wait time is `max(per-server-init-time, 10s)` rather than the sum. A bad server delays startup by at most its timeout. M4 may move slow servers to background-init.
4. **JSON Schema strictness**: server-provided tool schemas may use validations our `pkg/tool.Schema()` JSON shape doesn't fully express (e.g., `oneOf`, `$ref`). We pass them straight through to the Anthropic API; the model handles it. If a server schema is malformed, we surface an error at registration time.
5. **Tool name length**: Anthropic's tool name limit is **64 chars** (matches `^[a-zA-Z0-9_-]{1,64}$`). `mcp__<server>__<tool>` budget: `mcp__` (5) + `<server>` + `__` (2) + `<tool>` ≤ 64, so `len(server) + len(tool) ≤ 57`. We allocate **16 chars** to the server name and **41 chars** to the tool name; anything longer is truncated at registration with a log warning naming the original. (`5 + 16 + 2 + 41 = 64`, exact fit.) A SHA-8 suffix is appended on truncation to avoid collisions: `mcp__<server[:16]>__<tool[:33]>_<8-hex>` = `5+16+2+33+1+8 = 65` — too long; use `tool[:32]_<8-hex>` instead = `5+16+2+32+1+8 = 64`. The truncation helper lives in `pkg/tool/mcp_adapter.go` and is unit-tested.
6. **Concurrency**: each `Server` has its own SDK client which serialises in-flight requests. Multiple servers run in parallel. The Manager exposes them via a `sync.RWMutex`.

## 9. References

- M2 spec: `docs/superpowers/specs/2026-05-17-anthrogo-m2-design.md`
- Go SDK: `github.com/modelcontextprotocol/go-sdk@v1.6.0`
- Upstream sources (read-only reference):
  - `restored-src/src/services/mcp/client.ts` (lifecycle)
  - `restored-src/src/services/mcp/config.ts` (settings shape)
  - `restored-src/src/services/mcp/elicitationHandler.ts` (M4 reference)
  - `restored-src/src/tools/MCPTool/` (tool name format)
  - `restored-src/src/services/mcp/headersHelper.ts` (M4 SSE auth)

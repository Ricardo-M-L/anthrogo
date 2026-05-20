# anthrogo M4.5 — MCP debt sweep

**Status:** approved (self-authorized)
**Date:** 2026-05-20
**Predecessor:** M4.4

## 1. Goal

Close out the M3 MCP debt list that's been carried as known-issues across M4. Scope is the **three smallest, highest-value items**; the larger items (OAuth, WebSocket, elicitations, resources) defer to M5.

## 2. In-scope (3 items)

### Item 1: `/mcp reload` truly refreshes the tool registry

Today `/mcp reload` restarts subprocesses but the `tool.Registry` is frozen from startup — reloaded servers' tool list changes never reach the model. Fix: after `mcpMgr.Close() + mcpMgr.Start(ctx)`, **remove every existing `mcp__*` tool from `tool.Registry`, then re-register from `mcpMgr.AllTools()`**.

Requires a new `tool.Registry.RemoveByPrefix(prefix string) int` method (returns count removed). Then the `/mcp` builtin (currently in `pkg/command/builtins/mcp.go`) calls `host.Tools().RemoveByPrefix("mcp__") + for _, t := range host.MCP().AllTools() { host.Tools().Register(t) }`.

The model's system prompt is still built at startup; mention this limitation in the reload's confirmation message ("note: model's system prompt was built at startup and still lists original tools; restart to refresh model awareness").

### Item 2: plan-mode `IsWriteTool` recognizes `mcp__*` write tools

Currently `pkg/permissions.IsWriteTool` only recognizes the names `Write`, `Edit`, `NotebookEdit`. Plan mode therefore lets any MCP tool through. MCP tools can have arbitrary side effects; the conservative behavior in plan mode is **treat every `mcp__*` tool as a write tool** (unless the user explicitly allowed it via plan-mode-aware rules in a future milestone). Fix: extend `IsWriteTool(name)` to return true when `strings.HasPrefix(name, "mcp__")`.

Document this in the README plan-mode section + CHANGELOG: "Plan mode now blocks all MCP tool calls by default. To run a specific MCP tool in plan mode, exit plan mode (`/mode default`)."

### Item 3: MCP HTTP transports — SSE + Streamable

Today `MCPServerConfig` only supports stdio (spawn subprocess). The SDK ships `SSEClientTransport` (2024-11-05 spec) and `StreamableClientTransport` (newer spec) as drop-in `Transport` implementations with the same `Connect(ctx)` contract.

Extend the config:

```yaml
mcpServers:
  my-stdio:
    command: npx
    args: [...]
  my-sse:
    type: sse
    endpoint: https://example.com/mcp
  my-streamable:
    type: streamable
    endpoint: https://example.com/mcp
    max_retries: 3
```

`MCPServerConfig` gains:

```go
type MCPServerConfig struct {
    Type       string            `yaml:"type,omitempty"`  // "stdio" (default) | "sse" | "streamable"
    Command    string            `yaml:"command,omitempty"`
    Args       []string          `yaml:"args,omitempty"`
    Env        map[string]string `yaml:"env,omitempty"`
    Cwd        string            `yaml:"cwd,omitempty"`
    Endpoint   string            `yaml:"endpoint,omitempty"` // for sse / streamable
    MaxRetries int               `yaml:"max_retries,omitempty"` // for streamable
    Timeout    time.Duration     `yaml:"timeout,omitempty"`
}
```

`Server.Start` picks the transport based on `cfg.Type`:

```go
switch cfg.Type {
case "", "stdio":
    transport = &sdk.CommandTransport{Command: cmd, TerminateDuration: 2 * time.Second}
case "sse":
    transport = &sdk.SSEClientTransport{Endpoint: cfg.Endpoint}
case "streamable":
    transport = &sdk.StreamableClientTransport{Endpoint: cfg.Endpoint, MaxRetries: cfg.MaxRetries}
default:
    return error
}
```

For non-stdio types, the subprocess setup (`cmd := exec.Command`, `Setpgid`) is skipped, and `Server.Close` only closes the session (no SIGTERM needed).

Validation: `Type=="stdio"` requires Command; `Type=="sse"||"streamable"` requires Endpoint; mismatch = state Failed at Start with a clear error.

## 3. Out-of-scope (deferred to M5)

- WebSocket transport (SDK has no native WebSocketClientTransport)
- OAuth 2.1 client flow for HTTP transports
- Elicitations (server-initiated user prompts via MCP)
- Resources (`resources/list`, `resources/read`)

These get a fresh spec in M5.

## 4. Testing

- `pkg/tool/registry_test.go`: TestRegistry_RemoveByPrefix
- `pkg/command/builtins/mcp_test.go`: extend reload test to assert tools are actually re-registered (use a fake Manager / Registry combo or just exercise the in-process flow)
- `pkg/permissions/plan_test.go`: TestIsWriteTool_MatchesMCPPrefix
- `internal/mcp/server_test.go` (or extend existing manager_test.go): TestServer_Start_RejectsBadType, TestServer_Start_SSETransportSelected (probably no real e2e — just assert no panic and that `Type == "sse" + Endpoint == ""` returns error)

## 5. Implementation order

1. `tool.Registry.RemoveByPrefix` + unit test (smallest, clear contract)
2. `IsWriteTool` extension + test
3. `MCPServerConfig` Type field + Server.Start transport switch + tests
4. `/mcp reload` wire to RemoveByPrefix + re-register from mcpMgr.AllTools()
5. Docs + version 0.4.4-dev

## 6. Acceptance

- `go build/vet/test/-race` clean
- 3× uncached full-repo sweep clean
- `./bin/anthrogo --version` → `0.4.4-dev`
- README plan-mode + MCP sections updated
- CHANGELOG entry under `[0.4.4-dev] — 2026-05-20`

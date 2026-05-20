# anthrogo M5.2 — MCP elicitations + resources

**Status:** approved (self-authorized)
**Date:** 2026-05-20
**Predecessor:** M5.1

## 1. Goal

Two more items off the M3 MCP debt list:
- **Resources**: model can list and read MCP-server-provided resources via a new built-in tool `MCPResource`.
- **Elicitations**: an MCP server can request structured form input from the user. M5.2 ships a *minimal* handler that records the request and returns `decline`; full TUI form integration defers to M5.3.

WebSocket transport and OAuth 2.1 client flow are out of scope (no native SDK support — they need separately scoped transport implementations) — defer to M5.3.

## 2. Resources

### Server lifecycle additions

`internal/mcp/server.go`:
- New method `ListResources(ctx context.Context) ([]*sdk.Resource, error)` — calls `session.ListResources(ctx, &sdk.ListResourcesParams{})`, returns the slice (handles pagination by following NextCursor if present).
- New method `ReadResource(ctx context.Context, uri string) (*sdk.ReadResourceResult, error)` — calls `session.ReadResource(ctx, &sdk.ReadResourceParams{URI: uri})`.
- Both methods return error if state != Ready.

### Manager

`internal/mcp/manager.go`:
- New method `AllResources(ctx context.Context) map[string][]*sdk.Resource` — iterates Ready servers, calls ListResources, returns by server name. Errors per-server are logged via LogSink, not propagated; missing entries indicate failure.
- New method `ReadResource(ctx context.Context, server, uri string) (*sdk.ReadResourceResult, error)` — finds named server, delegates.

### `MCPResource` tool

`pkg/tool/mcpresource.go`:
- `MCPResource` tool. Schema `{server: string, uri?: string}`.
- If `uri` is empty: list resources from `server`, return JSON-encoded list (name, description, mimeType, uri, size).
- If `uri` is set: read the resource, return its text content (or `[binary blob, N bytes]` for blob).
- Errors: server not found, server not ready, list/read failure → IsError result.
- `Permission()` defers to gate (DefaultPermission). Default `alwaysAllow: [{Tool: "MCPResource"}]` at CLI level (same pattern as Skill tool from M4.3) so the gate respects deny rules + hooks but doesn't ask per-call.
- `IsReadOnly() == true`.

### Construction
The tool needs the Manager. Inject via `tool.NewMCPResource(mgr)`.

### System prompt
After the MCP servers list, list resources per server (capped at 50 per server to avoid prompt bloat):

```
Available MCP resources (use the MCPResource tool to inspect or read):
- fs:
  - file:///tmp/notes.md (text/markdown, 1.2 KB) — Daily notes
  - file:///tmp/log.txt (text/plain, 4.0 KB)
```

`internal/system.Options` gains `MCPResources map[string][]*sdk.Resource`.

cmd/anthrogo passes `mcpMgr.AllResources(ctx)` (with a short timeout) to BuildSystemPrompt at startup.

## 3. Elicitations (minimal handler)

### Server lifecycle
`internal/mcp/server.go` `Start`: when constructing `sdk.ClientOptions`, set:

```go
ElicitationHandler: func(ctx context.Context, req *sdk.ElicitRequest) (*sdk.ElicitResult, error) {
    msg := fmt.Sprintf("server %s requested elicitation: %q", s.Name, req.Params.Message)
    if s.notifyLog != nil { s.notifyLog(s.Name, msg) }
    return &sdk.ElicitResult{Action: "decline"}, nil
},
```

This advertises the elicitation capability but always declines. Servers that need form input from the user will see decline; they're expected to handle decline gracefully.

A future M5.3 expansion replaces the body with an actual TUI form-prompt flow.

### Config

Add to `MCPServerConfig`:

```go
ElicitationMode string `yaml:"elicitation_mode,omitempty"` // "decline" (default) | "disabled"
```

- `"decline"` (default): the handler is registered; capability advertised; all requests get `decline`.
- `"disabled"`: handler not registered (capability not advertised).

Validation: any other value → log warning + treat as `"decline"`.

### Tests

Verify the handler is wired with `decline` semantics in `internal/mcp/server_test.go` (can be done with the embedded echo-server extended to emit one elicitation request and assert the result; or skip the e2e and just unit-test the handler closure shape).

## 4. Out of scope (M5.3)

- WebSocket transport — no native SDK support; needs custom Transport implementation (likely using `gorilla/websocket` or `nhooyr.io/websocket`).
- OAuth 2.1 client flow — needs a separate OAuth client + redirect handling + token storage.
- Full elicitation TUI form integration — needs structured form UI in the bubbletea modal layer.
- Resource notifications (`notifications/resources/list_changed`) — current code is request-response only.

## 5. Code organization

```
pkg/tool/mcpresource.go         # new
pkg/tool/mcpresource_test.go    # new
internal/mcp/server.go          # extend Start + add ListResources/ReadResource
internal/mcp/manager.go         # extend with AllResources/ReadResource
internal/mcp/config.go          # add ElicitationMode field
internal/system/prompt.go       # add MCPResources option + emit section
cmd/anthrogo/main.go            # register MCPResource tool + ship default alwaysAllow rule + pass MCPResources to prompt
```

## 6. Testing

- `pkg/tool/mcpresource_test.go`: schema sanity; list path (mock manager returning fixture resources); read path; unknown server → IsError; ReadResource error → IsError.
- `internal/mcp/server_test.go`: TestServer_Start_RegistersElicitationHandlerByDefault, TestServer_Start_ElicitationModeDisabled_SkipsHandler. Use the existing echo-server harness if practical; or instantiate options directly and inspect non-nil.
- `internal/system/prompt_test.go`: TestBuildSystemPrompt_ListsMCPResources.

## 7. Acceptance

- `go build/vet/test/-race` clean
- 3× uncached sweep clean
- Version 0.5.1-dev
- CHANGELOG + README updates

## 8. Deferred to M5.3

- WebSocket transport
- OAuth 2.1 client
- Full TUI form-based elicitation handler
- Resource list_changed notifications
- Resource subscription

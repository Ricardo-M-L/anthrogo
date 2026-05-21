# MCP

anthrogo implements the Model Context Protocol (MCP) client with four transport types. Add servers to `~/.anthrogo/settings.yaml` under `mcpServers:`.

## Quick start

```yaml
mcpServers:
  filesystem:
    command: npx
    args: ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
```

`type` defaults to `stdio`. Tools surface as `mcp__<server>__<tool>` (names exceeding 64 chars get a sha-8 suffix).

## Four transports

| Type | Use case | Key field |
|------|----------|-----------|
| `stdio` (default) | Local subprocess | `command`, `args` |
| `sse` | 2024-11-05 SSE remote | `endpoint` |
| `streamable` | Streamable HTTP remote | `endpoint` |
| `websocket` | Custom WS (ws:// or wss://) | `endpoint`, `subprotocols` |

Full config examples:

```yaml
mcpServers:
  # stdio (default)
  filesystem:
    command: npx
    args: ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]

  # streamable HTTP
  remote-fetch:
    type: streamable
    endpoint: https://example.com/mcp
    max_retries: 3
    headers:
      X-Api-Key: env:MY_API_KEY

  # legacy SSE
  legacy-sse:
    type: sse
    endpoint: https://legacy.example.com/mcp
    headers:
      Authorization: "Bearer static-token"

  # WebSocket
  ws-server:
    type: websocket
    endpoint: wss://example.com/mcp
    subprotocols: ["mcp"]
    headers:
      X-Tenant-Id: "acme"
```

## Headers

`headers` injects arbitrary HTTP headers on every outgoing request for `sse`, `streamable`, and `websocket` transports.

- Values support `env:VARNAME` expansion (resolved once at startup): `X-Api-Key: "env:MY_API_KEY"` reads `$MY_API_KEY`
- `/mcp status <name>` redacts sensitive values — any key containing `authorization`, `auth`, `token`, `key`, `secret`, `password`, or `bearer` (case-insensitive) shows as `<redacted>`

When `oauth:` is also set, headers are applied first and the OAuth `Authorization: Bearer` is layered on top.

## OAuth 2.1 PKCE

For HTTP/WebSocket servers requiring authentication:

```yaml
mcpServers:
  api-with-oauth:
    type: streamable
    endpoint: https://example.com/mcp
    oauth:
      authorization_url: https://example.com/oauth/authorize
      token_url: https://example.com/oauth/token
      client_id: my-anthrogo-app
      scopes: [mcp.read, mcp.write]
      redirect_port: 8765   # local loopback for the OAuth callback (default 8765)
      # client_secret: optional for confidential clients
```

Flow when `oauth:` is set:
1. Checks `~/.anthrogo/oauth/<server-name>.json` for a cached token
2. If valid (not expired within a 30s margin), reuses it
3. If expired with a `refresh_token`, attempts a refresh-token grant
4. Otherwise: opens the browser to `authorization_url`, starts a local HTTP listener on `redirect_port`, exchanges the code + PKCE verifier for tokens, caches the result
5. Access token injected as `Authorization: Bearer <token>` on every request

## WebSocket subprotocols

`subprotocols` (websocket only) advertises listed subprotocols during the WebSocket handshake (`Sec-WebSocket-Protocol`). The server's chosen subprotocol is echoed back.

## MCP resources

anthrogo lists resources advertised by Ready servers in the system prompt at startup. The model uses the built-in `MCPResource` tool to access them:

```json
// List resources on a server
{"server": "filesystem"}
→ [{uri, name, description, mime_type, size}, ...]

// Read a resource
{"server": "filesystem", "uri": "file:///tmp/notes.md"}
→ resource text
```

`MCPResource` has a default `alwaysAllow` rule (read-only; deny rules and `PreToolUse` hooks still take precedence).

## Elicitations

When an MCP server sends an `elicitation/create` request, TUI users get a form modal:

- **Multi-field mode**: schema is a flat `object` with all primitive properties (`string`, `number`, `integer`, `boolean`) or `enum` string arrays. Each property renders as its own input row. Tab/Shift-Tab moves focus; Enter submits; Esc cancels. Schema `default` values pre-fill buffers.
- **Single-textarea fallback**: schemas with nested objects, arrays, or multi-select enums fall back to a JSON-blob textarea. Type a JSON object matching the schema, press Enter.

Headless mode (`-p`) always declines. Disable elicitation capability per-server:

```yaml
mcpServers:
  filesystem:
    command: npx
    args: ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
    elicitation_mode: "disabled"
```

## Slash commands

```
/mcp                    # list all servers and their status
/mcp status <name>      # show one server's last error and headers (redacted)
/mcp reload             # remove and re-register all mcp__* tools
```

Note: `/mcp reload` re-registers tools but the model's system prompt is built at startup — restart anthrogo to refresh model awareness of newly-added servers.

## Plan mode

Plan mode blocks all MCP tool calls (`mcp__*` tools are treated as write tools). Use `/mode default` to invoke MCP tools.

## list-changed notifications

anthrogo registers `notifications/tools/list_changed` handlers. When a server pushes the notification, anthrogo refreshes its per-server tool cache and logs the event. The model-facing tool registry is NOT auto-rebuilt (would race with in-flight turns); run `/mcp reload` to surface new tools.

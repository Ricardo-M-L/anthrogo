# MCP

See [README — MCP](https://github.com/Ricardo-M-L/anthrogo#mcp).

anthrogo implements the Model Context Protocol (MCP) client with four transport types.

## Transports

| Transport | Use case |
|---|---|
| `stdio` | Local process (most common) |
| `sse` | Remote server-sent events |
| `streamable-http` | Remote HTTP streaming |
| `websocket` | Real-time bidirectional (+ OAuth 2.1 PKCE) |

## Configuring MCP servers

In `settings.yaml`:

```yaml
mcp:
  servers:
    - name: my-server
      transport: stdio
      command: ["./mcp-server"]
```

(Full MCP reference migrating from README — M11.4 follow-up.)

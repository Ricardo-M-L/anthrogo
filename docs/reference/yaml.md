# Settings YAML reference

Default location: `~/.anthrogo/settings.yaml` (override via `ANTHROGO_HOME`).

See [Configuration](../configuration.md) for layered config rules, environment variable expansion, and system prompt overlays.

## Top-level keys

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `mode` | string | `default` | `default` / `acceptEdits` / `bypassPermissions` / `plan` |
| `model` | string | provider default | Model ID override |
| `provider` | string | `anthropic` | Active profile name |
| `apiKey` | string | — | Anthropic API key (or `env:VAR`) |
| `auto_compact_threshold` | int | 0 | Auto-compact at N tokens since last compact; 0 = disabled |
| `cost_limit_usd` | float | 0 | Budget cap in USD; 0 = disabled |
| `providers_failover` | []string | — | Ordered list of profile names to try on pre-commit error |

## alwaysAllow / alwaysDeny / alwaysAsk

```yaml
alwaysAllow:
  - tool: Read
  - tool: Bash
    match: "git *"        # glob against full tool input; omit to match all calls

alwaysDeny:
  - tool: Bash
    match: "rm -rf*"

alwaysAsk:
  - tool: HTTPRequest
```

## profiles

```yaml
profiles:
  <name>:
    type: openai | bedrock | vertex | ollama    # default: openai
    base_url: https://...                        # openai / ollama only
    model: <model-id>
    api_key: env:MY_KEY                          # openai / ollama
    region: us-west-2                            # bedrock / vertex
    project_id: my-gcp-project                  # vertex only
```

## mcpServers

```yaml
mcpServers:
  <name>:
    command: npx                          # stdio only
    args: ["-y", "@mcp/server", "/tmp"]  # stdio only
    type: stdio | sse | streamable | websocket
    endpoint: https://...                 # sse / streamable / websocket
    headers:
      X-Key: env:MY_API_KEY
    subprotocols: ["mcp"]                # websocket only
    max_retries: 3                        # streamable only
    elicitation_mode: disabled            # suppress elicitation capability
    oauth:
      authorization_url: https://...
      token_url: https://...
      client_id: my-app
      scopes: [mcp.read]
      redirect_port: 8765
```

## hooks

```yaml
hooks:
  PreToolUse:
    - matcher: "Bash"           # Go regexp; omit to match all tools
      command: /path/to/hook.sh
      timeout: 30s
  PostToolUse:
    - matcher: "Write|Edit"
      command: /path/to/hook.sh
  UserPromptSubmit:
    - command: /path/to/hook.sh
  Stop:
    - command: /path/to/hook.sh
  SubagentStop:
    - command: /path/to/hook.sh
  PreCompact:
    - command: /path/to/hook.sh
```

## pricing

```yaml
pricing:
  claude-sonnet-4-6:
    input_per_m: 3.0
    output_per_m: 15.0
  "anthropic.claude-sonnet-4-6-v1:0":    # Bedrock / quoted for colon
    input_per_m: 3.0
    output_per_m: 15.0
```

Keys are exact model names or `filepath.Match` glob patterns. Rates are USD per million tokens.

## theme

```yaml
theme:
  name: dark   # dark | light | custom
  # custom palette (only when name: custom):
  user_prompt:  "#ff79c6"
  assistant:    "#f8f8f2"
  tool_header:  "#50fa7b"
  tool_body:    "#8be9fd"
  error:        "#ff5555"
  status_line:  "#6272a4"
  border:       "#44475a"
  modal_border: "#bd93f9"
```

## auth (OAuth /login)

```yaml
auth:
  authorization_url: https://your-idp/oauth2/authorize
  token_url:         https://your-idp/oauth2/token
  client_id:         your-client-id
  # client_secret:   optional for confidential clients
  scopes: [openid, profile]
  redirect_port: 8765
```

## telemetry

```yaml
telemetry:
  enabled: false
  endpoint: https://your-collector.example.com/events
```

## webSearch

```yaml
webSearch:
  backend: brave   # brave | google | bing | tavily | disabled
  apiKey: env:BRAVE_API_KEY
  # endpoint: optional override for default API URL
```

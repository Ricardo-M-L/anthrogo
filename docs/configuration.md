# Configuration

Settings live in `$ANTHROGO_HOME/settings.yaml` (default: `~/.anthrogo/settings.yaml`). Override the home directory via the `ANTHROGO_HOME` environment variable.

Run `anthrogo init-config` to generate a settings file interactively, or create it manually.

## Layered config

anthrogo applies two layers in order:

1. **Home config** — `~/.anthrogo/settings.yaml` — applies to all sessions
2. **Project overlay** — `<cwd>/.anthrogo/settings.yaml` — merges on top of home config when anthrogo is started in that directory (deep merge for maps; lists replace)

## Full settings.yaml schema

```yaml
# ── Provider ──────────────────────────────────────────
mode: default       # default | acceptEdits | bypassPermissions | plan
model: claude-sonnet-4-6
provider: anthropic       # anthropic | openai | bedrock | vertex | ollama | <custom>
apiKey: ""                # or use ANTHROPIC_API_KEY env var

# ── Permission rules ──────────────────────────────────
alwaysAllow:
  - tool: Read
  - tool: Glob
  - tool: Grep
  - tool: Bash
    match: "git status*"

alwaysDeny:
  - tool: Bash
    match: "rm -rf*"

alwaysAsk:
  - tool: HTTPRequest

# ── Provider profiles ─────────────────────────────────
profiles:
  deepseek:
    type: openai
    base_url: https://api.deepseek.com
    model: deepseek-chat
    api_key: env:DEEPSEEK_API_KEY
  kimi:
    type: openai
    base_url: https://api.moonshot.cn/v1
    model: kimi-k2-0905-preview
    api_key: env:KIMI_API_KEY
  ollama-llama3:
    type: ollama
    model: llama3
  bedrock-sonnet:
    type: bedrock
    model: anthropic.claude-sonnet-4-6-v1:0
    region: us-west-2
  vertex-sonnet:
    type: vertex
    model: claude-sonnet-4-6@20260101
    region: us-east5
    project_id: my-gcp-project

providers_failover: []    # list of profile names; try in order on pre-commit error

# ── Web search ────────────────────────────────────────
webSearch:
  backend: brave           # brave | google | bing | tavily | disabled
  apiKey: env:BRAVE_API_KEY

# ── MCP servers ──────────────────────────────────────
mcpServers:
  filesystem:
    command: npx
    args: ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  remote:
    type: streamable        # stdio | sse | streamable | websocket
    endpoint: https://example.com/mcp
    headers:
      X-Api-Key: env:MY_API_KEY
    oauth:
      authorization_url: https://example.com/oauth/authorize
      token_url: https://example.com/oauth/token
      client_id: my-app
      scopes: [mcp.read]
      redirect_port: 8765
    elicitation_mode: "disabled"   # suppress elicitation capability

# ── Hooks ─────────────────────────────────────────────
hooks:
  PreToolUse:
    - matcher: "Bash"
      command: ~/.anthrogo/hooks/audit.sh
      timeout: 30s
  PostToolUse:
    - matcher: "Write|Edit"
      command: ~/.anthrogo/hooks/gofmt.sh
  UserPromptSubmit:
    - command: ~/.anthrogo/hooks/inject-cwd.sh
  Stop:
    - command: ~/.anthrogo/hooks/notify.sh
  PreCompact:
    - command: ~/.anthrogo/hooks/pre-compact.sh

# ── Cost / budget ────────────────────────────────────
cost_limit_usd: 0.50      # 0 = disabled

pricing:
  claude-sonnet-4-6:
    input_per_m: 3.0
    output_per_m: 15.0
  "anthropic.claude-sonnet-4-6-v1:0":
    input_per_m: 3.0
    output_per_m: 15.0

# ── Compaction ───────────────────────────────────────
auto_compact_threshold: 150000   # 0 = disabled; checked after each turn

# ── Theme ────────────────────────────────────────────
theme:
  name: dark       # dark | light | custom
  # Custom palette (only when name: custom):
  user_prompt:  "#ff79c6"
  assistant:    "#f8f8f2"
  tool_header:  "#50fa7b"
  tool_body:    "#8be9fd"
  error:        "#ff5555"
  status_line:  "#6272a4"
  border:       "#44475a"
  modal_border: "#bd93f9"

# ── OAuth / /login ───────────────────────────────────
auth:
  authorization_url: https://your-idp.example.com/oauth2/authorize
  token_url:         https://your-idp.example.com/oauth2/token
  client_id:         your-client-id
  scopes: [openid, profile]
  redirect_port: 8765

# ── Telemetry ────────────────────────────────────────
telemetry:
  enabled: false
  endpoint: https://your-collector.example.com/events
```

## Environment variable resolution

Any string value in `settings.yaml` starting with `env:` is resolved from the process environment at startup:

```yaml
api_key: env:DEEPSEEK_API_KEY    # reads $DEEPSEEK_API_KEY
```

This keeps secrets out of YAML files. `/mcp status <name>` redacts header values whose keys contain `authorization`, `auth`, `token`, `key`, `secret`, `password`, or `bearer` (case-insensitive).

## Key environment variables

| Variable | Purpose |
|----------|---------|
| `ANTHROPIC_API_KEY` | Anthropic API key (fallback when no OAuth token) |
| `ANTHROGO_HOME` | Override `~/.anthrogo/` home directory |
| `ANTHROGO_RELEASE_REPO` | Override `owner/repo` for `/version` update check |
| `GITHUB_TOKEN` | Raise GitHub API rate limit for `/version` check |
| `KAIROS_AUTH_TOKEN` | Bearer token for KAIROS worker/client |
| `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION` | Bedrock credentials |
| `GOOGLE_APPLICATION_CREDENTIALS` | Vertex AI service-account key path |

## CLAUDE.md auto-loading

At startup anthrogo walks from `cwd` up to `$HOME` and merges every `CLAUDE.md` it finds. The merged content is appended to the system prompt. Files closer to the project root take higher positional prominence.

## System prompt overlays

Two overlay layers are appended to the system prompt in order:

1. `~/.anthrogo/system_overlay.md` — home overlay, applies to every session
2. `<cwd>/.anthrogo/system_overlay.md` — project overlay, higher positional prominence

Manage with `/system`:

| Command | Effect |
|---------|--------|
| `/system show` | Print active system prompt + both overlay files |
| `/system edit` | Open `$EDITOR` on home overlay |
| `/system edit project` | Open `$EDITOR` on project overlay |
| `/system reset` | Remove home overlay (effective next session) |
| `/system reset project` | Remove project overlay (effective next session) |

## Theme switching

```
/theme list         # show available themes
/theme show         # print current theme name
/theme set light    # switch to light theme immediately
```

Built-in themes: `dark` (default), `light`. Use `name: custom` with hex palette for full control.

## Mode flags

| Mode | Write tools | MCP tools | Task/subagent |
|------|-------------|-----------|---------------|
| `default` | ask (per rules) | allowed | allowed |
| `acceptEdits` | auto-allow | allowed | allowed |
| `plan` | denied | denied | denied |
| `bypassPermissions` | all allowed | allowed | allowed |

Switch mode at runtime with `/mode <name>` or `--permission-mode <name>` on the CLI.

# Migration guide: v0.3.0 → v0.13.x

anthrogo's config has accreted features across 80+ releases. This guide
helps you upgrade `settings.yaml` and understand behaviour changes milestone
by milestone. Read only the sections that apply to your current version.

---

## v0.3 → v0.4 (Hooks + Compact + Skills + Plugins)

### Hooks stanza (M4.1)

A new top-level `hooks:` key lets you run shell commands at lifecycle events.

**Before (v0.3):** no hooks support.

**After (v0.4):**

```yaml
hooks:
  # Run before every user message is sent to the model.
  preMessage: []

  # Run after every assistant message is received.
  postMessage:
    - command: ./my-hook.sh
      args: []
      env: {}

  # Run once when the session starts.
  sessionStart: []

  # Run once when the session ends (/exit or Ctrl-C).
  sessionEnd: []
```

Each hook entry supports `command`, `args`, and `env` fields.

### Pricing stanza (M7.4 — backfill note)

The `pricing:` stanza was added later (M7.4) but depends on `provider:` which
was introduced in M4.x. If you see `/cost` returning $0, add:

```yaml
pricing:
  inputPerMToken: 3.00    # USD per million input tokens
  outputPerMToken: 15.00  # USD per million output tokens
```

Or rely on built-in defaults which cover all Claude models by name.

### /compact is no longer a placeholder

In v0.3 `/compact` printed "not implemented". From v0.4 it summarises the
conversation using the current model and replaces the history with the summary.
No config change required; behaviour is now active.

### Skills directory (M4.3)

```yaml
# New optional key. Omit to disable skills.
skillsDir: ~/.anthrogo/skills   # or a relative path
```

Each subdirectory containing a `SKILL.md` becomes a skill available via
trigger phrases or the Task tool.

### Plugins directory (M4.4)

```yaml
# New optional key. Omit to disable plugins.
pluginsDir: ~/.anthrogo/plugins
```

---

## v0.4 → v0.5 (Subagents + MCP debt)

### subagent_type on the Task tool

The Task tool now accepts a `subagent_type` field:

| Value | Behaviour |
|-------|-----------|
| `local` (default) | Run subagent in the same process |
| `remote:<name>` | Dispatch to a KAIROS worker named `<name>` |

No config change required for local subagents. For remote subagents see
[v0.5 → v0.6](#v05--v06-kairos--websocket--oauth).

### /mcp reload

`/mcp reload` now re-registers tools with the model without restarting
anthrogo. Previously only a full restart would pick up MCP config changes.

### MCP server env vars

MCP server entries can now forward environment variables to the server process:

```yaml
mcpServers:
  my-server:
    command: my-mcp-server
    args: []
    env:
      MY_SECRET: env:MY_SECRET_ENV_VAR   # "env:" prefix reads from shell
```

---

## v0.5 → v0.6 (KAIROS + WebSocket + OAuth)

### KAIROS configuration

Two new top-level stanzas for KAIROS distributed subagents:

**Worker side:**

```yaml
kairos:
  role: worker
  listen: ":9001"
  maxConcurrent: 4
  authToken: env:KAIROS_AUTH_TOKEN
```

**Client side:**

```yaml
kairos:
  role: client
  remote:
    - name: my-worker
      address: "worker-host:9001"
      authToken: env:KAIROS_AUTH_TOKEN
      maxHops: 1
```

### WebSocket MCP servers

`mcpServers.<name>.type: websocket` is now supported alongside the default
`stdio` type:

```yaml
mcpServers:
  remote-tools:
    type: websocket
    url: wss://my-mcp-server.example.com/mcp
    headers:
      Authorization: "Bearer env:MCP_TOKEN"
```

### OAuth for MCP (M6.3)

MCP servers that require OAuth 2.0 can use:

```yaml
mcpServers:
  oauth-server:
    type: websocket
    url: wss://example.com/mcp
    auth:
      type: oauth2
      clientId: env:OAUTH_CLIENT_ID
      clientSecret: env:OAUTH_CLIENT_SECRET
      tokenUrl: https://example.com/oauth/token
```

---

## v0.6 → v0.7 (multi-provider + /compact auto + /cost)

### profiles map and provider selection (M7.1)

The config now supports multiple provider profiles. `provider:` selects which
profile to use by name.

**Before (v0.6):** single `apiKey:` at top level, implicit Anthropic.

**After (v0.7):**

```yaml
# Active provider — must match a key in profiles: (or "anthropic" built-in).
provider: anthropic

# Built-in shorthand still works — no profiles: block needed for Anthropic:
apiKey: env:ANTHROPIC_API_KEY

# Multi-provider example:
provider: openai-gpt4
profiles:
  openai-gpt4:
    type: openai
    model: gpt-4o
    apiKey: env:OPENAI_API_KEY
    baseUrl: https://api.openai.com/v1
  anthropic-haiku:
    type: anthropic
    model: claude-haiku-4-5
    apiKey: env:ANTHROPIC_API_KEY
```

### --provider CLI flag

```bash
anthrogo --provider openai-gpt4
```

Overrides the `provider:` key in settings.yaml for this session only.

### /compact auto-trigger (M7.2)

Auto-compact fires when context usage exceeds a threshold. Configurable:

```yaml
compact:
  autoThreshold: 0.85      # Trigger at 85% context usage (0.0 to 1.0).
  autoEnabled: true        # Set false to disable auto-compact.
  summaryMaxTokens: 2048   # Max tokens for the compaction summary.
```

### /cost command (M7.4)

`/cost` is now available. It uses the `pricing:` stanza or built-in per-model
defaults. Override if you're using a custom endpoint with different pricing:

```yaml
pricing:
  inputPerMToken: 3.00
  outputPerMToken: 15.00
```

---

## v0.7 → v0.8 (system overlay + theme + UX)

### System overlay file (M8.1)

Create `~/.anthrogo/system_overlay.md` (or a project-local `system_overlay.md`
in `ANTHROGO_HOME`) to inject persistent instructions into every session's
system prompt:

```markdown
# My overlay

Always respond in British English.
Never use bullet points — use numbered lists instead.
```

No config key needed — anthrogo reads the file automatically.

You can also set the path explicitly:

```yaml
systemOverlay: ./my-overlay.md
```

### Theme stanza (M8.2)

```yaml
theme:
  # "default", "dark", "light", or "solarized"
  colorScheme: dark

  # Highlight colour for the prompt and tool output.
  accentColor: "#7c3aed"   # purple

  # Width of the main content column (characters).
  contentWidth: 100
```

### Input history persistence (M8.3)

```yaml
history:
  file: ~/.anthrogo/history     # Path to the readline history file.
  maxEntries: 1000
```

---

## v0.8 → v0.9 (Bedrock + Vertex + KAIROS multi-hop)

### AWS Bedrock provider (M9.1)

```yaml
profiles:
  bedrock-sonnet:
    type: bedrock
    model: anthropic.claude-sonnet-4-6-20260101-v1:0
    region: us-west-2
    # Credentials: uses the standard AWS credential chain
    # (env vars, ~/.aws/credentials, instance role, etc.)
    # Optional explicit override:
    # accessKeyId: env:AWS_ACCESS_KEY_ID
    # secretAccessKey: env:AWS_SECRET_ACCESS_KEY
```

### Google Vertex AI provider (M9.2)

```yaml
profiles:
  vertex-sonnet:
    type: vertex
    model: claude-sonnet-4-6@20260101
    project: my-gcp-project-id
    location: us-east5
    # Credentials: uses Application Default Credentials (gcloud auth).
    # Optional service account key file:
    # credentialsFile: env:GOOGLE_APPLICATION_CREDENTIALS
```

### KAIROS multi-hop (M9.3)

Workers can now forward tasks to other workers. Increase `maxHops` on the
client to allow chained dispatch:

```yaml
kairos:
  role: client
  remote:
    - name: tier-1-worker
      address: "worker1:9001"
      authToken: env:KAIROS_AUTH_TOKEN
      maxHops: 2   # worker1 may itself dispatch one more hop
```

---

## v0.9 → v0.10 (search index + Bash sandbox + multi-backend search)

### Session search cache (M10.1)

```yaml
# Number of past session summaries to keep in the in-process search index.
# Increase for better /search recall at the cost of memory.
sessionSearchCacheSize: 200
```

### Bash tool sandbox (M10.2)

The Bash tool can now run commands in an isolated environment:

```yaml
tools:
  bash:
    # Enable a sandboxed shell (uses Docker or macOS Sandbox).
    sandbox: true

    # Allowed paths inside the sandbox (bind-mounted read-write).
    allowedPaths:
      - /tmp
      - ./workspace

    # Commands blocked even inside the sandbox.
    blockedCommands:
      - curl
      - wget
```

### Web search backends (M10.3)

`webSearch.backend` now accepts multiple values tried in order:

```yaml
webSearch:
  # Try Brave first, fall back to DuckDuckGo if quota exceeded.
  backend:
    - brave
    - duckduckgo
  braveApiKey: env:BRAVE_API_KEY
  maxResults: 5
```

---

## v0.10 → v0.11 (multi-pane TUI + plugin marketplace + speech I/O)

### TUI layout cycling (M11.1)

Press `F2` at runtime to cycle through TUI layouts:
`default` → `wide` → `split` → `minimal` → `default`

Set the initial layout in config:

```yaml
theme:
  layout: split    # "default", "wide", "split", "minimal"
```

### Plugin install from URL (M11.2)

```
/plugin install https://github.com/owner/my-anthrogo-plugin
/plugin install https://example.com/plugins/my-plugin.tar.gz
```

No config change required — plugins are installed to `pluginsDir`.

### Speech I/O (M11.3)

```yaml
speech:
  # Input: transcribe spoken audio to text.
  input:
    enabled: false
    provider: whisper       # "whisper" or "system"
    language: en

  # Output: read assistant messages aloud.
  output:
    enabled: false
    provider: system        # "system" (OS TTS) or "elevenlabs"
    voice: default
    # elevenlabsApiKey: env:ELEVENLABS_API_KEY
```

---

## v0.11 → v0.12 (doctor + init-config + HTTPRequest + SQLQuery + Ollama)

### anthrogo doctor (M12.1)

New subcommand that checks your installation and config:

```bash
anthrogo doctor
```

No config change. Outputs a checklist of warnings and errors.

### anthrogo init-config wizard (M12.2)

```bash
anthrogo init-config              # Interactive wizard, does not overwrite
anthrogo init-config --force      # Backs up existing config, starts fresh
```

The wizard creates a new `settings.yaml` in `ANTHROGO_HOME`.

### HTTPRequest tool (M12.3)

```yaml
tools:
  httpRequest:
    enabled: true
    # Domains the model is allowed to call. Omit for unrestricted.
    allowedDomains:
      - api.github.com
      - httpbin.org
    # Max response body size in bytes (default 1 MB).
    maxResponseBytes: 1048576
```

### SQLQuery tool (M12.4)

```yaml
tools:
  sqlQuery:
    enabled: true
    # Named database connections the model may query.
    connections:
      analytics:
        driver: postgres
        dsn: env:ANALYTICS_DB_DSN
      local:
        driver: sqlite3
        dsn: ./local.db
    # Block write statements (INSERT/UPDATE/DELETE/DROP/…).
    readOnly: true
```

### Ollama local provider (M12.5)

```yaml
profiles:
  ollama-llama3:
    type: ollama
    model: llama3
    # baseUrl defaults to http://localhost:11434
    baseUrl: http://localhost:11434
```

Ollama pricing defaults to $0/M (local inference). See
`docs/providers/ollama.md` for the full model list.

---

## v0.12 → v0.13 (docs restructure)

### README slimmed (M13.1)

The README.md was reduced from ~1 000 lines to ~280 lines. All feature
documentation moved to dedicated `docs/*.md` pages. If you had links to
README sections, update them to the corresponding `docs/` page.

Full reference: https://Ricardo-M-L.github.io/anthrogo/

### No YAML changes

v0.13 introduces no breaking config changes. Existing `settings.yaml` files
from v0.12 work without modification.

---

## YAML upgrade strategy

If you have an old `settings.yaml` and want to get to v0.13:

### Option A — incremental (recommended)

1. Run `anthrogo doctor` to get a checklist of missing or deprecated keys.
2. Apply one section of this guide at a time, testing after each change.
3. Reload with `/mcp reload` (for MCP changes) or restart for everything else.

### Option B — fresh start

```bash
# Back up your current config.
cp "$ANTHROGO_HOME/settings.yaml" "$ANTHROGO_HOME/settings.yaml.bak"

# Run the wizard to generate a fresh config.
anthrogo init-config --force
```

The wizard asks which features you want and writes a complete, annotated
`settings.yaml`. You can then merge your customisations from the backup.

### Key compatibility notes

| Feature | Min version | Notes |
|---------|-------------|-------|
| `hooks:` | v0.4 | New in M4.1 |
| `skillsDir:` | v0.4 | New in M4.3 |
| `pluginsDir:` | v0.4 | New in M4.4 |
| `profiles:` / `provider:` | v0.7 | New in M7.1 |
| `compact.autoThreshold:` | v0.7 | New in M7.2 |
| `pricing:` | v0.7 | New in M7.4 |
| `systemOverlay:` | v0.8 | New in M8.1 |
| `theme:` | v0.8 | New in M8.2 |
| `kairos:` | v0.6 (basic) / v0.9 (multi-hop) | — |
| `Profile.Type: "bedrock"` | v0.9 | New in M9.1 |
| `Profile.Type: "vertex"` | v0.9 | New in M9.2 |
| `sessionSearchCacheSize:` | v0.10 | New in M10.1 |
| `tools.bash.sandbox:` | v0.10 | New in M10.2 |
| `webSearch.backend:` (multi) | v0.10 | New in M10.3 |
| `speech:` | v0.11 | New in M11.3 |
| `tools.httpRequest:` | v0.12 | New in M12.3 |
| `tools.sqlQuery:` | v0.12 | New in M12.4 |
| `Profile.Type: "ollama"` | v0.12 | New in M12.5 |

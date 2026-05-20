# anthrogo

A Go port of Anthropic's Claude Code CLI, reconstructed from the
source-mapped `@anthropic-ai/claude-code@2.1.88` package.

> **Status**: M10.2 complete (v0.10.1-dev). Bash sandbox (lightweight). See `docs/superpowers/specs/` for design docs.

## Input history

Every prompt you submit is appended to `~/.anthrogo/input_history` (one entry per line, rolling cap of 1000, consecutive duplicates skipped). In the TUI, press **Up/Down** to scroll back and forward through history — your current draft is saved when you start scrolling and restored when you press Down past the newest entry.

Use the `/history` slash command to inspect or manage history:

| Command | Effect |
|---------|--------|
| `/history` or `/history list` | Show the 20 most recent prompts |
| `/history list N` | Show the last N prompts |
| `/history search <keyword>` | Case-insensitive substring search across all history |
| `/history clear` | Delete the history file |

## Why

The upstream CLI is TypeScript + Bun + React/Ink. `anthrogo` re-expresses the
same architecture in Go, preserving the shapes of `Tool`, `QueryEngine`,
`PermissionContext`, `ToolUseContext`, MCP client, hooks, skills, plugins —
while replacing the Ink UI with a Bubble Tea front-end.

This project is **not** a 1:1 line transliteration: feature flags become
Go build tags, Zod becomes JSON schema, React components become Bubble Tea
update/view loops.

## Roadmap

| Milestone | Scope                                                                      | Status   |
|-----------|----------------------------------------------------------------------------|----------|
| M1        | TUI REPL + 7 core tools + Anthropic SDK + permission gate + CLAUDE.md      | shipped  |
| M2        | More tools, session persistence, plan mode, slash-command palette          | shipped  |
| M3        | MCP client                                                                 | shipped  |
| M4        | Hooks, skills, plugins                                                     | shipped  |
| M5.1      | Subagents (Task tool + sub-engine, depth limit, SubagentStop hook)         | shipped  |
| M5.2      | MCP resources + minimal elicitations (decline handler)                    | shipped  |
| M5.3      | Concurrent subagents, isolated perms, user-defined YAML types              | shipped  |
| M6.3      | Real TUI form elicitation handler (JSON-blob form modal)                   | shipped  |
| M8.9      | Multi-field form elicitation UI (per-field input, Tab nav, type coercion)  | shipped  |
| M8.11     | Diff / Format / Git built-in tools                                         | shipped  |
| M6.5      | OAuth 2.1 PKCE client flow for MCP HTTP transports                         | shipped  |
| M6.6      | KAIROS coordinator (minimal cross-process subagent dispatch)               | shipped  |
| M9.4      | Subagent real-time stream to parent TUI (OnTextDelta callback + buffering)  | shipped  |
| M9.5      | LSP-style code intel tools: SymbolSearch + References                       | shipped  |
| M9.7      | Form UI completion: cursor nav, enum cycler, Ctrl+J newline, schema defaults | shipped  |
| M9.8      | `Diff.range` commit-range, `Format.paths` batch, per-nest subagent JSONL     | shipped  |
| M9.9      | Model + path + visibility polish, KAIROS hook resolver, nested prefix chain  | shipped  |
| M9.10     | Persistent input history (Up/Down nav), `/history` slash command             | shipped  |
| M6        | Bedrock/Vertex + OpenAI-compat / DeepSeek / Kimi / MiniMax / GLM           | planned  |

## Repository layout

```
anthrogo/
├── cmd/anthrogo/        # CLI entry
├── internal/
│   ├── tui/             # Bubble Tea models
│   ├── headless/        # -p stdout path
│   ├── config/          # settings + paths
│   ├── system/          # system prompt + CLAUDE.md walker
│   ├── session/         # conversation state
│   └── mcp/             # MCP stdio client (Manager + Server + LogSink)
├── pkg/
│   ├── message/         # ContentBlock types
│   ├── provider/        # Provider interface + Anthropic impl
│   ├── tool/            # Tool framework + 7 built-ins
│   ├── permissions/     # PermissionContext + gate
│   └── query/           # QueryEngine — owns turn loop
└── docs/superpowers/specs/  # design docs
```

## Building

```bash
make build              # produces ./bin/anthrogo
make test               # go test ./...
./bin/anthrogo --version
```

## Running

Set `ANTHROPIC_API_KEY` in your environment, then:

```bash
./bin/anthrogo                          # interactive REPL (Bubble Tea)
./bin/anthrogo -p "explain main.go"     # headless: prints assistant text, exits
./bin/anthrogo --permission-mode acceptEdits -p "fix the typo in README"
./bin/anthrogo --model claude-haiku-4-5-20251001
./bin/anthrogo --cwd /path/to/project
```

## Configuration

Settings live in `$ANTHROGO_HOME/settings.yaml` (default: `~/.anthrogo/settings.yaml`). Example:

```yaml
mode: default
model: claude-sonnet-4-6
alwaysAllow:
  - tool: Read
  - tool: Glob
  - tool: Grep
  - tool: Bash
    match: "git status*"
  - tool: Bash
    match: "git diff*"
alwaysDeny:
  - tool: Bash
    match: "rm -rf*"
```

`CLAUDE.md` is auto-loaded by walking from cwd up to `$HOME`; merged contents are appended to the system prompt.

### Theme

anthrogo ships two built-in themes: `dark` (default) and `light`. Select one in `settings.yaml`:

```yaml
theme:
  name: light   # "dark" | "light" | "custom"
```

For a fully custom palette use `name: custom` with per-field hex colour values:

```yaml
theme:
  name: custom
  user_prompt:  "#ff79c6"
  assistant:    "#f8f8f2"
  tool_header:  "#50fa7b"
  tool_body:    "#8be9fd"
  error:        "#ff5555"
  status_line:  "#6272a4"
  border:       "#44475a"
  modal_border: "#bd93f9"
```

Switch theme at runtime with the `/theme` builtin:

| Command | Effect |
|---------|--------|
| `/theme` or `/theme list` | Show available themes |
| `/theme show` | Print the currently active theme name |
| `/theme set light` | Switch to the light theme immediately |

### Custom system prompt overlay

anthrogo supports two overlay layers, both loaded at startup and appended to the system prompt in order:

1. **Home overlay** — `~/.anthrogo/system_overlay.md` — applies to every session.
2. **Project overlay** — `<cwd>/.anthrogo/system_overlay.md` — applies only when anthrogo is started in that directory. Appended after the home overlay, so its instructions have higher positional prominence for the model.

Manage them with the `/system` builtin:

| Command | Effect |
|---------|--------|
| `/system show` | Print the active system prompt, then both overlay files with their paths |
| `/system edit` or `/system edit home` | Open `$EDITOR` on the home overlay inline (TUI suspends, editor runs, TUI resumes) |
| `/system edit project` | Open `$EDITOR` on the project overlay inline |
| `/system reset` or `/system reset home` | Remove the home overlay (effective next session) |
| `/system reset project` | Remove the project overlay (effective next session) |

Each overlay file is created with a seed comment the first time the corresponding `/system edit` is run. The editor opens inline via bubbletea's `ExecProcess`; the TUI suspends, you edit, save and quit the editor, and the TUI resumes with a status message showing how many bytes were saved. Changes apply on the **next** conversation turn (the engine's system prompt for the current session is frozen at startup).

## Cost tracking

anthrogo ships built-in defaults for major models (Claude opus-4-7/sonnet-4-6/haiku-4-5, OpenAI gpt-5*/gpt-4o/gpt-4o-mini/gpt-4-turbo/o1*/o3*, DeepSeek chat/reasoner, Kimi k2*, MiniMax M2/abab*, GLM 4*/zero-*), so `/cost` works out of the box without any configuration.

Add a `pricing:` stanza in `~/.anthrogo/settings.yaml` to override built-ins with negotiated rates or to add unlisted models:

```yaml
pricing:
  claude-sonnet-4-6:
    input_per_m: 3.0
    output_per_m: 15.0
  claude-haiku-4-5-*:
    input_per_m: 1.0
    output_per_m: 5.0
  deepseek-chat:
    input_per_m: 0.27
    output_per_m: 1.1
```

Keys are exact model names or glob patterns (`filepath.Match` syntax; `*` matches within a path segment). Rates are USD per one million tokens. User-supplied keys always win over built-in defaults.

Once configured, the TUI status line shows the running cost (`$0.0234`), and the `/cost` builtin prints a full summary:

```
Session usage: 12345 input + 1234 output = 13579 total tokens
Estimated cost: $0.0555 USD
```

To hard-cap spending, add `cost_limit_usd` to `settings.yaml` or pass `--cost-limit <USD>` on the command line. Once the cumulative estimated session cost reaches the limit, all tool calls are denied with a message showing the current cost and the limit. Set `--cost-limit 0` (or omit the field) to disable.

```yaml
cost_limit_usd: 0.50   # deny tools after ~$0.50 of estimated spend
```

Built-in rates are sourced from published pricing as of 2026-05; they will drift until the next anthrogo release updates them.

To zero the cumulative cost counter (e.g. after `/compact` to start fresh against the budget cap), run:

```
/cost reset
```

You can also reset automatically when compacting by passing `--reset-budget`:

```
/compact --reset-budget
```

This resets the in-memory usage counter so the post-compact session starts fresh. The budget cap remains armed; usage will accumulate again from zero.

## ContainerExec tool

The `ContainerExec` built-in tool runs a command inside a docker or podman container, providing real OS-level isolation. Auto-detects `docker` on PATH; falls back to `podman`.

Default network is `none` (no internet access). Containers are removed after exit (`--rm`).

Sample tool call (as the model would receive it):

```yaml
tool: ContainerExec
input:
  image: alpine
  command: "echo hello && uname -a"
  network: none          # default; omit to keep no-internet
  mounts:
    - /host/data:/data:ro   # read-only bind mount
    - /host/out:/out:rw     # writable bind mount
  env:
    MY_VAR: hello
  timeout_ms: 30000
```

Requirements: `docker` or `podman` must be on PATH. Image must be available locally or pullable (ContainerExec does not pre-pull — if the image is absent docker will pull it on first call).

## MCP servers

anthrogo can spawn MCP (Model Context Protocol) servers at startup and expose their tools to the model. Add to `~/.anthrogo/settings.yaml`:

```yaml
mcpServers:
  filesystem:
    command: npx
    args: ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  remote-fetch:
    type: streamable
    endpoint: https://example.com/mcp
    max_retries: 3
    headers:
      X-Api-Key: my-secret-key
  legacy-sse:
    type: sse
    endpoint: https://legacy.example.com/mcp
    headers:
      Authorization: "Bearer static-token"
  ws-server:
    type: websocket
    endpoint: wss://example.com/mcp
    subprotocols: ["mcp"]
    headers:
      X-Tenant-Id: "acme"
```

`type` defaults to `stdio`. Other values: `sse` (2024-11-05 SSE), `streamable` (newer streamable HTTP), `websocket` (custom WS implementation; ws:// or wss://).

`headers` injects arbitrary HTTP headers on every outgoing request for `sse`, `streamable`, and `websocket` transports. When `oauth:` is also set, headers are applied first and the OAuth `Authorization: Bearer` header is layered on top.

Header values support `env:VARNAME` expansion (resolved once at startup): `X-Api-Key: "env:MY_API_KEY"` reads `$MY_API_KEY` from the environment, keeping secrets out of the YAML file. `/mcp status <name>` prints headers with sensitive values redacted — any key containing `authorization`, `auth`, `token`, `key`, `secret`, `password`, or `bearer` (case-insensitive) shows as `<redacted>`.

`subprotocols` (websocket only) advertises the listed subprotocols during the WebSocket handshake (`Sec-WebSocket-Protocol`). The server's chosen subprotocol is echoed back in the response header.

Tools surface as `mcp__<server>__<tool>` (names exceeding 64 chars get a sha-8 suffix). Inspect status with `/mcp`, view one server's last error with `/mcp status <name>`, restart all servers with `/mcp reload` (removes and re-registers all `mcp__*` tools; the model's system prompt is still built at startup, so restart anthrogo to refresh model awareness of newly-added tools). Server log notifications render dim-styled in the TUI; in headless they go to stderr.

**Plan mode blocks all MCP tool calls** (`mcp__*` tools are treated as write tools). Switch to default mode (`/mode default`) to invoke MCP tools.

> anthrogo registers list-changed handlers for tools and resources. When a server pushes `notifications/tools/list_changed`, anthrogo refreshes its per-server tool cache and logs the event. The model-facing tool registry is NOT auto-rebuilt (would race with in-flight turns); run `/mcp reload` to surface new tools to the next system prompt.

### MCP resources

anthrogo lists resources advertised by Ready servers in the system prompt at startup and provides a built-in `MCPResource` tool the model can use to list or read them:

- **List resources on a server:** `{server: "filesystem"}` — returns a JSON array of `{uri, name, description, mime_type, size}`.
- **Read a resource:** `{server: "filesystem", uri: "file:///tmp/notes.md"}` — returns the resource text (or a blob summary for binary content).

The `MCPResource` tool has a default `alwaysAllow` rule at the CLI level (read-only; deny rules and `PreToolUse` hooks still take precedence).

### Elicitations

When an MCP server sends an `elicitation/create` request, TUI users get a form modal. The form behaviour depends on the schema:

- **Multi-field mode (M9.7):** If the schema is a flat `object` whose properties are all primitive types (`string`, `number`, `integer`, `boolean`) or `enum` string arrays, each property is rendered as its own input row. Tab/Shift-Tab moves focus; Enter submits; Esc cancels. Per-field cursor: Left/Right/Home/End navigate within the text; Backspace deletes before cursor; Delete removes at cursor; insertions go at cursor position. Ctrl+J inserts a literal newline into `string` fields. `enum` properties render as a horizontal cycler (`[selected] other1 other2`); Left/Right/Up/Down cycle the selection. Schema `default` values pre-fill buffers (or enum index) when the modal opens. Type coercion on submit: booleans accept `yes/no/y/n/true/false/1/0`; integers and numbers are parsed and declined with a reason on invalid input; required-empty fields decline; optional empty fields are omitted.
- **Single-textarea fallback (M6.3):** Schemas with nested objects, arrays, or multi-select enums fall back to the original single-textarea JSON-blob form. Type a JSON object that matches the schema, then press Enter to submit.

Headless mode (`-p`) always declines. To opt out entirely (suppress the capability advertisement), set `elicitation_mode: "disabled"` on the server config:

```yaml
mcpServers:
  filesystem:
    command: npx
    args: ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
    elicitation_mode: "disabled"   # don't advertise elicitation capability
```

### OAuth 2.1 (PKCE)

For HTTP/WebSocket MCP servers that require authentication, anthrogo supports the OAuth 2.1 authorization-code + PKCE flow:

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
      redirect_port: 8765
```

When `oauth:` is set on an `sse`, `streamable`, or `websocket` server:
1. anthrogo checks `~/.anthrogo/oauth/<server-name>.json` for a cached token.
2. If the token is valid (not expired within a 30s margin), it is reused.
3. If the token is expired but a `refresh_token` is present, a refresh-token grant is attempted.
4. Otherwise anthrogo opens your browser to the `authorization_url`, starts a local HTTP listener on `redirect_port` (default 8765) to catch the callback, exchanges the code + PKCE verifier for tokens, and caches the result.
5. The access token is injected as `Authorization: Bearer <token>` on every outgoing request.

`client_secret` is optional — public clients (PKCE-only) can omit it.

## KAIROS — cross-process subagent

anthrogo can route specific subagent types to another anthrogo instance running as a worker:

```bash
# Worker process:
KAIROS_AUTH_TOKEN=secret123 anthrogo --kairos-serve :9001
```

```yaml
# Client subagent yaml: ~/.anthrogo/subagents/heavy-research.yaml
name: heavy-research
description: Use for long research that benefits from the worker's tools.
remote:
  endpoint: http://worker.example.com:9001
  auth_token: env:KAIROS_AUTH_TOKEN
```

The client sends `POST /kairos/run` with `{subagent_type, prompt}`; the worker spawns a local subagent, streams `event: text` deltas, ends with `event: done`. Bearer auth via `Authorization: Bearer <token>`. M6.6 limits to one hop (the worker excludes Remote types from its own registry).

**M8.13 — exec-tools-locally mode.** Add `exec_tools_locally: true` to the subagent YAML to make the worker forward tool calls back to the client instead of running them on the worker. The client's tool registry and permission gate apply.

```yaml
remote:
  endpoint: http://worker.example.com:9001
  auth_token: env:KAIROS_AUTH_TOKEN
  exec_tools_locally: true   # tool calls run on the client, not the worker
```

Protocol: worker emits `event: run_id` (UUID), then `event: tool_use_request` per blocking tool call; client POSTs the result to `POST /kairos/run/<rid>/tool-result` before the worker resumes.

**M9.3 — multi-hop + remote hook/perm context.** Workers now support multi-hop chains (depth capped at `MaxHops = 2`). Remote-typed subagents in the worker's registry are registered only when the incoming `HopDepth` < 2; otherwise they are excluded, preventing unbounded recursion. The client automatically attaches a `RemoteContext` to each outgoing `/kairos/run` request that contains:

- `hop_depth` — incremented by each forwarding worker.
- `hooks` — the client's `hooks.Config` snapshot (PreToolUse, PostToolUse, etc.). The worker builds a `hooks.Manager` from it and applies it to the subagent run. Note: hook command paths are sent verbatim; use absolute paths for hooks meant to run on the worker.
- `permissions` — a `PermSnapshot` with the client's `Mode`, `AlwaysAllow`, `AlwaysDeny`, and `AlwaysAsk` rules. The worker substitutes its own permission context with this snapshot. `HookDecide` (a Go func) is intentionally not serialised; it is rebuilt from the forwarded hooks config.

## Vision / images

anthrogo supports sending images to multimodal models using the `@image:<path>` syntax anywhere in your prompt:

```
@image:./screenshot.png what's wrong with this UI?
look at @image:/tmp/diagram.png and explain the flow
describe @image:~/photo.jpg and @image:~/chart.png
```

Each `@image:<path>` token is replaced with a base64-encoded image block. Supported MIME types: `image/png`, `image/jpeg`, `image/gif`, `image/webp`. The MIME type is detected automatically from the file bytes.

- **Anthropic provider**: image blocks are passed using the native `image` content block (supported since M1).
- **OpenAI-compatible provider**: image blocks are converted to `{type: "image_url", image_url: {url: "data:<mime>;base64,..."}}` multimodal content arrays. Text-only messages continue to use the string-content path for backward compatibility.

Limitations: only local file paths are supported (no URLs or data URIs in the `@image:` syntax). Tool-result messages remain string-only (OpenAI limitation).

## Provider profiles

anthrogo ships an Anthropic provider by default but can route to any OpenAI Chat Completions compatible endpoint (DeepSeek, Kimi, MiniMax, GLM, vllm, ollama-openai, etc.), AWS Bedrock, or Google Cloud Vertex AI via profiles:

```yaml
provider: deepseek           # active profile
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
  minimax:
    type: openai
    base_url: https://api.minimaxi.com/v1
    model: MiniMax-M2
    api_key: env:MINIMAX_API_KEY
  glm:
    type: openai
    base_url: https://open.bigmodel.cn/api/paas/v4
    model: glm-4.6
    api_key: env:GLM_API_KEY
  bedrock-sonnet:
    type: bedrock
    model: anthropic.claude-sonnet-4-6-v1:0
    region: us-west-2   # optional; falls back to AWS_REGION or ~/.aws/config
  vertex-sonnet:
    type: vertex
    model: claude-sonnet-4-6@20260101
    region: us-east5      # mandatory
    project_id: my-gcp-project  # mandatory
```

### AWS Bedrock

Profile `type: bedrock` uses the AWS default credential chain — environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`), `~/.aws/credentials`, EC2/ECS IAM roles, etc. No `api_key` is needed; `ANTHROPIC_API_KEY` is not used.

Bedrock model IDs follow the AWS naming convention (`anthropic.claude-*`), which differs from the direct Anthropic API names. Pricing table lookups may not match built-in entries; add explicit `pricing:` entries if needed:

```yaml
pricing:
  "anthropic.claude-sonnet-4-6-v1:0":
    input_per_m: 3.0
    output_per_m: 15.0
```

### Google Cloud Vertex AI

Profile `type: vertex` uses Google Application Default Credentials — `GOOGLE_APPLICATION_CREDENTIALS` env var pointing to a service-account key file, `gcloud auth application-default login` for local development, or workload identity on GKE/Cloud Run. No `api_key` is needed; `ANTHROPIC_API_KEY` is not used.

Both `region` and `project_id` are mandatory. Vertex model IDs follow the Model Garden convention (`claude-sonnet-4-6@20260101`), which differs from the direct Anthropic API names. Add explicit `pricing:` entries if needed:

```yaml
pricing:
  "claude-sonnet-4-6@20260101":
    input_per_m: 3.0
    output_per_m: 15.0
```

Switch profiles at runtime:

```bash
anthrogo --provider kimi
anthrogo --provider deepseek -p "summarize this repo"
```

### Multi-provider failover

`providers_failover` lists profiles to try in order if the active provider emits an error **before** any text, tool-use, or usage event has been streamed. Once a "committed" event has been forwarded to the client the error passes through unchanged (partial streams cannot be retried transparently).

```yaml
provider: anthropic
providers_failover: [deepseek, kimi]
# On EventError before text/tool/usage from anthropic, anthrogo retries with deepseek.
# If deepseek also fails (pre-commit), it falls back to kimi.
```

Known limitations: no backoff between attempts; no selective retry by HTTP status code; partial-stream retry requires buffering (deferred).

## Hooks

anthrogo runs user-defined shell commands at 9 lifecycle events. Add to `~/.anthrogo/settings.yaml`:

```yaml
hooks:
  PreToolUse:
    - matcher: "Bash"
      command: ~/.anthrogo/hooks/audit.sh
      timeout: 30s
    - matcher: "Write|Edit|NotebookEdit"
      command: ~/.anthrogo/hooks/protect-secrets.sh
  PostToolUse:
    - matcher: "Write|Edit"
      command: ~/.anthrogo/hooks/gofmt.sh
  UserPromptSubmit:
    - command: ~/.anthrogo/hooks/inject-cwd.sh
  Stop:
    - command: ~/.anthrogo/hooks/notify-slack.sh
```

Each hook gets one JSON object on stdin describing the event. Exit code 2 blocks the action (PreToolUse → deny, UserPromptSubmit → abort prompt). Exit code 0 + JSON on stdout can `permissionDecision: "allow"|"deny"`, `modifiedInput: {...}` (PreToolUse only), or `additionalContext: "..."` (UserPromptSubmit / PostToolUse).

`matcher` is a Go regexp against the tool name (PreToolUse / PostToolUse only). Project-level `.anthrogo/hooks.yaml` appends to home-level `hooks:` block.

anthrogo doesn't auto-provision the `~/.anthrogo/hooks/` directory — create it and your hook scripts yourself, then `chmod +x` them.

Default timeouts: 30s for sync events, 5–10s for async. Async events (Stop / Notification / Session*) fire on a background goroutine.

Plan-mode hard-lock still overrides hook-allow for write tools.

`PreCompact` fires synchronously before `/compact` runs (M4.2).

## Skills

A Skill is a markdown file the model can invoke on demand. Layout:

```
~/.anthrogo/skills/<name>/
├── SKILL.md                # required, with frontmatter (name + description)
├── scripts/                # optional, model reads via the Read tool
└── references/             # optional
```

`SKILL.md`:

```markdown
---
name: git-flow
description: Use when starting a new feature branch off main.
---

# git-flow

When the user asks to start a new branch, do X then Y.
```

anthrogo lists every loaded skill in the system prompt (name + description). The model picks one, calls the `Skill` tool with `{"skill": "git-flow"}`, and gets the full markdown back. From there it follows the instructions, using Read / Bash / Write etc. as the skill dictates — all gated by the existing permission rules.

`/skills` lists them, `/skills show <name>` prints one, `/skills reload` re-scans.

Project-level `<cwd>/.anthrogo/skills/<name>/SKILL.md` overrides a same-named home skill.

**Trust:** the body of a SKILL.md becomes part of the prompt sent to the model when invoked. A malicious skill can instruct the model to leak data, exfiltrate files, or trigger side effects — though every action still flows through anthrogo's tool permission gate. Only install skills from sources you trust.

## Plugins

A Plugin is a directory bundling one or more of: slash commands, skills, hook configurations, MCP server configurations. Install by copying into `~/.anthrogo/plugins/` or via `/plugin install <local-path>`:

```
~/.anthrogo/plugins/git-tools/
├── plugin.yaml         # required manifest
├── skills/
│   └── git-flow/SKILL.md
└── hooks/audit.sh
```

`plugin.yaml`:

```yaml
name: git-tools
version: 0.1.0
description: Branch + PR helpers
commands:
  - name: /new-branch
    type: local-prompt
    body: |
      Start a new feature branch off main.
skills:
  - dir: skills/git-flow
hooks:
  PreToolUse:
    - matcher: Bash
      command: hooks/audit.sh
mcpServers:
  fs:
    command: npx
    args: ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
```

> Plugin-contributed MCP server keys are prefixed with `<plugin-name>:` at runtime to prevent collisions. So `git-tools`'s `fs:` server surfaces as tools like `mcp__git-tools:fs__read_file`. Use `/mcp` to inspect.

Project-level `<cwd>/.anthrogo/plugins/<name>/` overrides a same-named home plugin.

Manage with `/plugin` (list), `/plugin info <name>`, `/plugin reload`, `/plugin install <local-path>`, `/plugin remove <name>`. After install/remove anthrogo must be restarted for commands / skills / MCP-server / hook contributions to take effect at runtime.

**Trust:** Plugins execute shell commands (via hooks), spawn subprocesses (via MCP), and inject text into the model's prompt (via skills + commands). **Installing a plugin = trusting its author.** Every action still flows through anthrogo's existing permission gate, but the model's reasoning is fully influenceable by anything the plugin injects.

## Subagents

The model can spawn isolated sub-engines via the `Task` tool to perform self-contained multi-step tasks. Unlike skill invocations (which just return markdown), a subagent runs its own tool-use loop and returns its final answer as a tool result.

```
Task({
  "description": "find all TODO comments",
  "prompt":       "Search the codebase for TODO comments. Return a list.",
  "subagent_type": "general-purpose"
})
```

The subagent has no memory of the parent conversation — brief it fully in `prompt`. It inherits the parent's tools (unless the subagent type restricts them via `ToolAllowlist`), permission gate, and hook manager.

**Concurrent dispatch (M5.3):** when the model emits multiple `Task` tool_use blocks in a single turn, the engine runs them concurrently. Tool_result order is preserved. Log/stderr output from concurrent subagents may interleave.

**Permission isolation (M5.3):** each subagent runs with a cloned `permissions.Context`. Mode toggles (e.g. the model entering plan mode inside a subagent) do not leak back to the parent.

**Recursion limit:** nested subagents are allowed up to depth 3 by default (`MaxSubagentDepth`). Calls beyond the limit return an error to the model.

**Plan mode:** `Task` is treated as a write tool, so plan mode blocks it. Switch to default mode (`/mode default`) to invoke subagents.

**SubagentStop hook:** fires after every subagent completes (success or error). Wire it in `hooks.yaml` under `SubagentStop:`.

**Real-time streaming to TUI (M9.4):** subagent text deltas are forwarded to the parent TUI in real time, prefixed with `[Task: <description>] `. Deltas are buffered until newline boundaries to avoid scroll spam; the remaining buffer is flushed when the subagent finishes. Remote (KAIROS) subagents stream via `event: text` SSE messages, invoking the same callback path.

**Independent JSONL per subagent (M6.2):** each subagent run writes its own JSONL alongside the parent session for later inspection. Files land at `~/.anthrogo/projects/<cwd-hash>/<session-id>/subagents/<subagent-id>.jsonl`. A `subagent_start` record in the parent JSONL provides the ID for cross-referencing.

### Custom subagent types

Drop YAML files into `~/.anthrogo/subagents/` (home, all projects) or `<cwd>/.anthrogo/subagents/` (project-local; overrides home) to define your own types:

```yaml
# ~/.anthrogo/subagents/code-reviewer.yaml
name: code-reviewer
description: Use when reviewing a PR or code change for correctness and style.
system_prompt_suffix: |
  You are a code reviewer. Be specific. Cite file:line. Suggest concrete fixes.
tool_allowlist:
  - Read
  - Grep
  - Glob
  - Bash
```

Rules:
- `name` must be lowercase alphanumeric + hyphens (`^[a-z][a-z0-9-]{0,63}$`) and match the filename stem.
- `description` is required (shown to the model in the Task tool schema).
- `system_prompt_suffix` is optional extra instruction appended to the system prompt for this subagent.
- `tool_allowlist` is optional. Empty = inherit parent's full tool registry.
- The name `general-purpose` is reserved and cannot be overridden.

Use `/subagents` to list loaded types, `/subagents show <name>` to inspect, `/subagents reload` to hot-reload without restarting (note: the system prompt is built at startup, so newly added types won't be advertised to the model until restart).

## Compaction

For long sessions, `/compact` summarizes earlier turns to cut token cost:

```
/compact            # keeps the 10 most-recent messages, summarizes the rest
/compact --keep 20  # keeps 20 most-recent
```

Currently all earlier messages including MCP tool calls are summarized to prose; pair-preserving compaction is a future milestone. `PreCompact` hooks (configured under `hooks.PreCompact`) fire before each compact.

On completion, `/compact` reports actual token counts rather than byte sizes (e.g. `compacted 15 → 11 messages (~820 → ~210 tokens)`). Token counting uses a real BPE tokenizer for OpenAI-family models (tiktoken-go) and a char/4 approximation for Claude and other models. Image tokens are not counted client-side; the provider's EventUsage is authoritative for image cost.

Set `auto_compact_threshold: 150000` (or pass `--auto-compact 150000`) to have anthrogo automatically run `/compact` when cumulative token usage since the last compact exceeds the threshold. The threshold is checked at the end of every turn using the cumulative `usageSinceLastCompact` counter — not just the latest turn's usage — so sessions with many small turns are handled correctly. Set to 0 (default) to disable. After a successful compact the counter resets to zero. Manual `/compact` also resets the counter.

Use `/usage` at any time to inspect the current state:

```
/usage
Session totals: 1,240 input + 380 output = 1,620 tokens
Since last compact: 420 input + 130 output = 550 tokens
Auto-compact at: 150,000 tokens (keep recent: 10) — 149,450 tokens until trigger
```

The TUI status line shows `tok: <in>in/<out>out (since: <Z>) [⚙ <N>]` where `since` is the post-compact accumulation and `⚙ N` is the auto-compact threshold (omitted when disabled). Live 1Hz status refresh during turns (tokens, cost, budget).

## Session history

Use `/sessions` (or `/sessions list`) to inspect historical JSONLs for the current working directory, sorted newest-first:

```
/sessions
ID                                      Modified          Size
550e8400-e29b-41d4-a716-446655440000    2026-05-20 14:32  18423 B
3f2504e0-4f89-11d3-9a0c-0305e82c3301    2026-05-19 09:11  4201 B
```

Use `/sessions show <id-prefix>` for a quick metadata summary of a specific session (unambiguous prefix match).

Use `/sessions replay <id-prefix>` to render the matched session as a one-line-per-record timeline. Every record kind is covered: meta, user, asst, tool, result, compact, subagent, usage, turn-end, error. Text is truncated and newlines collapsed so the output stays readable in the TUI.

Use `/sessions search <keyword>` for case-insensitive substring search across all session JSONLs for the current cwd. Each match line shows `<session-id> [<kind>] <context>` (40 chars before + match + 40 chars after). Results are capped at 200 matches. Optional flags: `--regex` interprets the keyword as a Go regexp (invalid patterns return an error); `--recurse-subagents` also scans `<session-id>/subagents/*.jsonl` (matching lines are prefixed `<parent>/subagents/<sub>`); `--since YYYY-MM-DD` and `--until YYYY-MM-DD` filter records by timestamp.

Use `/sessions delete <id-prefix>` to remove a session. Without `--yes` it performs a **dry-run**: prints the JSONL path and size, the subagents subdirectory (if any) with file count and total bytes, and the exact command to re-run for real deletion. Add `--yes` to actually remove both the JSONL and the `<session-id>/subagents/` tree. This is irreversible — there is no undo.

Use `/sessions export <id-prefix> [-o file.md]` to render the session as a markdown document. Without `-o`, the markdown is printed to stdout. With `-o <file.md>`, it is written to the specified file and the command reports "exported <path> (<N> bytes)". `export` renders a human-readable document.

Use `/sessions stats` to aggregate metrics across all session JSONLs for the current cwd. The output shows session count, turn count, total input/output tokens, estimated USD cost (using the built-in default pricing table from M8.1), first-seen and latest timestamps, a per-model token and cost breakdown, and a per-day turn count table. Use `--since YYYY-MM-DD` and/or `--until YYYY-MM-DD` to narrow the aggregation to a date range.

Use `/sessions reindex` (alias `search-rebuild-index`) to clear the in-memory LRU parse cache (M8.12). The cache holds up to 64 parsed session files keyed by `(path, modtime)`. Unchanged files are served from cache on repeated searches without re-parsing. Modtime changes auto-invalidate; `reindex` forces a full rebuild on the next search.

**Persistence (M10.1):** The search cache is now two-level. L1 is the in-memory LRU (same as before). L2 is a SQLite database at `~/.anthrogo/search_index.db` (pure-Go, no cgo). Parsed records survive process restarts — on the next search the L2 hit is served directly without re-parsing the JSONL files. The cache degrades gracefully to L1-only if the DB can't be opened. To fully reset persistence, remove `~/.anthrogo/search_index.db` and restart.

### /audit (M10.4)

Use `/audit` (or `/audit list [N]`) to scan all session JSONLs for the current cwd and surface tool calls, errors, compact events, and subagent starts — newest first. N defaults to 50. Each row: `<ts>  [<short-session-id>]  <kind:tool>  <summary>`.

| Subcommand            | Description                                                  |
|-----------------------|--------------------------------------------------------------|
| `list [N]`            | Most-recent N audit events across all sessions (default 50). |
| `by-tool <name>`      | Filter to a specific tool name (e.g. `by-tool Bash`).        |
| `errors`              | Show only records where `IsError=true`.                      |
| `search <keyword>`    | Case-insensitive match against tool name + input summary.    |

Note: permission decisions (allow/deny/ask) are not currently recorded in the JSONL and are therefore not visible in `/audit` output. Only the resulting tool calls and error results are surfaced.

## Tools (M1)

| Tool        | Read-only | What it does                                                       |
|-------------|-----------|--------------------------------------------------------------------|
| Read        | yes       | Read a file with offset/limit, cat-n style line numbers            |
| Write       | no        | Write content to a file path, creating parent dirs                 |
| Edit        | no        | Replace `old_string` with `new_string`; unique match unless `replace_all` |
| Glob        | yes       | doublestar glob, results sorted newest-first by mtime              |
| Grep        | yes       | Go regexp recursive search with `output_mode` and glob filter      |
| Bash        | no        | Run a shell command with `timeout_ms` (default 120000); set `sandbox:true` for opt-in lightweight sandboxing |
| TodoWrite   | no        | Maintain a replace-on-write task list                              |
| Diff        | yes       | `git diff` wrapper; options: `path`, `cached`, `context`, `stat`  |
| Format      | no        | Format a file: gofmt / prettier / black / ruff / rustfmt          |
| Git         | yes       | Read-only git subcommands: status, log, branch, show, blame, remote |
| SymbolSearch | yes      | Find a symbol's definition by name; Go via `go/parser`, others via regex heuristics |
| References  | yes       | Find all word-boundary usages of a name across the tree           |
| WebSearch   | yes       | Search the web; dispatches to brave, google, bing, or tavily      |

### WebSearch backends (M10.3)

Configure `webSearch` in `~/.anthrogo/settings.yaml`. The `url` field is optional and overrides the default endpoint (useful for testing or self-hosted proxies).

```yaml
# Brave (default)
webSearch:
  backend: brave
  apiKey: "BSA..."           # Brave Search API key

# Google Custom Search
webSearch:
  backend: google
  apiKey: "AIza..."          # Google API key
  endpoint: "abc123:def456"  # CSE ID (cx parameter)

# Bing / Azure Cognitive Search
webSearch:
  backend: bing
  apiKey: "abc..."           # Ocp-Apim-Subscription-Key
  # endpoint: optional custom base URL (default: https://api.bing.microsoft.com/v7.0/search)

# Tavily
webSearch:
  backend: tavily
  apiKey: "tvly-..."         # Tavily API key
  # endpoint: optional custom base URL (default: https://api.tavily.com/search)

# Disable web search
webSearch:
  backend: disabled
```

All backends return a JSON array of `{title, url, description}` objects. Google caps results at 10; Bing at 50; Tavily at 20.

### Bash sandbox (M10.2)

Pass `"sandbox": true` in the Bash tool call to enable a lightweight, opt-in sandbox layer:

1. **Path validation** — heuristic denylist rejects commands containing `../`, `~/.ssh`, `~/.aws`, `~/.gnupg`, `~/.kube`, `/etc/passwd`, `/etc/shadow`, `/private/etc/`, `/var/log`, `/proc/`, `/sys/`. Violations surface as `is_error` results without executing.
2. **Restricted PATH** — `PATH` is replaced with `/usr/bin:/bin:/usr/sbin:/sbin`.
3. **Env stripping** — `HOME`, `SSH_*`, `AWS_*`, `GCP_*`, `AZURE_*`, `GOOGLE_*`, `GITHUB_*`, `GITLAB_*`, `ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, and other secret env vars are removed before the child process starts.

> **Limitations**: this is NOT a real sandbox — no chroot, no Linux namespaces, no container. A determined attacker can bypass substring checks via shell expansion or read sensitive paths via setuid binaries. For true isolation, run anthrogo inside a container.

## Permission model

For every tool call the engine consults `permissions.Decide`:

1. `bypassPermissions` mode → allow.
2. `alwaysDeny` rules → deny.
3. `acceptEdits` mode → allow Write/Edit/NotebookEdit by default.
4. `alwaysAllow` rules → allow.
5. `alwaysAsk` rules → ask (prompt the user).
6. Otherwise → ask (TUI shows a modal) / deny (headless when `ShouldAvoidPrompts`).

## Development

Run `make help` to see all available targets:

```
  help          Show this help.
  build         Build anthrogo for the current platform.
  test          Run all unit + integration tests.
  race          Run race detector on hot packages.
  sweep         3x uncached test sweep (catches flakes).
  vet           go vet all packages.
  lint          Run golangci-lint (install first via 'brew install golangci-lint' if missing).
  fmt           Format Go code.
  clean         Remove build artifacts.
  install       go install to $GOPATH/bin.
  release       Cross-compile release binaries for darwin/linux × amd64/arm64.
```

`make build` stamps the binary with the version from `internal/version/version.go` via `-ldflags`. `make release` produces version-named binaries under `dist/` for all four platforms (darwin/linux × amd64/arm64).

### CI

`.github/workflows/ci.yml` runs on every push and PR to `main`:

- **test** job — matrix over `ubuntu-latest` + `macos-latest`: `go build ./...`, `go vet ./...`, `go test ./...`, then a race-detector pass on the hot packages.
- **lint** job — `ubuntu-latest` only: `golangci-lint` (config in `.golangci.yml`).

## License

Source code Anthropic-attributed in the reference repo. This port is for
research and personal use; do not redistribute commercially.

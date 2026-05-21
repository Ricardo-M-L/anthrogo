# Tools

anthrogo ships 30+ built-in tools covering file I/O, shell execution, code intelligence, web access, databases, speech, containers, and background tasks. The model selects tools automatically; every call passes through the [permission gate](configuration.md#permission-model).

## Tool table

| Tool | Read-only | Description |
|------|-----------|-------------|
| `Read` | yes | Read a file with optional `offset`/`limit`; cat-n style line numbers |
| `Write` | no | Write content to a path, creating parent directories as needed |
| `Edit` | no | Replace `old_string` with `new_string` (unique match required unless `replace_all: true`) |
| `Glob` | yes | doublestar glob; results sorted newest-first by mtime |
| `Grep` | yes | Go regexp recursive search with `output_mode` and glob filter |
| `Bash` | no | Run a shell command with `timeout_ms` (default 120000); pass `sandbox: true` for opt-in sandboxing |
| `TodoWrite` | no | Maintain a replace-on-write task list |
| `Diff` | yes | `git diff` wrapper; supports `path`, `cached`, `context`, `stat`, and `range` (commit range) |
| `Format` | no | Format files: gofmt / prettier / black / ruff / rustfmt; supports `paths` batch |
| `Git` | yes | Read-only git subcommands: `status`, `log`, `branch`, `show`, `blame`, `remote` |
| `SymbolSearch` | yes | Find a symbol's definition by name; Go via `go/parser`, others via regex heuristics |
| `References` | yes | Find all word-boundary usages of a name across the tree |
| `WebSearch` | yes | Search the web via brave, google, bing, or tavily |
| `WebFetch` | yes | GET-only HTTP fetch; HTML→markdown, 15-min LRU cache |
| `HTTPRequest` | no* | Full HTTP client: GET/POST/PUT/DELETE/PATCH/HEAD, arbitrary body, headers, `save_to`, size cap. *Read-only for GET/HEAD |
| `SQLQuery` | no* | Run SQL against postgres/mysql/sqlite; DSN supports `env:VARNAME`. SELECT/EXPLAIN/SHOW/DESCRIBE auto-allow; mutating queries ask |
| `SpeechToText` | yes | Transcribe audio via OpenAI Whisper CLI |
| `TextToSpeech` | no | Synthesize speech via platform synthesizer (`say` / `espeak`) |
| `ContainerExec` | no | Run a command in a docker/podman container; network=none default, `--rm` always |
| `BackgroundLaunch` | no | Start `sh -c <command>` in background; returns `task_id` immediately |
| `BackgroundStatus` | yes | Status of one task (pass `task_id`) or list all tasks (omit) |
| `BackgroundOutput` | yes | Fetch captured stdout + stderr for a task |
| `BackgroundCancel` | no | Send cancellation signal to a running task |
| `MCPResource` | yes | List or read resources from MCP servers; default `alwaysAllow` |
| `Skill` | yes | Invoke a named skill — loads SKILL.md into the model context |
| `Task` | no | Spawn a subagent engine; see [Subagents](subagents.md) |

## Permission model

For every tool call `permissions.Decide` runs through priority order:

1. `bypassPermissions` mode → allow
2. `alwaysDeny` rules → deny
3. `acceptEdits` mode → allow Write/Edit/NotebookEdit by default
4. `alwaysAllow` rules → allow
5. `alwaysAsk` rules → ask
6. Otherwise → ask (TUI modal) or deny (headless / `ShouldAvoidPrompts`)

Configure rules in `~/.anthrogo/settings.yaml`:

```yaml
alwaysAllow:
  - tool: Read
  - tool: Glob
  - tool: Grep
  - tool: Bash
    match: "git status*"
alwaysDeny:
  - tool: Bash
    match: "rm -rf*"
```

`match` is a glob pattern matched against the full tool input string. Omit to match all invocations of that tool.

## Bash sandbox

Pass `"sandbox": true` in the Bash tool input to enable two-layer opt-in sandboxing:

1. **AST scan** (`pkg/bashscan/`) — parses via `mvdan.cc/sh/v3/syntax` and blocks commands invoking: `sudo`, `doas`, `rm`, `dd`, `mkfs`, `mount`, `umount`, `chmod`, `chown`, `chroot`, `setuid`, `setgid`. Parse failures are rejected (conservative fallback).
2. **Substring denylist** — rejects commands containing: `../`, `~/.ssh`, `~/.aws`, `~/.gnupg`, `~/.kube`, `/etc/passwd`, `/etc/shadow`, `/private/etc/`, `/var/log`, `/proc/`, `/sys/`.

Sandboxed commands run with `PATH=/usr/bin:/bin:/usr/sbin:/sbin` and have sensitive env vars stripped (`AWS_*`, `SSH_*`, `ANTHROPIC_API_KEY`, etc.).

> **Limitation**: `$VAR`-style indirection is not expanded by the AST scanner. For real isolation use `ContainerExec`.

## ContainerExec

Auto-detects `docker` on PATH; falls back to `podman`. Default network is `none`.

```yaml
tool: ContainerExec
input:
  image: alpine
  command: "echo hello && uname -a"
  network: none
  mounts:
    - /host/data:/data:ro
    - /host/out:/out:rw
  env:
    MY_VAR: hello
  timeout_ms: 30000
  pull_policy: missing   # always | missing (default) | never
  gpu: false             # true adds --gpus all (needs NVIDIA Container Toolkit)
  user: "1000:1000"
  workdir: /app
```

`pull_policy` values:
- `always` — runs `docker pull` before `run` (10-minute timeout)
- `never` — passes `--pull never`; fails if image not locally cached
- `missing` (default) — lets the container runtime decide

`Result.Text` shows stdout; stderr appended after `--- stderr ---` if non-empty. Programmatic callers can read `Result.Data["stdout"]`, `Result.Data["stderr"]`, `Result.Data["exit_code"]`.

## Background task tools

```
BackgroundLaunch({command: "make build", env: {GOOS: "linux"}})
→ {task_id: "abc123"}

BackgroundStatus({task_id: "abc123"})
→ {state: "running", started_at: "2026-05-21T10:00:00Z"}

BackgroundOutput({task_id: "abc123"})
→ {stdout: "...", stderr: "..."}

BackgroundCancel({task_id: "abc123"})
→ {ok: true}
```

Tasks are in-memory only — lost on process restart. Stdout/stderr are fully buffered in RAM; avoid tasks that emit very large output.

## WebSearch backends

Configure in `~/.anthrogo/settings.yaml`:

```yaml
# Brave (default)
webSearch:
  backend: brave
  apiKey: "BSA..."

# Google Custom Search
webSearch:
  backend: google
  apiKey: "AIza..."
  endpoint: "cse-id-here"    # cx parameter

# Bing / Azure Cognitive Search
webSearch:
  backend: bing
  apiKey: "abc..."

# Tavily
webSearch:
  backend: tavily
  apiKey: "tvly-..."

# Disable
webSearch:
  backend: disabled
```

All backends return `{title, url, description}` objects. Google caps at 10 results; Bing at 50; Tavily at 20. The optional `url` field overrides the default endpoint (useful for self-hosted proxies).

## Speech tools

### SpeechToText

Requires: `pip install openai-whisper`

| Parameter | Default | Description |
|-----------|---------|-------------|
| `path` | required | Audio file (wav / mp3 / m4a / flac) |
| `model` | `base` | tiny / base / small / medium / large |
| `binary` | `whisper` | Override whisper binary path |

### TextToSpeech

| Platform | Binary used |
|----------|-------------|
| macOS | `say` (built-in) |
| Linux | `espeak` or `espeak-ng` |
| Windows | not yet supported |

| Parameter | Default | Description |
|-----------|---------|-------------|
| `text` | required | Text to speak |
| `output` | (play live) | Optional output file path (AIFF on macOS, WAV on Linux) |
| `voice` | system default | Voice name (platform-specific) |

## SQLQuery

Supports postgres, mysql, and sqlite via `database/sql`:

```yaml
tool: SQLQuery
input:
  dsn: "postgres://user:pass@host:5432/db"
  # or: dsn: "env:DATABASE_URL"
  query: "SELECT id, name FROM users LIMIT 10"
  params: []
  timeout_ms: 10000
  max_rows: 500
```

## HTTPRequest

```yaml
tool: HTTPRequest
input:
  method: POST
  url: https://api.example.com/v1/items
  headers:
    Content-Type: application/json
    Authorization: "Bearer env:MY_TOKEN"
  body: '{"name": "test"}'
  save_to: /tmp/response.json
  timeout_ms: 30000
  max_bytes: 10485760
```

## Vision / images

Use `@image:<path>` in any prompt to attach a local image:

```
@image:./screenshot.png what's wrong with this UI?
describe @image:~/photo.jpg and @image:~/chart.png
```

Supported MIME types: `image/png`, `image/jpeg`, `image/gif`, `image/webp`. Only local file paths are supported (no URLs in the `@image:` syntax).

- **Anthropic provider**: native `image` content block
- **OpenAI-compatible provider**: `{type: "image_url", image_url: {url: "data:<mime>;base64,..."}}`

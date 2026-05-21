# CLI flags

## Synopsis

```
anthrogo [flags] [prompt]
```

## Global flags

| Flag | Default | Description |
|------|---------|-------------|
| `--version` | — | Print version and exit |
| `-p`, `--print <text>` | — | Headless mode: run prompt and exit (stdin is appended if piped) |
| `--json` | false | Emit ndjson events instead of plain text (headless only) |
| `--model <name>` | provider default | Override model for this session |
| `--provider <name>` | `anthropic` | Active provider profile |
| `--permission-mode <mode>` | `default` | `default` / `acceptEdits` / `bypassPermissions` / `plan` |
| `--cwd <path>` | current dir | Override working directory |
| `--session <id>` | — | Resume a specific session by UUID (or unambiguous prefix) |
| `--resume, -r <id>` | — | Resume the specified session (alias for `--session`) |
| `--continue, -c` | false | Continue the most recent session for this cwd |
| `--cost-limit <USD>` | 0 | Hard-cap estimated session cost; 0 = disabled |
| `--auto-compact <tokens>` | 0 | Auto-compact threshold in tokens; 0 = disabled |
| `--pprof <addr>` | — | Start pprof server at addr (e.g. `localhost:6060`) for debug profiling |
| `--debug` | false | Enable debug logging |
| `--no-color` | false | Disable color output |

## KAIROS worker flags

| Flag | Description |
|------|-------------|
| `--kairos-serve <addr>` | Start a KAIROS worker on the given address (e.g. `:9001`) |
| `--signing-key <path>` | ed25519 private key file for SSE stream signing |
| `--trust-key <path\|base64>` | Trust a KAIROS worker public key (global) |
| `--tls-cert <path>` | TLS certificate file |
| `--tls-key <path>` | TLS private key file |
| `--tls-auto` | Enable Let's Encrypt autocert |
| `--tls-domain <domains>` | Comma-separated domains for autocert |
| `--generate-key <path>` | Generate an ed25519 keypair at the given path stem |

## Subcommands

| Subcommand | Description |
|-----------|-------------|
| `anthrogo init-config` | Interactive wizard to create `~/.anthrogo/settings.yaml` |
| `anthrogo doctor` | Environment self-check (~20 checks; PASS/WARN/FAIL report) |
| `anthrogo serve` | Start the HTTP API daemon (REST/SSE) — see below |
| `anthrogo web` | Start the embedded browser UI — see below |

### anthrogo serve

Start a long-lived HTTP server that exposes the engine as a REST/SSE API.

| Flag | Default | Description |
|------|---------|-------------|
| `--addr` | `127.0.0.1:8765` | Listen address for the HTTP server |
| `--token` | — | Bearer auth token; all routes require `Authorization: Bearer <token>` if set |
| `--cors-origin` | — | `Access-Control-Allow-Origin` header value (e.g. `https://myapp.com`) |
| `--sessions-dir` | `~/.anthrogo` | Override session storage directory |
| `--model` | settings.yaml | Model alias override |
| `--provider` | settings.yaml | Provider profile name override |

**Endpoints:**

| Method | Path | Description |
|--------|------|-------------|
| `POST` | `/v1/chat` | Send a message; `stream: true` enables SSE streaming |
| `GET` | `/v1/sessions` | List recent sessions (up to 100) |
| `GET` | `/v1/sessions/{id}` | Fetch full JSONL records for a session |
| `DELETE` | `/v1/sessions/{id}` | Delete a session file |
| `GET` | `/v1/tools` | List registered tools |
| `GET` | `/v1/health` | Server health and uptime |

**Quick check:**

```bash
anthrogo serve &
curl http://127.0.0.1:8765/v1/health
```

### anthrogo web

Start the embedded browser UI (vanilla JS SPA served via `embed.FS`). Automatically opens the browser unless `--no-browser` is set. Supports the same `/v1/*` API endpoints as `serve`, plus the SPA at `/`.

| Flag | Default | Description |
|------|---------|-------------|
| `--addr` | auto (8766–8775) | Listen address; auto-detects a free port in range 8766–8775 if not set |
| `--token` | — | Bearer auth token |
| `--cors-origin` | — | `Access-Control-Allow-Origin` header value |
| `--sessions-dir` | `~/.anthrogo` | Override session storage directory |
| `--model` | settings.yaml | Model alias override |
| `--provider` | settings.yaml | Provider profile name override |
| `--no-browser` | false | Do not open the browser automatically |

## Examples

```bash
# Interactive TUI
anthrogo

# Headless — print assistant text
anthrogo -p "explain main.go"

# Headless — with piped stdin
git diff | anthrogo -p "summarize this diff"

# Headless — ndjson output
cat README.md | anthrogo --json -p "3 bullets" | jq -r '.text // empty'

# Accept file edits without asking
anthrogo --permission-mode acceptEdits -p "fix the typo in README"

# Override model
anthrogo --model claude-haiku-4-5-20251001

# Use a different provider profile
anthrogo --provider deepseek -p "summarize this repo"

# Start a KAIROS worker
KAIROS_AUTH_TOKEN=secret123 anthrogo --kairos-serve :9001
```

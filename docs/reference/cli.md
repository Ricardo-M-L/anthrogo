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
| `--cost-limit <USD>` | 0 | Hard-cap estimated session cost; 0 = disabled |
| `--auto-compact <tokens>` | 0 | Auto-compact threshold in tokens; 0 = disabled |
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

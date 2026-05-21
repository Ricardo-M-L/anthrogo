# anthrogo

A Go port of Anthropic's Claude Code CLI, reconstructed from the
source-mapped `@anthropic-ai/claude-code@2.1.88` package.

> **Status**: v0.13.0-dev. Documentation: https://Ricardo-M-L.github.io/anthrogo/

anthrogo re-expresses Claude Code's architecture in Go: preserving the shapes
of `Tool`, `QueryEngine`, `PermissionContext`, `ToolUseContext`, MCP client,
hooks, skills, and plugins — while replacing the Ink UI with a Bubble Tea
front-end. It is not a 1:1 line transliteration: feature flags become Go build
tags, Zod becomes JSON schema, React components become Bubble Tea
update/view loops.

The upstream CLI is TypeScript + Bun + React/Ink. anthrogo targets the same
behavioral contract while being a single statically-linked Go binary with no
Node.js runtime requirement.

## Quickstart

```bash
go install github.com/Ricardo-M-L/anthrogo/cmd/anthrogo@latest
anthrogo init-config   # interactive wizard → ~/.anthrogo/settings.yaml
anthrogo doctor        # ~20 environment checks
anthrogo               # launch TUI
```

Use `anthrogo -p "explain main.go"` for headless (non-interactive) mode.

## Highlights

| Feature area | Details |
|---|---|
| **6 providers** | [Anthropic](docs/providers/anthropic.md), [OpenAI-compat](docs/providers/openai.md) (DeepSeek/Kimi/MiniMax/GLM), [Bedrock](docs/providers/bedrock.md), [Vertex](docs/providers/vertex.md), [Ollama](docs/providers/ollama.md), [Failover](docs/providers/failover.md) |
| **4 MCP transports** | stdio · SSE · Streamable HTTP · WebSocket + OAuth 2.1 PKCE — [docs/mcp.md](docs/mcp.md) |
| **30+ tools** | Bash sandbox · ContainerExec · Diff · Format · Git · SymbolSearch · References · WebFetch · WebSearch (4 backends) · HTTPRequest · SQLQuery · Speech I/O · Background tasks · **BrowserAction** (headless Chrome) · **SlackPost** (webhook) · **CalendarEvent** (.ics / Calendar.app) — [docs/tools.md](docs/tools.md) |
| **9 hooks + Skills + Plugins** | PreToolUse · PostToolUse · UserPromptSubmit · Stop · SubagentStop · PreCompact + more — [docs/hooks.md](docs/hooks.md) · [docs/skills.md](docs/skills.md) · [docs/plugins.md](docs/plugins.md) |
| **Subagents + KAIROS** | Concurrent Task dispatch · user-defined YAML types · remote cross-process workers — [docs/subagents.md](docs/subagents.md) · [docs/kairos.md](docs/kairos.md) |
| **Persistent sessions + search** | JSONL storage · /sessions {list/show/replay/search/export/delete/stats/diff/reindex} · SQLite-backed L2 cache — [docs/sessions.md](docs/sessions.md) |
| **TUI multi-pane + mouse** | F2 cycles single/split/triple · wheel scroll · left-click URL open · custom themes |
| **/cost + budget + auto-compact** | Built-in pricing for 20+ models · budget cap · auto-compact threshold — [docs/cost.md](docs/cost.md) · [docs/compaction.md](docs/compaction.md) |
| **/login OAuth + /telemetry** | OAuth 2.1 PKCE for corporate SSO · opt-in anonymous telemetry — [docs/providers/anthropic.md](docs/providers/anthropic.md) |

## Reliability

### Stream retry

If the provider's HTTP/SSE stream is cut mid-response (network blip, proxy timeout), anthrogo automatically retries with exponential backoff (200 ms → 600 ms → 2 s, up to 3 attempts by default). Each retry emits a `KindStreamRetry` event so the TUI can show a `[reconnecting attempt N/3]` status. After exhausting all retries the original error is returned wrapped with `"stream retry exhausted"`. The retry cap is configurable via `Config.MaxStreamRetries`.

### Cancel-safe tool drain

When the user presses Ctrl-C, anthrogo does not immediately discard in-flight concurrent tool goroutines. Instead it waits up to `Config.MaxToolDrainTimeout` (default 5 s) for any running goroutines to finish before closing the event channel. During the drain window `KindCancelDraining` events are emitted so the TUI can show a `[draining N tools …]` indicator. If the timeout is exceeded a warning is logged and the function returns; goroutines that ignored their context may still be running (documented).

## Roadmap

| Milestone | Scope | Status |
|-----------|-------|--------|
| M1–M4 | TUI REPL · core tools · MCP · hooks · skills · plugins | shipped |
| M5.x | Subagents (concurrent, isolated, KAIROS) | shipped |
| M6.x | OAuth PKCE · MCP resources · elicitations | shipped |
| M8–M9.x | Form UI · Diff/Format/Git · LSP-style code intel · KAIROS multi-hop | shipped |
| M10.x | Bash AST sandbox · WebSearch multi-backend · /audit · mouse TUI | shipped |
| M11.x | Multi-pane TUI · Background tasks · plugin remote install · /login · Speech I/O · KAIROS signing + TLS | shipped |
| M12.x | `doctor` · `init-config` · HTTPRequest · SQLQuery · Ollama provider | shipped |
| **M13.1** | **Documentation restructure (this release)** | **shipped** |
| M13.x | API godoc auto-gen · migration guide · example projects | planned |
| M6 (remaining) | Bedrock/Vertex full feature parity | planned |

## Development

```bash
make build              # produces ./bin/anthrogo (version-stamped)
make test               # go test ./...
make race               # race detector on hot packages
make lint               # golangci-lint (install: brew install golangci-lint)
make release            # cross-compile for darwin/linux × amd64/arm64 → dist/
make install            # go install to $GOPATH/bin
```

CI (`.github/workflows/ci.yml`) runs on every push and PR to `main`:
- **test** job — matrix over `ubuntu-latest` + `macos-latest`
- **lint** job — `ubuntu-latest` with golangci-lint

## Repository layout

```
anthrogo/
├── cmd/anthrogo/        # CLI entry + cobra commands
├── internal/
│   ├── tui/             # Bubble Tea models
│   ├── headless/        # -p stdout path
│   ├── config/          # settings + paths
│   ├── system/          # system prompt + CLAUDE.md walker
│   ├── session/         # conversation state
│   ├── mcp/             # MCP client (Manager + Server + LogSink)
│   └── version/         # version string
├── pkg/
│   ├── message/         # ContentBlock types
│   ├── provider/        # Provider interface + implementations
│   ├── tool/            # Tool framework + built-ins
│   ├── permissions/     # PermissionContext + gate
│   ├── query/           # QueryEngine — owns the turn loop
│   ├── pricing/         # built-in price defaults
│   └── bashscan/        # Bash AST safety scanner
└── docs/                # mkdocs site (https://Ricardo-M-L.github.io/anthrogo/)
```

## License

Source code Anthropic-attributed in the reference repo. This port is for
research and personal use; do not redistribute commercially.

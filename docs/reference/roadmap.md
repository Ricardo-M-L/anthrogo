# Roadmap

## Shipped milestones

| Milestone | Scope | Status |
|-----------|-------|--------|
| M1 | TUI REPL + 7 core tools + Anthropic SDK + permission gate + CLAUDE.md | shipped |
| M2 | More tools, session persistence, plan mode, slash-command palette | shipped |
| M3 | MCP client | shipped |
| M4 | Hooks, skills, plugins | shipped |
| M5.1 | Subagents (Task tool + sub-engine, depth limit, SubagentStop hook) | shipped |
| M5.2 | MCP resources + minimal elicitations | shipped |
| M5.3 | Concurrent subagents, isolated perms, user-defined YAML types | shipped |
| M6.3 | Real TUI form elicitation handler (JSON-blob form modal) | shipped |
| M6.5 | OAuth 2.1 PKCE client flow for MCP HTTP transports | shipped |
| M6.6 | KAIROS coordinator (cross-process subagent dispatch) | shipped |
| M8.9 | Multi-field form elicitation UI | shipped |
| M8.11 | Diff / Format / Git built-in tools | shipped |
| M9.3 | Multi-hop + remote hook/perm context | shipped |
| M9.4 | Subagent real-time stream to parent TUI | shipped |
| M9.5 | SymbolSearch + References (LSP-style code intel) | shipped |
| M9.7 | Form UI: cursor nav, enum cycler, Ctrl+J newline, schema defaults | shipped |
| M9.8 | `Diff.range`, `Format.paths` batch, per-nest subagent JSONL | shipped |
| M9.9 | Model + path + visibility polish, KAIROS hook resolver | shipped |
| M9.10 | Persistent input history (Up/Down nav), `/history` | shipped |
| M10.3 | WebSearch multi-backend (brave/google/bing/tavily) | shipped |
| M10.4 | `/audit` session scanning | shipped |
| M10.10 | Bash AST safety scan (`pkg/bashscan/`, sandbox binary denylist) | shipped |
| M10.13 | TUI mouse support: wheel scroll, left-click URL open | shipped |
| M11.1 | TUI multi-pane layout (F2 cycles single/split/triple) | shipped |
| M11.2 | Background task tools: BackgroundLaunch/Status/Output/Cancel | shipped |
| M11.3 | Plugin remote install: `/plugin install <url>` and `git+https://` | shipped |
| M11.4 | mkdocs documentation site | shipped |
| M11.5 | `/login` OAuth 2.1 PKCE; Anthropic provider prefers saved token | shipped |
| M11.6 | Speech I/O: SpeechToText (whisper) + TextToSpeech (say/espeak) | shipped |
| M11.9 | KAIROS ed25519 SSE signature verification | shipped |
| M11.10 | KAIROS TLS (cert file + Let's Encrypt autocert) | shipped |
| M12.1 | `anthrogo doctor` self-check | shipped |
| M12.2 | `anthrogo init-config` interactive wizard | shipped |
| M12.3 | `HTTPRequest` tool | shipped |
| M12.4 | `SQLQuery` tool | shipped |
| M12.5 | Local Ollama provider (`type: ollama`) | shipped |
| M13.1 | Documentation restructure (this page) | shipped |

## Upcoming / deferred

- M13.x — API reference auto-gen from godoc
- M13.x — Migration guide v0.3 → v0.13
- M13.x — Example projects directory
- Bedrock / Vertex / OpenAI-compat multi-provider (M6)
- mkdocs versioning via `mike`
- Sigstore / GPG plugin signature verification
- Shallow-fetch tag resolution for git-sourced plugins
- Partial-stream retry for failover

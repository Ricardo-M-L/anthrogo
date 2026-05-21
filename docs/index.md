# anthrogo

A Go port of Anthropic's Claude Code CLI, reconstructed from the source-mapped `@anthropic-ai/claude-code@2.1.88` package.

[![CI](https://github.com/Ricardo-M-L/anthrogo/actions/workflows/ci.yml/badge.svg)](https://github.com/Ricardo-M-L/anthrogo/actions)

**Status**: v0.13.0-dev &nbsp;|&nbsp; [GitHub](https://github.com/Ricardo-M-L/anthrogo)

## What is it?

anthrogo re-expresses Claude Code's architecture in Go: preserving the shapes of Tool, QueryEngine, PermissionContext, ToolUseContext, MCP client, hooks, skills, and plugins — while replacing the Ink UI with a Bubble Tea front-end. It is not a 1:1 line transliteration: feature flags become Go build tags, Zod becomes JSON schema, React components become Bubble Tea update/view loops.

## Key features

- **6 providers**: [Anthropic](providers/anthropic.md), [OpenAI-compat](providers/openai.md) (DeepSeek/Kimi/MiniMax/GLM), [Bedrock](providers/bedrock.md), [Vertex](providers/vertex.md), [Ollama](providers/ollama.md) + [failover](providers/failover.md)
- **4 MCP transports**: stdio, SSE, Streamable HTTP, WebSocket + OAuth 2.1 PKCE — see [MCP](mcp.md)
- **30+ built-in tools**: Bash sandbox, ContainerExec, Diff, Format, Git, SymbolSearch, References, WebFetch, WebSearch (4 backends), HTTPRequest, SQLQuery, Speech I/O, Background tasks — see [Tools](tools.md)
- **9 hook events** + Skills + Plugins + Subagents (concurrent + KAIROS remote dispatch) — see [Hooks](hooks.md), [Skills](skills.md), [Plugins](plugins.md), [Subagents](subagents.md)
- **/compact** with auto-trigger, `/cost` with built-in pricing, budget caps — see [Compaction](compaction.md), [Cost](cost.md)
- **/sessions** {list/show/replay/search/export/delete/stats/diff/reindex} — see [Sessions](sessions.md)
- **Persistent SQLite-backed search index** (two-level: in-memory LRU + SQLite L2)
- **Real tokenizer** (tiktoken for OpenAI-family; char/4 approximation for Claude)
- **Multi-pane TUI** (F2 cycle), mouse support, markdown rendering, custom themes
- **KAIROS** cross-process subagent dispatch with ed25519 signing + TLS — see [KAIROS](kairos.md)

## Get started

[Installation](install.md) → [First run](first-run.md) → [Configuration](configuration.md)

# anthrogo

A Go port of Anthropic's Claude Code CLI, reconstructed from the source-mapped `@anthropic-ai/claude-code@2.1.88` package.

[![CI](https://github.com/Ricardo-M-L/anthrogo/actions/workflows/ci.yml/badge.svg)](https://github.com/Ricardo-M-L/anthrogo/actions)

## What is it?

anthrogo re-expresses Claude Code's architecture in Go: preserving the shapes of Tool, QueryEngine, PermissionContext, ToolUseContext, MCP client, hooks, skills, plugins — while replacing the Ink UI with a Bubble Tea front-end.

## Key features

- **3 providers**: Anthropic, OpenAI-compatible (DeepSeek/Kimi/MiniMax/GLM), Bedrock, Vertex
- **4 MCP transports**: stdio, SSE, Streamable HTTP, WebSocket + OAuth 2.1 PKCE
- **20+ built-in tools** including Bash sandbox, Diff, Format, Git, ContainerExec, SymbolSearch, References, WebFetch, WebSearch (multi-backend)
- **9 hook events** + Skills + Plugins + Subagents (concurrent + KAIROS remote dispatch)
- **/compact** with auto-trigger, /cost with built-in pricing, budget caps, /sessions {list/show/replay/search/export/delete/stats/diff/reindex}
- **Persistent SQLite-backed search index**
- **Real tokenizer** (tiktoken + Anthropic count_tokens API opt-in)
- **Multi-pane TUI** (F2 cycle), mouse support, markdown rendering, custom themes

[Get started](install.md)

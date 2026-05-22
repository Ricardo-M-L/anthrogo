# anthrogo

[English](README.md) | **简体中文**

[![CI](https://github.com/Ricardo-M-L/anthrogo/actions/workflows/ci.yml/badge.svg)](https://github.com/Ricardo-M-L/anthrogo/actions/workflows/ci.yml)
[![Docs](https://github.com/Ricardo-M-L/anthrogo/actions/workflows/docs.yml/badge.svg)](https://ricardo-m-l.github.io/anthrogo/)
[![Release](https://img.shields.io/github/v/release/Ricardo-M-L/anthrogo)](https://github.com/Ricardo-M-L/anthrogo/releases/latest)
[![License: AGPL-3.0](https://img.shields.io/badge/License-AGPL_v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)

把 Anthropic 的 Claude Code CLI 用 Go 重写了一遍——从 source-mapped
`@anthropic-ai/claude-code@2.1.88` 反推架构，保留 `Tool` /
`QueryEngine` / `PermissionContext` / MCP / hooks / skills / plugins
的接口形状，把 Ink UI 换成 Bubble Tea，把 Zod schema 换成 JSON
schema，把 React 组件换成 update/view 循环。

最终是一个**静态链接的单文件 Go 二进制**，不依赖 Node.js。

> **当前版本**：v0.14.0（首个稳定发布）。文档：https://ricardo-m-l.github.io/anthrogo/

## TUI 长什么样

```
┌─ anthrogo · ~/code/myproject ─────────────────────────── claude-opus-4-7 ─┐
│                                                                            │
│  > 把 pkg/auth 重构成用新的 token 中间件                                  │
│                                                                            │
│  ● Read pkg/auth/middleware.go (87 行)                                     │
│  ● Read pkg/auth/token.go (124 行)                                         │
│  ● Grep "WithToken" --type go                                              │
│     ↳ 14 处 匹配 跨 8 文件                                                │
│                                                                            │
│  我把 3 个还没用 helper 的调用点修一下。计划:                             │
│    1. cmd/api/server.go:42 — 包 WithToken                                 │
│    2. internal/handlers/user.go:118 — 同上                                │
│    3. internal/handlers/admin.go:73 — 同上                                │
│                                                                            │
│  ● Edit cmd/api/server.go (+1, -1) ✓                                       │
│  ● Edit internal/handlers/user.go (+1, -1) ✓                               │
│  ● Edit internal/handlers/admin.go (+1, -1) ✓                              │
│  ● Bash "go test ./pkg/auth/..."                                          │
│     ↳ ok  github.com/foo/myproject/pkg/auth   2.342s                       │
│                                                                            │
├─ history ──────────────────────────────────────────────────────────────────┤
│  > _                                                                       │
└── Ctrl+E 编辑 · F2 布局 · /help · $0.0421 · 12.3k tokens ──────────────────┘
```

## 快速开始

### 安装

**macOS / Linux 用 Homebrew（推荐）**

```bash
brew install Ricardo-M-L/tap/anthrogo
```

**预编译二进制** — 去
[Releases](https://github.com/Ricardo-M-L/anthrogo/releases/latest)
挑你的平台（linux/darwin × amd64/arm64），解压扔进 `$PATH`。

**从源码（需要 Go 1.26+）**

```bash
go install github.com/Ricardo-M-L/anthrogo/cmd/anthrogo@latest
```

### 首次使用

```bash
anthrogo init-config   # 交互式向导 → ~/.anthrogo/settings.yaml
anthrogo doctor        # 跑 ~20 项环境自检
anthrogo               # 启动 TUI
```

非交互（脚本 / pipe）模式：

```bash
anthrogo -p "解释一下 main.go"
echo "总结一下" | anthrogo --json
```

HTTP 服务模式：

```bash
anthrogo serve --addr 127.0.0.1:8765
curl http://127.0.0.1:8765/v1/health
```

浏览器 UI：

```bash
anthrogo web   # 自动打开 http://127.0.0.1:8766
```

## 主要特性

| 类别 | 内容 |
|---|---|
| **6 个 provider** | Anthropic、OpenAI 兼容（DeepSeek/Kimi/MiniMax/GLM）、Bedrock、Vertex、Ollama、Failover |
| **4 种 MCP transport** | stdio · SSE · Streamable HTTP · WebSocket + OAuth 2.1 PKCE |
| **30+ 内置工具** | Bash 沙箱 · ContainerExec · Diff · Format · Git · SymbolSearch · References · WebFetch · WebSearch（4 backend）· HTTPRequest · SQLQuery · Speech I/O · 后台任务 · **BrowserAction** (chromedp) · **SlackPost** · **CalendarEvent** · **Embed** · **ImageGen** · PDFRead · XlsxRead |
| **9 个 hook + Skills + Plugins** | PreToolUse · PostToolUse · UserPromptSubmit · Stop · SubagentStop · PreCompact 等 |
| **Subagent + KAIROS** | 并发 Task 分发 · YAML 定义类型 · 远程跨进程 worker |
| **会话持久化 + 搜索** | JSONL 存储 · /sessions（list/show/replay/search/export/delete/stats/diff/reindex）· SQLite L2 缓存 |
| **TUI 多 pane + 鼠标** | F2 切换单/双/三栏 · 滚轮 · 左键打开链接 · 自定义主题 |
| **/cost + 预算 + 自动 compact** | 20+ 模型内置定价 · 预算上限 · 自动 compact 阈值 |
| **HTTP daemon (`serve`)** | REST + SSE API：/v1/chat（同步 + 流式）、/v1/sessions CRUD、/v1/tools、/v1/health · Bearer 鉴权 · CORS · LRU session 缓存 |
| **浏览器 UI (`web`)** | 内嵌 vanilla-JS SPA（无 npm/React），sessions 侧栏、SSE 流式聊天、设置面板 |

## 安全审计

v0.14.0 之前跑过两轮深度安全审计（M14 + M15），修了 **11 项 critical 漏洞**：

- SSRF 防护（httprequest / webfetch / embed / imagegen）+ DNS-rebinding 防御
- 时序攻击安全的 bearer token 比较
- 归档 setuid/setgid 位剥离
- Hook 子进程 env 脱敏（剥离所有 API key）
- TOCTOU 修复（serve session cache）
- Web UI XSS 修复
- subagent `insecure_skip_verify` 改 env 显式 opt-in

威胁模型 + 漏洞披露流程见 [SECURITY.md](SECURITY.md)。

## 贡献

参考 [CONTRIBUTING.md](CONTRIBUTING.md)。简短版：

1. fork → 改 → 跑 `go test ./...` → 跑 `go test -race ./...` → 提 PR
2. 错误信息小写、不带句号
3. 测试用 `t.TempDir()`、不要 `time.Sleep` 当同步用
4. 不接受 GPL-2.0-only / 专有许可证的代码

## License

AGPL-3.0。详细条款见 [LICENSE](LICENSE)。这是一条**网络 copyleft**：
如果你把 anthrogo 改成一个网络服务暴露给别人用，你必须把改后的源码
也开源给那些用户。

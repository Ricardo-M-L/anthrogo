# Contributing to anthrogo

Thanks for considering a contribution. anthrogo is AGPL-3.0 licensed; by
submitting a patch you agree to license it under the same terms.

## Quick start

```bash
git clone https://github.com/<your-fork>/anthrogo.git
cd anthrogo
go build ./...
go test ./...
```

Required: **Go 1.26+** (pinned by chromedp's `cdproto` transitive — see
`go.mod` for the rationale). Older Go won't build until BrowserAction is
moved behind a build tag (a future milestone).

## Repository layout

The big picture:

| Directory | What it holds |
|---|---|
| `cmd/anthrogo/` | The `anthrogo` binary entry point + subcommands. |
| `pkg/tool/` | Built-in tools: Read, Write, Edit, Bash, Glob, Grep, WebFetch, etc. |
| `pkg/query/` | The `Engine` — message loop, tool dispatch, subagent orchestration. |
| `pkg/permissions/` | Gate logic (allow/deny/ask rules, plan mode, hook decisions). |
| `pkg/provider/` | LLM provider implementations (anthropic, openai, bedrock, vertex, ollama, failover, fake). |
| `pkg/skill/` `pkg/plugin/` `pkg/subagent/` | User-installable extensions. |
| `pkg/kairos/` | Cross-process subagent dispatch + signing + TLS. |
| `pkg/command/` | Slash command builtins (`/sessions`, `/compact`, `/hooks`, etc.). |
| `pkg/bgtasks/` | The `BackgroundLaunch` worker pool. |
| `internal/serve/` | HTTP daemon (`anthrogo serve`). |
| `internal/web/` | Embedded SPA + handler (`anthrogo web`). |
| `internal/mcp/` | MCP client transport + manager. |
| `internal/hooks/` | Hook runner (subprocess execution with sanitized env). |
| `internal/session/` | JSONL session store + SQLite replay cache. |
| `internal/tui/` | Bubbletea TUI surfaces. |
| `docs/` | mkdocs-served Markdown documentation. |

## Development workflow

1. **Branch from `main`.** Name like `feat/<topic>` or `fix/<issue>`.
2. **Write the test first.** This repo has 200+ tests; a regression-test
   for the bug is more important than the fix itself.
3. **Run the full suite.** `go test ./...` should pass. `go test -race
   ./...` should pass too — race-free is a hard requirement.
4. **Run vet + lint.** `go vet ./...` and `make lint` (if available) should
   be clean.
5. **Commit small.** One commit per logical change. Reference the file:line
   in commit messages when fixing a specific bug.
6. **Open a PR** with a description of the problem, your approach, and the
   test that demonstrates the fix.

## Code style

- Plain Go style: `gofmt`, no aliasing of stdlib types, prefer composition
  over inheritance, prefer concrete types over interfaces unless multiple
  implementations exist.
- Error messages are lowercase, no trailing period, no exclamation marks.
- Logging: `log.Printf("anthrogo: <component>: <msg>")` for stderr lines;
  structured logging (`slog`) is not yet wired but acceptable for new code.
- No new external deps without discussion in the PR.
- Don't introduce build tags unless absolutely required.
- Don't add `Co-Authored-By` trailers to commit messages.

## Test conventions

- `pkg/foo/foo_test.go` lives next to `pkg/foo/foo.go`.
- Test names: `TestType_Method_Scenario` (e.g., `TestEngine_RunSubagent_DepthLimit`).
- Tests must run without network or hardware. Tools that hit external APIs
  (Slack, OpenAI, etc.) use `httptest.NewServer`. Tests that need loopback
  set `t.Setenv("ANTHROGO_NETGUARD_ALLOW_LOOPBACK", "1")` to bypass the SSRF
  guard.
- Use `t.TempDir()` for any file IO. Never write outside it.
- Don't add `time.Sleep` for synchronization — use channels or polling
  with a timeout. Existing `sleep`-based tests are tech debt; please don't
  add more.

## What to work on

Open issues in the GitHub tracker, OR look at the audit deferral list:

- M15.4b items (O1, O3, O4, O5, O6, O8, O9, T1, T2) in `CHANGELOG.md`.
- Any `TODO(m15.4b):` or similar in source code.

If you have an idea outside the audit list, open an issue first to discuss
scope.

## Reporting bugs

For non-security bugs: open an issue with a minimal reproduction. For
security issues: see [SECURITY.md](SECURITY.md).

## License notes

AGPL-3.0 means contributions must be license-compatible with AGPL — no
inbound code under proprietary or copyleft-incompatible licenses (GPL-2.0
without "or later", MPL-1.x, original BSD-4-clause, etc.). MIT, BSD-2/3,
Apache-2.0, ISC, and zlib contributions are fine.

If your patch incorporates code from another project, include the source
attribution + a note about its license at the top of the file.

# Subagents

The model can spawn isolated sub-engines via the `Task` tool to perform self-contained multi-step tasks. Unlike skill invocations (which just return markdown), a subagent runs its own full tool-use loop and returns its final answer as a tool result.

## Basic dispatch

```json
Task({
  "description": "find all TODO comments",
  "prompt": "Search the codebase for TODO comments. Return a list.",
  "subagent_type": "general-purpose"
})
```

The subagent has **no memory** of the parent conversation — brief it fully in `prompt`. It inherits the parent's tool registry (unless the type restricts it), permission gate, and hook manager.

## Concurrent dispatch

When the model emits multiple `Task` tool-use blocks in a single turn, the engine runs them concurrently. Tool result order is preserved. Log/stderr output from concurrent subagents may interleave.

## Permission isolation

Each subagent runs with a cloned `permissions.Context`. Mode toggles (e.g. the model entering plan mode inside a subagent) do not leak back to the parent.

## Recursion limit

Nested subagents are allowed up to depth 3 by default (`MaxSubagentDepth = 3`). Calls beyond the limit return an error to the model.

## Plan mode

`Task` is treated as a write tool. Plan mode blocks it. Switch to default mode with `/mode default`.

## Real-time streaming

Subagent text deltas are forwarded to the parent TUI in real time, prefixed with `[Task: <description>] `. Deltas are buffered until newline boundaries to avoid scroll spam; the remaining buffer is flushed when the subagent finishes. Remote KAIROS subagents stream via `event: text` SSE messages using the same callback.

## Independent JSONL

Each subagent run writes its own JSONL alongside the parent session for later inspection:

```
~/.anthrogo/projects/<cwd-hash>/<session-id>/subagents/<subagent-id>.jsonl
```

A `subagent_start` record in the parent JSONL provides the ID for cross-referencing.

## Custom subagent types

Drop YAML files into `~/.anthrogo/subagents/` (home) or `<cwd>/.anthrogo/subagents/` (project-local; overrides home):

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

Field rules:
- `name` — lowercase alphanumeric + hyphens, `^[a-z][a-z0-9-]{0,63}$`, must match the filename stem
- `description` — required (shown to the model in the Task tool schema)
- `system_prompt_suffix` — optional extra instruction appended to the system prompt for this subagent
- `tool_allowlist` — optional; empty = inherit parent's full tool registry
- `model` — optional model override; empty = inherit parent model (see below)
- `general-purpose` is reserved and cannot be overridden

## Per-subagent model override

Set `model:` to run a subagent on a different model than the parent session. Useful for cost/speed trade-offs — e.g., a heavy `code-reviewer` on Opus while the parent uses Sonnet, or a `summarizer` on Haiku for speed.

```yaml
# ~/.anthrogo/subagents/fast-summarizer.yaml
name: fast-summarizer
description: Summarise large documents quickly using a lighter model.
model: claude-haiku-4-5-20251001
system_prompt_suffix: |
  You are a concise summariser. Return bullet points only.
```

If the provider does not support the specified model, the error is returned to the parent as the subagent's tool result. No validation of the model name is performed at load time (it is provider-dependent).

## Slash commands

```
/subagents                # list loaded types
/subagents show <name>    # inspect a type definition
/subagents reload         # hot-reload without restarting
```

Note: the system prompt is built at startup; newly added types won't be advertised to the model until restart.

## /refactor — multi-file refactor via subagent

```
/refactor <glob-pattern> -- <natural-language instruction>
```

Resolves the glob under the current working directory (supports `**` via `doublestar`), then spawns a `refactor` subagent with a restricted tool allowlist (`Read`, `Edit`, `Write`, `Glob`, `Grep`) to apply the instruction across all matched files.

Limits:
- Maximum **50 matched files** per invocation (narrow the pattern if exceeded).
- Per-file read cap advisory: **50 KB** (larger files are listed with a note to use `Read` with `offset`/`limit`).
- Total in-scope advisory: **500 KB** (a warning is included in the subagent prompt).

Examples:

```
/refactor pkg/tool/**/*.go -- add a doc-comment to every exported function that is missing one
/refactor internal/**/*.go -- replace all uses of fmt.Sprintf("%v", err) with err.Error()
/refactor cmd/**/*.go -- rename the variable cfg to config throughout
```

The subagent's final per-file summary is returned as the command output.

## Remote subagents (KAIROS)

Add `remote:` to a subagent YAML to dispatch to a KAIROS worker:

```yaml
# ~/.anthrogo/subagents/heavy-research.yaml
name: heavy-research
description: Use for long research that benefits from the worker's tools.
remote:
  endpoint: http://worker.example.com:9001
  auth_token: env:KAIROS_AUTH_TOKEN
  exec_tools_locally: false   # true = worker forwards tool calls back to client
```

See [KAIROS](kairos.md) for the full remote subagent reference, multi-hop chains, signature verification, and TLS.

## SubagentStop hook

Fires after every subagent completes (success or error). Wire it in `settings.yaml`:

```yaml
hooks:
  SubagentStop:
    - command: ~/.anthrogo/hooks/subagent-done.sh
```

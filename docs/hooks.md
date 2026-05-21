# Hooks

anthrogo runs user-defined shell commands at 9 lifecycle events. Add a `hooks:` block to `~/.anthrogo/settings.yaml` (home-level, applies to all sessions) or `.anthrogo/hooks.yaml` inside a project directory (project-level, appended to the home block).

## The 9 hook events

| Event | Timing | When it fires |
|-------|--------|---------------|
| `PreToolUse` | sync | Immediately before a tool call executes |
| `PostToolUse` | sync | After a tool call returns its result |
| `UserPromptSubmit` | sync | After the user submits a prompt, before the engine turn |
| `Stop` | async | When the session ends or `/quit` is run |
| `SubagentStop` | async | After every subagent completes (success or error) |
| `Notification` | async | When the TUI emits a notification event |
| `SessionStart` | async | At session initialization |
| `SessionStop` | async | At session teardown |
| `PreCompact` | sync | Synchronously before `/compact` runs |

Default timeouts: 30 s for sync events; 5–10 s for async events. Async hooks fire on a background goroutine.

## Configuration

```yaml
hooks:
  PreToolUse:
    - matcher: "Bash"
      command: ~/.anthrogo/hooks/audit.sh
      timeout: 30s
    - matcher: "Write|Edit|NotebookEdit"
      command: ~/.anthrogo/hooks/protect-secrets.sh
  PostToolUse:
    - matcher: "Write|Edit"
      command: ~/.anthrogo/hooks/gofmt.sh
  UserPromptSubmit:
    - command: ~/.anthrogo/hooks/inject-cwd.sh
  Stop:
    - command: ~/.anthrogo/hooks/notify-slack.sh
  SubagentStop:
    - command: ~/.anthrogo/hooks/subagent-done.sh
  PreCompact:
    - command: ~/.anthrogo/hooks/pre-compact.sh
```

`matcher` is a Go regexp tested against the tool name. It applies only to `PreToolUse` and `PostToolUse`. Omit `matcher` to match every tool (or any event type).

anthrogo does not auto-create `~/.anthrogo/hooks/` — create the directory and scripts yourself, then `chmod +x` each script.

## JSON envelope (stdin)

Each hook receives one JSON object on stdin describing the event. The envelope shape depends on the event type:

### PreToolUse / PostToolUse

```json
{
  "event":     "PreToolUse",
  "tool_name": "Bash",
  "tool_input": { "command": "git status" },
  "session_id": "abc123"
}
```

`PostToolUse` additionally includes:

```json
{
  "tool_result": { "output": "On branch main\n..." },
  "is_error": false
}
```

### UserPromptSubmit

```json
{
  "event":   "UserPromptSubmit",
  "prompt":  "explain main.go",
  "session_id": "abc123"
}
```

### Stop / SubagentStop

```json
{
  "event":      "Stop",
  "session_id": "abc123",
  "reason":     "user_quit"
}
```

## Exit codes and stdout responses

| Exit code | Meaning |
|-----------|---------|
| 0 | OK; optional JSON on stdout to influence behaviour |
| 2 | **Block**: PreToolUse → deny the tool call; UserPromptSubmit → abort the prompt |
| other | Hook error; logged and ignored (non-blocking) |

When a hook exits 0 and writes JSON to stdout, the following fields are recognised:

| Field | Applicable to | Effect |
|-------|--------------|--------|
| `permissionDecision` | PreToolUse | `"allow"` or `"deny"` — overrides permission gate |
| `modifiedInput` | PreToolUse | JSON object replacing the tool input before execution |
| `additionalContext` | UserPromptSubmit, PostToolUse | String appended to the engine context for this turn |

## Per-event policy table

| Event | Can block? | Can modify input? | Can inject context? |
|-------|-----------|-------------------|---------------------|
| PreToolUse | yes (exit 2) | yes | no |
| PostToolUse | no | no | yes |
| UserPromptSubmit | yes (exit 2) | no | yes |
| Stop | no | no | no |
| SubagentStop | no | no | no |
| Notification | no | no | no |
| SessionStart | no | no | no |
| SessionStop | no | no | no |
| PreCompact | no | no | no |

## Plan-mode hard-lock

Plan-mode (`/mode plan`) overrides hook-allow for write tools. Even if a `PreToolUse` hook returns `permissionDecision: "allow"`, plan mode denies Write/Edit/NotebookEdit/Task. Switch to default mode with `/mode default`.

## Example: audit hook

```bash
#!/usr/bin/env bash
# ~/.anthrogo/hooks/audit.sh
# Logs every Bash tool call with a timestamp.
set -euo pipefail
input=$(cat)
tool=$(echo "$input" | jq -r '.tool_name')
cmd=$(echo "$input" | jq -r '.tool_input.command // "n/a"')
echo "$(date -u +%FT%TZ) [$tool] $cmd" >> ~/.anthrogo/audit.log
```

## Example: protect-secrets hook

```bash
#!/usr/bin/env bash
# Deny any file write that contains a string matching "sk-ant-" or "AKIA"
set -euo pipefail
input=$(cat)
content=$(echo "$input" | jq -r '.tool_input.content // ""')
if echo "$content" | grep -qE '(sk-ant-|AKIA)'; then
  exit 2
fi
```

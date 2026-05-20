# anthrogo M4.1 — Hooks design

**Status:** approved
**Date:** 2026-05-20
**Author:** anthrogo project, single-maintainer
**Predecessors:** M3 (`docs/superpowers/specs/2026-05-17-anthrogo-m3-design.md`)

## 1. Goal

Port upstream `claude-code@2.1.88`'s hook framework to anthrogo. Users define shell commands in `settings.yaml` that fire at lifecycle events. Hooks can audit, block, mutate, or react to anthrogo's behavior.

## 2. Scope (M4.1)

All 9 upstream event points:

| Event | Fires at | Sync? | Can block? | Can mutate? |
|---|---|---|---|---|
| `PreToolUse` | Permission gate consultation | sync | yes (deny) | yes (allow / new input) |
| `PostToolUse` | After tool result, before next turn | sync | no | yes (replace text) |
| `UserPromptSubmit` | User pressed Enter in REPL / `-p` arg parsed | sync | yes (abort prompt) | yes (inject context) |
| `Stop` | One turn ends (assistant `end_turn`) | async | no | no |
| `SubagentStop` | Subagent finishes (M5 placeholder — stub only) | async | no | no |
| `Notification` | Permission ask modal raised | async | no | no |
| `PreCompact` | `/compact` triggered (M4.2 dep — wired when M4.2 lands) | sync | no | no |
| `SessionStart` | New / resumed session | async | no | no |
| `SessionEnd` | Process exit | async | no | no |

**Out of scope (deferred):** sandbox, signing/encryption, hook-call tracing in TUI, MCP-tool-aware matchers (matcher is plain Go regexp against the composed `mcp__server__tool` name — works but verbose).

## 3. Configuration

YAML stanza `hooks:` keyed by event name. Each entry is a list of hook specs.

```yaml
hooks:
  PreToolUse:
    - matcher: "Bash"                    # Go regexp, matched against tool_name
      command: ~/.anthrogo/hooks/bash-audit.sh
      timeout: 30s                        # default 30s
    - matcher: "Write|Edit|NotebookEdit"
      command: ~/.anthrogo/hooks/protect-secrets.sh
  PostToolUse:
    - matcher: "Write|Edit"
      command: ~/.anthrogo/hooks/gofmt.sh
  UserPromptSubmit:
    - command: ~/.anthrogo/hooks/inject-time.sh
  Stop:
    - command: ~/.anthrogo/hooks/notify-slack.sh
```

**Matcher semantics:**
- `PreToolUse` / `PostToolUse`: matcher is a Go regexp matched against the tool name. Empty / missing matcher = match all tools.
- All other events: matcher field is ignored (every hook of that type fires).

**Layered config:** `~/.anthrogo/settings.yaml` is the home base. If `<cwd>/.anthrogo/hooks.yaml` exists, its entries are appended (not replaced) to each event's hook list. Order: home first, then cwd. Hooks run in declaration order within an event.

**Path expansion:** `~` and `$VAR` are expanded at config-load time.

## 4. Data protocol (stdin / stdout / exit code)

Each fired hook is one short-lived subprocess. anthrogo writes one JSON object to the hook's stdin, then closes stdin. Hook responds with optional JSON on stdout, optional human text on stderr, and an exit code.

### 4.1 Input JSON (anthrogo → hook stdin)

Common envelope every event uses:

```json
{
  "hook_event_name": "PreToolUse",
  "session_id": "01HE...uuid",
  "cwd": "/Users/x/proj",
  "anthrogo_version": "0.4.0-dev"
}
```

Per-event additions:

```jsonc
// PreToolUse / PostToolUse
{
  "tool_name": "Bash",
  "tool_input": { "command": "rm -rf /" }
}
// PostToolUse only:
{
  "tool_response": { "text": "...", "is_error": false }
}

// UserPromptSubmit
{
  "prompt": "the user's raw input"
}

// Stop / SubagentStop
{
  "stop_reason": "end_turn"
}

// Notification
{
  "message": "human-readable notification text",
  "kind": "permission_ask" // or "error", "info"
}

// PreCompact
{
  "trigger": "manual" // or "auto"
}

// SessionStart
{
  "kind": "new" // or "resume"
}

// SessionEnd
{
  "kind": "user_quit" // or "signal"
}
```

### 4.2 Exit code semantics

| Code | Meaning | Behavior |
|---|---|---|
| `0` | OK | continue; parse stdout for optional JSON output (4.3) |
| `2` | Block | for events where blocking is meaningful (PreToolUse, UserPromptSubmit): deny / abort and surface stderr text to model + UI. For non-blocking events: treat as `0` but log stderr. |
| other non-zero | Error | log; behavior per event (see 4.4) |

### 4.3 Output JSON (hook stdout → anthrogo) — all fields optional

```jsonc
{
  "continue": true,                       // false = abort entire turn (rare)
  "stopReason": "...",                    // shown to user if continue=false
  "hookSpecificOutput": {
    "hookEventName": "PreToolUse",
    "permissionDecision": "allow",        // "allow" | "deny" — PreToolUse only. If a hook wants to surface a permission ask, return Pass (don't set permissionDecision); the existing alwaysAsk rule path handles it.
    "permissionDecisionReason": "matched safe-list",
    "modifiedInput": { "command": "ls" }, // PreToolUse only — replace tool_input
    "additionalContext": "..."            // UserPromptSubmit only — appended to prompt
  }
}
```

Anything malformed → fall back to exit-code semantics, log a warning.

### 4.4 Per-event failure / timeout policy

| Event | Timeout default | Timeout action | Non-zero exit (≠2) | Exit 2 |
|---|---|---|---|---|
| PreToolUse | 30s | kill, treat as deny | deny | deny |
| PostToolUse | 30s | kill, log+warn | log+warn | log+warn, append stderr to tool_result |
| UserPromptSubmit | 30s | kill, abort prompt | abort prompt | abort prompt, show stderr |
| Stop / SubagentStop | 10s | kill, log | log | log |
| Notification | 5s | kill, log | log | log |
| PreCompact | 30s | kill, log | log | log |
| SessionStart / SessionEnd | 5s | kill, log | log | log |

## 5. Execution model

### 5.1 Concurrency

- Within one event, hooks of the same event type run **sequentially in declaration order** (predictable for the user; later hooks see prior hook's mutations to `modifiedInput` / `additionalContext`).
- Across different events, hooks can overlap (e.g., a Stop hook from turn N can still be running when turn N+1's PreToolUse fires).
- Hook stderr is captured to anthrogo stderr in headless mode, and appended to the TUI chat as a dim line in TUI mode (similar to MCP log lines).

### 5.2 Sync vs async

Sync events (PreToolUse / PostToolUse / UserPromptSubmit / PreCompact) block the calling goroutine. Async events (Stop / SubagentStop / Notification / SessionStart / SessionEnd) are fire-and-forget: `Manager.FireAsync` returns immediately and the hook runs on a background goroutine.

### 5.3 Permission gate integration

Permission gate flow becomes:

1. `PreToolUse` hooks run sequentially.
2. If any hook returns `permissionDecision: "deny"` OR exits with code 2 → behavior = Deny (stderr is the reason).
3. If any hook returns `permissionDecision: "allow"` AND no later hook denies → behavior = Allow.
4. Plan-mode hard-lock is checked **after** hooks but **before** rule lookup: a `PreToolUse` allow CANNOT unlock plan-mode write tools. (Hooks are user intent; plan-mode is a deliberately stronger gate.)
5. Otherwise, fall through to existing rule lookup (alwaysDeny → mode → alwaysAllow → alwaysAsk → fallback).
6. `modifiedInput` from any hook replaces the tool input before the tool executes (last writer wins).

This makes hooks more powerful than YAML rules (they can mutate input) but weaker than plan-mode.

### 5.4 UserPromptSubmit integration

After the user presses Enter (TUI) or `-p` parses its arg (headless), every `UserPromptSubmit` hook runs:
1. Concatenate every hook's `additionalContext` (in order) into a single suffix.
2. If any hook returns exit code 2, abort: don't submit, show stderr.
3. Final prompt sent to the model = user input + "\n\n" + concatenated additionalContext.

### 5.5 PostToolUse integration

After each tool runs and before the result message is appended to the conversation:
1. Run hooks sequentially.
2. Any hook can write replacement text to stdout (top-level `additionalContext` field — extending 4.3 with one optional `additionalContext` for PostToolUse, semantics "append to tool_result").
3. Errors/timeouts log+warn only; the original tool result still goes through.

## 6. Code organization

```
internal/hooks/
├── config.go         # Config{} + per-hook spec + path expansion
├── event.go          # EventName const + per-event payload structs + JSON marshaling
├── runner.go         # runHook(ctx, spec, payload) — spawn / stdin / wait / timeout / parse
├── manager.go        # Manager{config} + Fire(ctx, event) (sync) + FireAsync(event)
├── decision.go       # combinePreToolUseDecisions(...) (returns *permissions.Decision + modified input)
├── *_test.go         # unit
├── manager_test.go   # e2e against testdata/hooks/*.sh
└── testdata/
    ├── allow.sh           # exits 0
    ├── deny.sh            # exits 2 with stderr
    ├── inject-context.sh  # exits 0 with JSON {additionalContext: "..."}
    ├── mutate-input.sh    # exits 0 with JSON {modifiedInput: {...}}
    ├── slow.sh            # sleeps longer than timeout
    └── crash.sh           # exits non-zero non-2
```

Hooked into existing surfaces (no new packages outside `internal/hooks`):
- `pkg/permissions/gate.go`: `Decide()` calls `hooks.Manager.FirePreToolUse` first (via a function injected on `Context` to avoid the import cycle).
- `pkg/query/loop.go`: after each tool result, calls `hooks.Manager.FirePostToolUse`; on turn end, `FireStop`.
- `internal/tui/app.go`: prompt submit → `FireUserPromptSubmit`; permission modal raised → `FireNotification`.
- `internal/headless/runner.go`: prompt parse → `FireUserPromptSubmit`; turn end → `FireStop`.
- `cmd/anthrogo/main.go`: `SessionStart` immediately after session init; `SessionEnd` via `defer`.
- `/compact` (M4.2): `FirePreCompact` before compaction. M4.1 only declares the event constant; FirePreCompact is a no-op stub until M4.2 wires it.

## 7. Errors / edge cases

| Case | Handling |
|---|---|
| Hook binary not found | Log "hook X: command not found"; treat as exit-code 127 per event policy |
| Hook is a script without exec bit | Same as above |
| Hook stdin closed by hook before reading | Kernel handles; we don't block on write |
| Hook produces > 1 MB on stdout | Truncate; warn |
| Hook produces > 1 MB on stderr | Truncate; warn |
| Hook produces non-UTF-8 | Pass through bytes |
| User Ctrl-C during sync hook | Cancel ctx → kill process → propagate ctx.Err() up |
| Hook config has invalid regexp | Log at load; skip that hook |
| Empty `hooks:` block | OK |

## 8. Testing strategy

- **Unit:** runner.go for each exit-code path, timeout, malformed JSON, stdin write.
- **Unit:** manager.go FirePreToolUse decision combining (allow + deny + modify cases), matcher regexp filtering, sequential ordering.
- **e2e:** Compile testdata shell scripts (no compile — they're chmod +x scripts), spin up a Manager with real subprocess, verify each scripted scenario.
- **Race:** `go test -race` over `internal/hooks/`, `pkg/permissions/`, `pkg/query/`.
- **Integration:** one Engine-level test that runs a full turn with PreToolUse-deny + PostToolUse-log + UserPromptSubmit-inject.

## 9. Bookkeeping debt addressed in M4.1

- `internal/tui/chat_test.go`: add concurrent-AppendServerLog → tea.Program.Send race regression test (review fix #2 had no dedicated test).
- `internal/mcp/server_test.go`: add `Server.Start` state-reset regression test (covers `/mcp reload` re-Start path).

## 10. CHANGELOG / version

- Bump to `0.4.0-dev`.
- Prepend `[0.4.0-dev] — 2026-05-20` section in `CHANGELOG.md`.
- README: new "Hooks" section between "MCP servers" and "Tools".

## 11. Acceptance

- `go build ./...`, `go vet ./...`, `go test ./...` clean
- `go test -race` on `pkg/query` `pkg/tool` `internal/tui` `internal/session` `pkg/command` `internal/mcp` `internal/system` `internal/hooks` `pkg/permissions` clean
- 3× uncached full-repo runs pass (M3 flake history)
- `./bin/anthrogo --version` prints `anthrogo 0.4.0-dev`
- An optional live smoke: configure one PreToolUse hook that denies `rm -rf*` and exercise via headless.

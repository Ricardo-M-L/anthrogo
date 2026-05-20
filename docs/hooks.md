# Hooks

See [README — Hooks](https://github.com/Ricardo-M-L/anthrogo#hooks).

anthrogo supports 9 hook events that fire at well-defined points in the agent lifecycle. Hooks are configured in `settings.yaml` under the `hooks:` key.

## Hook events

| Event | When it fires |
|---|---|
| `PreToolUse` | Before any tool is executed |
| `PostToolUse` | After a tool returns |
| `PreQuery` | Before sending a request to the model |
| `PostQuery` | After the model responds |
| `SessionStart` | On session initialization |
| `SessionStop` | On session teardown |
| `SubagentStart` | Before spawning a subagent |
| `SubagentStop` | After a subagent completes |
| `CompactionTrigger` | When auto-compaction fires |

(M9.6 deferred: full hook reference page; planned for M11.4 follow-up.)

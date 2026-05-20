# Subagents

See [README — Subagents](https://github.com/Ricardo-M-L/anthrogo#subagents).

anthrogo can spawn concurrent subagents to work on independent tasks in parallel, or dispatch remote tasks via KAIROS.

## Concurrent subagents

Use the `/subagent` slash command or the `Dispatch` tool to fan out work:

```
/subagent "Summarize file A" "Summarize file B" "Summarize file C"
```

All subagents share a read-only snapshot of the current context but write to isolated scratch spaces. Results are merged when all complete.

## KAIROS remote dispatch

See [KAIROS](kairos.md) for remote agent dispatch across machines.

(Full subagent reference migrating from README — M11.4 follow-up.)

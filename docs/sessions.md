# Sessions

anthrogo persists every conversation as a JSONL file keyed by working directory and session UUID. Use `/sessions` subcommands to inspect, search, replay, export, and manage historical sessions.

## Session storage

Files live at:

```
~/.anthrogo/projects/<cwd-hash>/<session-id>.jsonl
~/.anthrogo/projects/<cwd-hash>/<session-id>/subagents/<subagent-id>.jsonl
```

A two-level search cache accelerates repeated searches:
- **L1** — in-memory LRU (64 slots keyed by `path + modtime`)
- **L2** — SQLite at `~/.anthrogo/search_index.db` (pure-Go, no cgo); survives process restarts

To fully reset persistence: `rm ~/.anthrogo/search_index.db` and restart.

## /sessions subcommands

### list

```
/sessions
/sessions list
```

Shows all sessions for the current working directory, sorted newest-first:

```
ID                                      Modified          Size
550e8400-e29b-41d4-a716-446655440000    2026-05-20 14:32  18423 B
3f2504e0-4f89-11d3-9a0c-0305e82c3301    2026-05-19 09:11  4201 B
```

### show

```
/sessions show <id-prefix>
```

Prints a metadata summary for the matched session (unambiguous prefix match).

### replay

```
/sessions replay <id-prefix>
```

Renders the matched session as a one-line-per-record timeline. Every record kind is covered: `meta`, `user`, `asst`, `tool`, `result`, `compact`, `subagent`, `usage`, `turn-end`, `error`. Text is truncated and newlines collapsed for readability.

### search

```
/sessions search <keyword>
/sessions search <keyword> --regex
/sessions search <keyword> --recurse-subagents
/sessions search <keyword> --since 2026-05-01
/sessions search <keyword> --until 2026-05-21
```

Case-insensitive substring search across all session JSONLs for the current cwd. Each match shows `<session-id> [<kind>] <context>` (40 chars before + match + 40 chars after). Results are capped at 200 matches.

Flags:
- `--regex` — interprets keyword as a Go regexp (invalid patterns return an error)
- `--recurse-subagents` — also scans `<session-id>/subagents/*.jsonl` (prefixed `<parent>/subagents/<sub>`)
- `--since YYYY-MM-DD`, `--until YYYY-MM-DD` — filter records by timestamp

### delete

```
/sessions delete <id-prefix>
/sessions delete <id-prefix> --yes
```

Without `--yes` performs a dry-run: prints the JSONL path, size, subagents directory (if any) with file count and total bytes, and the exact command to confirm real deletion. Add `--yes` to remove both the JSONL and the `<session-id>/subagents/` tree. Irreversible — no undo.

### export

```
/sessions export <id-prefix>
/sessions export <id-prefix> -o file.md
```

Renders the session as a Markdown document. Without `-o`, output goes to stdout. With `-o <file.md>`, writes the file and reports `exported <path> (<N> bytes)`.

### stats

```
/sessions stats
/sessions stats --since 2026-05-01
/sessions stats --until 2026-05-21
```

Aggregates metrics across all session JSONLs for the current cwd:
- Session count, turn count
- Total input/output tokens and estimated USD cost (built-in pricing table)
- First-seen and latest timestamps
- Per-model token and cost breakdown
- Per-day turn count table

### diff

```
/sessions diff <id1-prefix> <id2-prefix>
```

Compares two sessions side-by-side. Each session is flattened to one line per turn event (user/assistant/tool/result/compact) and a unified diff is rendered using LCS dynamic programming. Lines prefixed `  ` are common; `+ ` in second only; `- ` in first only. Text trimmed to 200 chars per line (100 chars for tool results).

### reindex

```
/sessions reindex
```

Alias: `search-rebuild-index`. Clears the in-memory LRU parse cache (L1). The cache holds up to 64 parsed session files keyed by `(path, modtime)`. Unchanged files are served from cache on repeated searches. Modtime changes auto-invalidate; `reindex` forces a full rebuild on the next search.

## /audit

```
/audit
/audit list [N]
/audit by-tool <name>
/audit errors
/audit search <keyword>
```

Scans all session JSONLs for the current cwd and surfaces tool calls, errors, compact events, and subagent starts — newest first. N defaults to 50. Each row: `<ts>  [<short-session-id>]  <kind:tool>  <summary>`.

| Subcommand | Description |
|------------|-------------|
| `list [N]` | Most-recent N audit events (default 50) |
| `by-tool <name>` | Filter to a specific tool (e.g. `by-tool Bash`) |
| `errors` | Show only records where `IsError=true` |
| `search <keyword>` | Case-insensitive match against tool name + input summary |

Note: permission decisions (allow/deny/ask) are not recorded in the JSONL and are not visible in `/audit`.

## JSONL record schema

Each line is a JSON object. Common fields:

```json
{"kind": "meta",      "session_id": "...", "cwd": "/path", "ts": "2026-05-21T10:00:00Z"}
{"kind": "user",      "ts": "...", "text": "explain main.go"}
{"kind": "asst",      "ts": "...", "text": "..."}
{"kind": "tool",      "ts": "...", "tool_name": "Read", "input": {...}}
{"kind": "result",    "ts": "...", "tool_name": "Read", "output": "...", "is_error": false}
{"kind": "compact",   "ts": "...", "summary": "...", "kept": 10}
{"kind": "usage",     "ts": "...", "input_tokens": 1234, "output_tokens": 56}
{"kind": "turn-end",  "ts": "..."}
{"kind": "subagent_start", "ts": "...", "subagent_id": "...", "description": "..."}
{"kind": "error",     "ts": "...", "message": "..."}
```

## Search index architecture

The search subsystem uses a two-level cache:

- **L1 (in-memory LRU)**: holds up to 64 parsed session file objects keyed by `(absolute-path, modtime)`. Hot sessions (repeated searches) avoid re-parsing.
- **L2 (SQLite)**: `~/.anthrogo/search_index.db` stores parsed record sets with their modtime. Cache misses at L1 are promoted from L2 before falling back to disk parse. Writes go to both levels.

The DB degrades gracefully to L1-only if it cannot be opened (read-only filesystem, permissions error, etc.).

# Compaction

For long sessions, `/compact` summarizes earlier turns to cut token cost while preserving the working context.

## Manual compaction

```
/compact            # keeps the 10 most-recent messages, summarizes the rest
/compact --keep 20  # keep 20 most-recent messages
```

Currently all earlier messages including MCP tool calls are summarized to prose. Pair-preserving compaction is a future milestone.

`PreCompact` hooks (configured under `hooks.PreCompact`) fire synchronously before each compact.

## Token counting and reporting

On completion, `/compact` reports actual token counts:

```
compacted 15 → 11 messages (~820 → ~210 tokens)
```

Token counting uses:
- **OpenAI-family models** — real BPE tokenizer via tiktoken-go
- **Claude and others** — char/4 approximation

Image tokens are not counted client-side; the provider's `EventUsage` is authoritative for image cost.

## Auto-compaction

Set `auto_compact_threshold` to automatically run `/compact` when cumulative token usage since the last compact exceeds the threshold:

```yaml
auto_compact_threshold: 150000   # tokens; 0 = disabled (default)
```

Or pass it on the command line:

```bash
anthrogo --auto-compact 150000
```

The threshold is checked at the end of every turn using the `usageSinceLastCompact` counter — not just the latest turn's usage — so sessions with many small turns are handled correctly. After a successful compact the counter resets to zero. Manual `/compact` also resets the counter.

## /usage

Inspect the current state at any time:

```
/usage
Session totals: 1,240 input + 380 output = 1,620 tokens
Since last compact: 420 input + 130 output = 550 tokens
Auto-compact at: 150,000 tokens (keep recent: 10) — 149,450 tokens until trigger
```

The TUI status line shows `tok: <in>in/<out>out (since: <Z>) [⚙ <N>]` where `since` is the post-compact accumulation and `⚙ N` is the auto-compact threshold (omitted when disabled). Live 1 Hz status refresh during turns.

## Budget reset on compact

To zero the cost counter when compacting (e.g. to start fresh against a budget cap):

```
/compact --reset-budget
```

This resets the in-memory usage counter so the post-compact session starts fresh. The budget cap remains armed; usage accumulates again from zero.

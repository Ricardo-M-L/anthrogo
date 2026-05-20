# anthrogo M4.2 — Real `/compact` (MCP-aware history compaction)

**Status:** approved (self-authorized per user grant)
**Date:** 2026-05-20
**Predecessor:** M4.1 (`docs/superpowers/specs/2026-05-20-anthrogo-m4-1-hooks-design.md`)

## 1. Goal

Replace the placeholder `/compact` slash command with a real LLM-summarization-driven history compaction. Wires up the `PreCompact` hook event that M4.1 left dormant. Reduces token cost on long sessions without losing tool-call lineage or active plan-mode state.

## 2. Scope

- `pkg/compact/` package: pure summarization logic, provider-agnostic via existing `provider.Provider` interface.
- `Engine.Compact(ctx, opts) (Summary, error)` method on `pkg/query.Engine`: orchestrates hook → LLM call → message replacement.
- `/compact` slash command becomes real (calls `host.Engine().Compact(...)`).
- Session JSONL records a new event type `compact` with before/after counts + summary text.
- TUI prints a one-line confirmation `compacted N → M messages, ~X → ~Y tokens`.

**Out of scope:** auto-trigger on token threshold (manual `/compact` only); precise token counting (estimate via byte count; real tokenizer in M6); preserving thinking blocks (drop them — they're context for one turn, not for the next).

## 3. Compaction algorithm

Given current `messages []message.Message` and `keepRecent N` (default 10):

1. Split: `head = messages[:len-N]`, `tail = messages[len-N:]`. If `len(messages) <= N`, no-op return.
2. Build summary prompt (see §4) carrying `head` only.
3. Call provider with `Stream(ctx, Request{Model, SystemPrompt: summarySystem, Messages: [oneSyntheticUserMsg], MaxTokens: 4096})`. Collect all `text_delta` events.
4. Construct `summary message.Message` with role=user, single text block containing:
   `"[Compacted earlier conversation (" + headCount + " messages)]\n\n" + summaryText`.
5. **MCP-aware preservation: deferred to a later milestone.** Until then, every head message including MCP tool_use blocks is summarized; the model loses direct visibility into earlier MCP tool calls but the summary should mention them.
6. New messages = `[summary_user] + tail`.
7. Replace via `engine.SetInitialMessages(new)`.
8. Compute approximate before/after byte counts; record `Summary{OriginalCount, NewCount, OriginalBytes, NewBytes, SummaryText}`.

**Plan-mode handling:** plan mode is a `Permissions.Mode` flag, not part of `messages`. Compaction leaves the flag untouched. The plan-mode addendum already lives in the system prompt, which we don't touch.

## 4. Summary prompt

System prompt (embedded in `pkg/compact/prompt.go`):

```
You are summarizing an in-progress conversation between a user and an AI coding assistant.

Produce a dense, faithful summary that:
- Preserves all factual claims the assistant made about the codebase (file paths, function names, decisions taken).
- Preserves outstanding questions or in-progress tasks.
- Drops chit-chat, retracted statements, and verbose tool output.
- Does NOT invent new information.

Output ONLY the summary as plain prose. No preamble, no markdown headings, no apologies.
```

User-role message constructed by the compactor:

```
Conversation to summarize (JSON, oldest first):

<JSON-encoded array of head messages>
```

The assistant's first text response is captured verbatim as the summary.

## 5. Integration points

### 5.1 `pkg/query.Engine.Compact`

```go
type CompactOptions struct {
    KeepRecent int            // default 10
    Trigger    string         // "manual" | "auto" (manual in M4.2)
}

type Summary struct {
    OriginalCount  int
    NewCount       int
    OriginalBytes  int
    NewBytes       int
    SummaryText    string
    Skipped        bool   // true if len(messages) <= KeepRecent
    SkipReason     string
}

func (e *Engine) Compact(ctx context.Context, opts CompactOptions) (Summary, error)
```

Flow:
1. If `e.cfg.Hooks != nil { e.cfg.Hooks.FirePreCompact(ctx, opts.Trigger) }`. Sync; logs only per M4.1 design.
2. Call `compact.Run(ctx, compact.Input{Provider: e.cfg.Provider, Model: e.cfg.Model, Messages: e.Messages(), KeepRecent: opts.KeepRecent})` — pure function.
3. On error, return error; messages untouched.
4. On Skipped=true, return Summary with Skipped=true; don't replace.
5. Otherwise `e.SetInitialMessages(result.NewMessages)`; emit `KindCompact` event over `e.streamCh` (new event kind in `pkg/query.EventKind`) so TUI can render.
6. Append a `session_compact` JSONL record via the existing RecordHook (new record type).

Add `FirePreCompact` to `query.HookSink` interface (currently only PostToolUse + Stop). `*hooks.Manager` already has the method.

### 5.2 `pkg/compact.Run`

Pure: takes `Input{Provider, Model, Messages, KeepRecent}`, returns `Output{NewMessages, SummaryText, OriginalCount, ...}`. No hook calls; no engine reference. Easy to unit-test with `pkg/provider/fake`.

### 5.3 `/compact` slash command

Replace placeholder `pkg/command/builtins/compact.go` body. Use `host.Engine().Compact(ctx, CompactOptions{Trigger:"manual"})`. Format result string. Use `args` to parse `--keep N` if user passes one.

### 5.4 Session record

`internal/session/record.go` (new record kind):

```go
type Record struct {
    Kind string
    /* existing fields */
    Compact *CompactRecord // when Kind == "compact"
}
type CompactRecord struct {
    OriginalCount int
    NewCount      int
    OriginalBytes int
    NewBytes      int
    Trigger       string
}
```

Replay logic: a `compact` record means "drop all prior message records from replay; replay the summary message that was the first message after compact". For M4.2 we keep this simple: when replaying and we encounter a `compact` record, throw away the accumulated messages and continue replay from the next record (which is the synthesized summary user-message, written as a normal message record right after the `compact` marker). This means `--resume` after a compacted session resumes from the compact point, not the original.

### 5.5 TUI event handling

New `query.EventKind` constant `EventCompact` with payload `Summary`. TUI's existing `handleEvent` adds:

```go
case query.EventCompact:
    a.chat.appendUser(fmt.Sprintf("compacted %d → %d messages (~%d → ~%d bytes)",
        ev.Compact.OriginalCount, ev.Compact.NewCount,
        ev.Compact.OriginalBytes, ev.Compact.NewBytes))
```

(Use `appendUser` style so it's visually distinct as a system-style chat entry.)

## 6. Error handling

| Case | Action |
|---|---|
| Provider stream returns error | Return error; messages untouched |
| Stream ends with zero text | Return error "compact: empty summary"; messages untouched |
| ctx cancelled mid-stream | Return ctx.Err() |
| `len(messages) <= KeepRecent` | Return `Summary{Skipped: true, SkipReason: "fewer than KeepRecent messages"}` |
| PreCompact hook fails | Log via Manager.LogSink; continue (per M4.1 §5.2 semantics) |
| Session record write fails | Log to stderr; continue (compact already happened in-memory) |
| Summary message contains MCP tool_use blocks ourselves | Cannot happen — we only ask for text output |

## 7. Testing

- `pkg/compact/compact_test.go`:
  - `TestCompact_NoOpWhenShort` — 5 messages, keepRecent=10 → Skipped=true
  - `TestCompact_SummarizesHead` — 15 messages, keepRecent=10 → NewCount=11 (1 summary + 10 tail), summary text matches fake provider output
  - `TestCompact_PreservesMCPToolUses` — head has 5 messages including 2 with mcp__-prefixed tool_use → NewCount = 2 preserved + 1 summary + tail
  - `TestCompact_ProviderError_PropagatesUntouched` — fake provider that emits error → Run returns error, no NewMessages
- `pkg/query/engine_test.go`:
  - `TestEngine_Compact_FiresHookAndReplaces` — wire hookMgr with a PreCompact spec that increments a counter; assert Compact runs the spec, replaces messages, emits EventCompact
- `pkg/command/builtins/compact_test.go`:
  - `TestCompactBuiltin_Skipped_ReturnsSkipMessage`
  - `TestCompactBuiltin_Success_ReturnsCountSummary`

`go test -race -count=2 ./pkg/compact ./pkg/query ./pkg/command/builtins ./internal/hooks` clean.

## 8. Code organization

```
pkg/compact/
├── compact.go        # Run(ctx, Input) (Output, error)
├── prompt.go         # embedded summarySystem + buildUserMessage helper
├── tokens.go         # approximate byte count (real tokens M6)
├── compact_test.go
```

Touched:
- `pkg/query/engine.go` — Compact method + Summary type + EventCompact kind
- `pkg/query/HookSink` — add FirePreCompact to interface
- `pkg/command/builtins/compact.go` — real impl
- `internal/session/record.go` — CompactRecord kind + replay handling
- `internal/tui/app.go` — handle EventCompact

## 9. Acceptance

- `go build/vet/test/-race` clean
- 3× uncached full-repo sweep clean
- `./bin/anthrogo --version` → `0.4.1-dev`
- README: short note in Hooks section that PreCompact now fires on `/compact`
- CHANGELOG entry under `[0.4.1-dev] — 2026-05-20`

## 10. Deferred to M4.3+

- Auto-compact on token threshold (M4 future)
- Real tokenizer (M6 alongside provider plurality)
- Compaction of subagent message trees (M5)
- Resume across multiple compacts (currently we replay from latest compact only — older compacts in the JSONL are "lost" on resume; acceptable for M4.2)

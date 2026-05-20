# M4.2 Real `/compact` Implementation Plan

> Execute via superpowers:subagent-driven-development. Checkbox tasks.

**Goal:** Replace placeholder `/compact` with LLM-summarized history compaction. MCP-tool-use messages preserved verbatim. PreCompact hook fires. `pkg/query.Engine` gains `Compact(ctx, opts) (Summary, error)`.

**Architecture:** New pure `pkg/compact/` package (no engine deps). `Engine.Compact` orchestrates: PreCompact hook → `compact.Run` → swap messages → emit EventCompact. Session JSONL gets a `compact` record kind.

---

## Task 1: `pkg/compact/` package

**Files:**
- Create: `pkg/compact/compact.go`
- Create: `pkg/compact/prompt.go`
- Create: `pkg/compact/tokens.go`
- Create: `pkg/compact/compact_test.go`

- [ ] **Step 1.1: tokens.go**

```go
package compact

import (
    "encoding/json"

    "github.com/ricardo/anthrogo/pkg/message"
)

// ApproxBytes is a stand-in for token counting. Returns the sum of JSON-
// marshaled byte lengths across all messages. Good enough for M4.2; a real
// tokenizer arrives with M6 (multi-provider).
func ApproxBytes(msgs []message.Message) int {
    total := 0
    for i := range msgs {
        raw, err := json.Marshal(msgs[i])
        if err != nil {
            continue
        }
        total += len(raw)
    }
    return total
}
```

- [ ] **Step 1.2: prompt.go**

```go
package compact

import (
    "encoding/json"

    "github.com/ricardo/anthrogo/pkg/message"
)

const SummarySystemPrompt = `You are summarizing an in-progress conversation between a user and an AI coding assistant.

Produce a dense, faithful summary that:
- Preserves all factual claims the assistant made about the codebase (file paths, function names, decisions taken).
- Preserves outstanding questions or in-progress tasks.
- Drops chit-chat, retracted statements, and verbose tool output.
- Does NOT invent new information.

Output ONLY the summary as plain prose. No preamble, no markdown headings, no apologies.`

// buildSummaryUserMessage constructs the single synthetic user message we send
// to the LLM along with the head messages JSON-serialized inside it.
func buildSummaryUserMessage(head []message.Message) message.Message {
    raw, _ := json.Marshal(head)
    text := "Conversation to summarize (JSON, oldest first):\n\n" + string(raw)
    return message.Message{
        Role:    message.RoleUser,
        Content: []message.Block{{Type: message.BlockText, Text: text}},
    }
}
```

- [ ] **Step 1.3: compact.go**

```go
package compact

import (
    "context"
    "fmt"
    "strings"

    "github.com/ricardo/anthrogo/pkg/message"
    "github.com/ricardo/anthrogo/pkg/provider"
)

// Input is the pure input to Run. No engine, no hooks.
type Input struct {
    Provider   provider.Provider
    Model      string
    Messages   []message.Message
    KeepRecent int // default 10
    MaxTokens  int // default 4096 for the summary call
}

// Output is what Run returns. NewMessages is empty if Skipped.
type Output struct {
    NewMessages   []message.Message
    OriginalCount int
    NewCount      int
    OriginalBytes int
    NewBytes      int
    SummaryText   string
    Skipped       bool
    SkipReason    string
}

// Run produces a compacted message list. Pure (modulo provider call).
func Run(ctx context.Context, in Input) (Output, error) {
    if in.KeepRecent <= 0 {
        in.KeepRecent = 10
    }
    if in.MaxTokens <= 0 {
        in.MaxTokens = 4096
    }
    msgs := in.Messages
    out := Output{
        OriginalCount: len(msgs),
        OriginalBytes: ApproxBytes(msgs),
    }
    if len(msgs) <= in.KeepRecent {
        out.Skipped = true
        out.SkipReason = "fewer than KeepRecent messages"
        return out, nil
    }
    split := len(msgs) - in.KeepRecent
    head := msgs[:split]
    tail := msgs[split:]

    // MCP-aware: pull out messages whose content contains a tool_use block
    // whose Name starts with "mcp__". They are preserved verbatim.
    var mcpPreserved []message.Message
    var summarizable []message.Message
    for _, m := range head {
        if hasMCPToolUse(m) {
            mcpPreserved = append(mcpPreserved, m)
        } else {
            summarizable = append(summarizable, m)
        }
    }

    summary, err := streamSummary(ctx, in.Provider, in.Model, in.MaxTokens, summarizable)
    if err != nil {
        return Output{}, err
    }
    if strings.TrimSpace(summary) == "" {
        return Output{}, fmt.Errorf("compact: empty summary")
    }
    out.SummaryText = summary

    summaryMsg := message.Message{
        Role: message.RoleUser,
        Content: []message.Block{{
            Type: message.BlockText,
            Text: fmt.Sprintf("[Compacted earlier conversation (%d messages)]\n\n%s", len(summarizable), summary),
        }},
    }

    newMsgs := make([]message.Message, 0, len(mcpPreserved)+1+len(tail))
    newMsgs = append(newMsgs, mcpPreserved...)
    newMsgs = append(newMsgs, summaryMsg)
    newMsgs = append(newMsgs, tail...)

    out.NewMessages = newMsgs
    out.NewCount = len(newMsgs)
    out.NewBytes = ApproxBytes(newMsgs)
    return out, nil
}

func hasMCPToolUse(m message.Message) bool {
    for _, b := range m.Content {
        if b.Type == message.BlockToolUse && strings.HasPrefix(b.Name, "mcp__") {
            return true
        }
    }
    return false
}

func streamSummary(ctx context.Context, p provider.Provider, model string, maxTokens int, head []message.Message) (string, error) {
    req := provider.Request{
        Model:        model,
        SystemPrompt: SummarySystemPrompt,
        Messages:     []message.Message{buildSummaryUserMessage(head)},
        MaxTokens:    maxTokens,
    }
    ch, err := p.Stream(ctx, req)
    if err != nil {
        return "", err
    }
    var b strings.Builder
    for ev := range ch {
        switch ev.Kind {
        case provider.EventTextDelta:
            b.WriteString(ev.Text)
        case provider.EventError:
            return "", ev.Err
        }
    }
    return b.String(), nil
}
```

Spot-check `message.BlockToolUse` constant exists by grepping `grep -n "BlockToolUse\|BlockText\|RoleUser" pkg/message/`. If the name differs (e.g. `BlockTool`), adapt.

- [ ] **Step 1.4: compact_test.go**

```go
package compact

import (
    "context"
    "errors"
    "strings"
    "testing"

    "github.com/stretchr/testify/require"

    "github.com/ricardo/anthrogo/pkg/message"
    "github.com/ricardo/anthrogo/pkg/provider"
    "github.com/ricardo/anthrogo/pkg/provider/fake"
)

func textMsg(role message.Role, text string) message.Message {
    return message.Message{Role: role, Content: []message.Block{{Type: message.BlockText, Text: text}}}
}

func mcpToolUseMsg(name string) message.Message {
    return message.Message{
        Role: message.RoleAssistant,
        Content: []message.Block{
            {Type: message.BlockToolUse, ID: "t1", Name: name, Input: map[string]any{"x": 1}},
        },
    }
}

// summaryProvider returns a fake provider that emits one text_delta with the
// configured summary and then EventMessageStop.
func summaryProvider(summary string) provider.Provider {
    return fake.NewScripted([][]fake.Step{
        {{Kind: provider.EventStart}, {Kind: provider.EventTextDelta, Text: summary}, {Kind: provider.EventMessageStop}},
    })
}

func TestCompact_NoOpWhenShort(t *testing.T) {
    msgs := []message.Message{textMsg(message.RoleUser, "hi"), textMsg(message.RoleAssistant, "hello")}
    out, err := Run(context.Background(), Input{
        Provider:   summaryProvider("unused"),
        Model:      "m",
        Messages:   msgs,
        KeepRecent: 10,
    })
    require.NoError(t, err)
    require.True(t, out.Skipped)
    require.Equal(t, 2, out.OriginalCount)
    require.Empty(t, out.NewMessages)
}

func TestCompact_SummarizesHead(t *testing.T) {
    msgs := make([]message.Message, 15)
    for i := range msgs {
        if i%2 == 0 {
            msgs[i] = textMsg(message.RoleUser, "u"+itoa(i))
        } else {
            msgs[i] = textMsg(message.RoleAssistant, "a"+itoa(i))
        }
    }
    out, err := Run(context.Background(), Input{
        Provider:   summaryProvider("SUMMARY_TEXT"),
        Model:      "m",
        Messages:   msgs,
        KeepRecent: 10,
    })
    require.NoError(t, err)
    require.False(t, out.Skipped)
    require.Equal(t, 15, out.OriginalCount)
    require.Equal(t, 11, out.NewCount) // 1 summary + 10 tail
    require.Equal(t, "SUMMARY_TEXT", out.SummaryText)
    require.Contains(t, out.NewMessages[0].Content[0].Text, "SUMMARY_TEXT")
    require.Contains(t, out.NewMessages[0].Content[0].Text, "Compacted")
}

func TestCompact_PreservesMCPToolUses(t *testing.T) {
    msgs := []message.Message{
        textMsg(message.RoleUser, "old1"),
        textMsg(message.RoleAssistant, "old2"),
        mcpToolUseMsg("mcp__fs__read_file"),
        textMsg(message.RoleUser, "old3"),
        textMsg(message.RoleAssistant, "old4"),
        // 10 tail messages:
        textMsg(message.RoleUser, "t1"),
        textMsg(message.RoleAssistant, "t2"),
        textMsg(message.RoleUser, "t3"),
        textMsg(message.RoleAssistant, "t4"),
        textMsg(message.RoleUser, "t5"),
        textMsg(message.RoleAssistant, "t6"),
        textMsg(message.RoleUser, "t7"),
        textMsg(message.RoleAssistant, "t8"),
        textMsg(message.RoleUser, "t9"),
        textMsg(message.RoleAssistant, "t10"),
    }
    out, err := Run(context.Background(), Input{
        Provider:   summaryProvider("S"),
        Model:      "m",
        Messages:   msgs,
        KeepRecent: 10,
    })
    require.NoError(t, err)
    require.Equal(t, 12, out.NewCount) // 1 mcp preserved + 1 summary + 10 tail
    // Position 0 is the preserved MCP message
    require.Equal(t, message.BlockToolUse, out.NewMessages[0].Content[0].Type)
    require.Equal(t, "mcp__fs__read_file", out.NewMessages[0].Content[0].Name)
    // Position 1 is the summary
    require.Contains(t, out.NewMessages[1].Content[0].Text, "S")
}

func TestCompact_ProviderError_PropagatesUntouched(t *testing.T) {
    fp := fake.NewScripted([][]fake.Step{
        {{Kind: provider.EventError, Err: errors.New("upstream down")}},
    })
    msgs := make([]message.Message, 15)
    for i := range msgs {
        msgs[i] = textMsg(message.RoleUser, "x")
    }
    _, err := Run(context.Background(), Input{Provider: fp, Model: "m", Messages: msgs, KeepRecent: 10})
    require.Error(t, err)
    require.True(t, strings.Contains(err.Error(), "upstream down"))
}

// itoa avoids importing strconv just for tests.
func itoa(i int) string {
    if i == 0 {
        return "0"
    }
    var out []byte
    for i > 0 {
        out = append([]byte{byte('0' + i%10)}, out...)
        i /= 10
    }
    return string(out)
}
```

**Spot-check `fake.NewScripted` signature:** `grep -n "func NewScripted\|func New\|type Step\|type Scripted" pkg/provider/fake/`. If the fake's API is different (e.g. takes a single slice instead of slice-of-slices, or uses a builder), adapt the helper accordingly. Use whatever the existing fake provides; do not modify the fake package.

- [ ] **Step 1.5: Run + stage**

```bash
go test ./pkg/compact/... -count=1
git add pkg/compact/
```

Expected: 4 PASS.

---

## Task 2: `Engine.Compact` + EventCompact + HookSink extension

**Files:**
- Modify: `pkg/query/engine.go`
- Modify: `pkg/query/loop.go` (if EventKind enum lives there)
- Modify: `pkg/query/engine_test.go`

- [ ] **Step 2.1: Extend HookSink interface**

In `pkg/query/engine.go` (or wherever `HookSink` is), add:

```go
FirePreCompact(ctx context.Context, trigger string)
```

`*hooks.Manager` already has this method.

- [ ] **Step 2.2: New EventKind + Event field**

In `pkg/query/`'s Event definitions (look for `type Event` / `EventKind`), add:

```go
EventCompact EventKind = "compact"
```

Extend `Event` struct with `Compact *Summary`. Define `Summary` (re-export `compact.Output` or wrap):

```go
type Summary struct {
    OriginalCount int
    NewCount      int
    OriginalBytes int
    NewBytes      int
    SummaryText   string
    Skipped       bool
    SkipReason    string
    Trigger       string
}
```

- [ ] **Step 2.3: Engine.Compact method**

In `engine.go`:

```go
type CompactOptions struct {
    KeepRecent int    // default 10
    Trigger    string // "manual"|"auto"
}

func (e *Engine) Compact(ctx context.Context, opts CompactOptions) (Summary, error) {
    if opts.Trigger == "" {
        opts.Trigger = "manual"
    }
    if e.cfg.Hooks != nil {
        e.cfg.Hooks.FirePreCompact(ctx, opts.Trigger)
    }
    in := compact.Input{
        Provider:   e.cfg.Provider,
        Model:      e.cfg.Model,
        Messages:   e.Messages(),
        KeepRecent: opts.KeepRecent,
    }
    out, err := compact.Run(ctx, in)
    if err != nil {
        return Summary{}, err
    }
    s := Summary{
        OriginalCount: out.OriginalCount, NewCount: out.NewCount,
        OriginalBytes: out.OriginalBytes, NewBytes: out.NewBytes,
        SummaryText: out.SummaryText, Skipped: out.Skipped, SkipReason: out.SkipReason,
        Trigger: opts.Trigger,
    }
    if !out.Skipped {
        e.SetInitialMessages(out.NewMessages)
        if e.cfg.RecordHook != nil {
            e.cfg.RecordHook(session.Record{Kind: "compact", Compact: &session.CompactRecord{
                OriginalCount: s.OriginalCount, NewCount: s.NewCount,
                OriginalBytes: s.OriginalBytes, NewBytes: s.NewBytes,
                Trigger: s.Trigger,
            }})
            // Also record the new summary message so resume sees it
            if len(out.NewMessages) > 0 {
                if mcp := out.NewMessages[0]; mcp.Role == message.RoleAssistant && hasMCPInMessage(mcp) {
                    // skip recording the preserved MCP ones — they were already in the JSONL
                }
                // Record the summary message verbatim:
                summaryIdx := indexOfFirstSummary(out.NewMessages)
                if summaryIdx >= 0 {
                    e.cfg.RecordHook(session.Record{Kind: "message", Message: &out.NewMessages[summaryIdx]})
                }
            }
        }
    }
    return s, nil
}
```

Two helper funcs `hasMCPInMessage` / `indexOfFirstSummary`:

```go
func hasMCPInMessage(m message.Message) bool {
    for _, b := range m.Content {
        if b.Type == message.BlockToolUse && strings.HasPrefix(b.Name, "mcp__") {
            return true
        }
    }
    return false
}

func indexOfFirstSummary(msgs []message.Message) int {
    for i, m := range msgs {
        if m.Role == message.RoleUser && len(m.Content) == 1 &&
            m.Content[0].Type == message.BlockText &&
            strings.HasPrefix(m.Content[0].Text, "[Compacted earlier conversation") {
            return i
        }
    }
    return -1
}
```

Imports needed: `"strings"`, `"github.com/ricardo/anthrogo/pkg/compact"`, `"github.com/ricardo/anthrogo/internal/session"`, `"github.com/ricardo/anthrogo/pkg/message"`.

**Beware:** if `pkg/query` cannot import `internal/session` (it's currently in the imports list — verify with `grep -n session pkg/query/*.go`), use the existing `RecordHook` signature directly without referencing `session.Record` — just emit via the existing hook. Look at how M2 wired record persistence and follow that pattern.

- [ ] **Step 2.4: Engine.Compact emits EventCompact**

The Engine has a stream channel for events. After `SetInitialMessages`, also send:

```go
e.streamCh <- Event{Kind: EventCompact, Compact: &s}
```

But `Compact` is not called inside an active stream — it's a standalone method. We need a way to ship the event. Two options: (a) caller (Engine.Compact) returns Summary; the slash command formats it and uses `host.AppendUIMessage(...)`; no event needed. (b) push to streamCh anyway in case TUI is listening.

**Pick option (a)** — simpler, no event-routing complexity. Drop `EventCompact` from the plan. The /compact builtin will print directly via host.AppendUIMessage.

Update Task 2.2: skip adding EventCompact. The Engine API surface is just `Compact(ctx, opts) (Summary, error)`.

- [ ] **Step 2.5: Test**

Append to `pkg/query/engine_test.go`:

```go
type recordingHookSink struct {
    preCompactCalled bool
}

func (r *recordingHookSink) FirePostToolUse(context.Context, string, map[string]any, map[string]any) string { return "" }
func (r *recordingHookSink) FireStop(context.Context, string)                                              {}
func (r *recordingHookSink) FirePreCompact(context.Context, string) {
    r.preCompactCalled = true
}

func TestEngine_Compact_FiresHookAndReplaces(t *testing.T) {
    // 15 messages, fake provider scripted to emit "SUM" as summary
    msgs := make([]message.Message, 15)
    for i := range msgs {
        msgs[i] = message.Message{Role: message.RoleUser, Content: []message.Block{{Type: message.BlockText, Text: "x"}}}
    }
    fp := fake.NewScripted([][]fake.Step{
        {{Kind: provider.EventStart}, {Kind: provider.EventTextDelta, Text: "SUM"}, {Kind: provider.EventMessageStop}},
    })
    rec := &recordingHookSink{}
    e := NewEngine(Config{Provider: fp, Model: "m", Hooks: rec})
    e.SetInitialMessages(msgs)

    s, err := e.Compact(context.Background(), CompactOptions{KeepRecent: 10})
    require.NoError(t, err)
    require.False(t, s.Skipped)
    require.True(t, rec.preCompactCalled)
    require.Equal(t, 11, s.NewCount)
    require.Equal(t, 11, len(e.Messages()))
    require.Equal(t, "SUM", s.SummaryText)
}

func TestEngine_Compact_SkipsWhenShort(t *testing.T) {
    e := NewEngine(Config{Provider: nil, Model: "m"})
    e.SetInitialMessages([]message.Message{
        {Role: message.RoleUser, Content: []message.Block{{Type: message.BlockText, Text: "a"}}},
    })
    s, err := e.Compact(context.Background(), CompactOptions{KeepRecent: 10})
    require.NoError(t, err)
    require.True(t, s.Skipped)
}
```

- [ ] **Step 2.6: Run + stage**

```bash
go test ./pkg/query/... -count=1
git add pkg/query/
```

Expected: existing tests still PASS + 2 new tests PASS.

---

## Task 3: `/compact` builtin real impl

**Files:**
- Modify: `pkg/command/builtins/compact.go`
- Create: `pkg/command/builtins/compact_test.go`

- [ ] **Step 3.1: Rewrite `/compact`**

```go
package builtins

import (
    "context"
    "fmt"
    "strconv"
    "strings"

    "github.com/ricardo/anthrogo/pkg/command"
    "github.com/ricardo/anthrogo/pkg/query"
)

type Compact struct{}

func (Compact) Name() string        { return "/compact" }
func (Compact) Aliases() []string   { return nil }
func (Compact) Description() string { return "Summarize earlier conversation to reduce context" }
func (Compact) Type() command.Type  { return command.TypeLocal }

func (Compact) Run(ctx context.Context, args string, host command.Host) (command.Result, error) {
    opts := query.CompactOptions{Trigger: "manual"}
    args = strings.TrimSpace(args)
    if strings.HasPrefix(args, "--keep ") {
        n, err := strconv.Atoi(strings.TrimPrefix(args, "--keep "))
        if err == nil && n > 0 {
            opts.KeepRecent = n
        }
    }
    eng := host.Engine()
    if eng == nil {
        return command.Result{Text: "compact: no active engine"}, nil
    }
    s, err := eng.Compact(ctx, opts)
    if err != nil {
        return command.Result{Text: fmt.Sprintf("compact failed: %v", err)}, nil
    }
    if s.Skipped {
        return command.Result{Text: "compact: " + s.SkipReason}, nil
    }
    return command.Result{Text: fmt.Sprintf(
        "compacted %d → %d messages (~%d → ~%d bytes)",
        s.OriginalCount, s.NewCount, s.OriginalBytes, s.NewBytes,
    )}, nil
}
```

- [ ] **Step 3.2: Tests**

```go
package builtins

import (
    "context"
    "testing"

    "github.com/stretchr/testify/require"

    "github.com/ricardo/anthrogo/pkg/message"
    "github.com/ricardo/anthrogo/pkg/provider"
    "github.com/ricardo/anthrogo/pkg/provider/fake"
    "github.com/ricardo/anthrogo/pkg/query"
)

func newCompactHost(t *testing.T, msgs []message.Message, summary string) *fakeHost {
    fp := fake.NewScripted([][]fake.Step{
        {{Kind: provider.EventStart}, {Kind: provider.EventTextDelta, Text: summary}, {Kind: provider.EventMessageStop}},
    })
    e := query.NewEngine(query.Config{Provider: fp, Model: "m"})
    e.SetInitialMessages(msgs)
    h := newFakeHost()
    h.engine = e
    return h
}

func TestCompactBuiltin_Skipped(t *testing.T) {
    h := newCompactHost(t,
        []message.Message{{Role: message.RoleUser, Content: []message.Block{{Type: message.BlockText, Text: "hi"}}}},
        "unused",
    )
    res, err := Compact{}.Run(context.Background(), "", h)
    require.NoError(t, err)
    require.Contains(t, res.Text, "compact: ")
}

func TestCompactBuiltin_Success(t *testing.T) {
    msgs := make([]message.Message, 15)
    for i := range msgs {
        msgs[i] = message.Message{Role: message.RoleUser, Content: []message.Block{{Type: message.BlockText, Text: "x"}}}
    }
    h := newCompactHost(t, msgs, "SUM")
    res, err := Compact{}.Run(context.Background(), "", h)
    require.NoError(t, err)
    require.Contains(t, res.Text, "compacted 15 → 11 messages")
}
```

You may need to update `fakeHost` to have an `engine *query.Engine` field — check `builtins_test.go`. If `Engine() *query.Engine` returns nil, set it via a `h.engine` field.

- [ ] **Step 3.3: Run + stage**

```bash
go test ./pkg/command/builtins/... -count=1
git add pkg/command/builtins/
```

---

## Task 4: Session record + replay

**Files:**
- Modify: `internal/session/record.go`
- Modify: `internal/session/replay.go` (or wherever Messages() / Replay() consumes records)

- [ ] **Step 4.1: Add CompactRecord type**

In `internal/session/record.go`:

```go
type CompactRecord struct {
    OriginalCount int    `json:"original_count"`
    NewCount      int    `json:"new_count"`
    OriginalBytes int    `json:"original_bytes"`
    NewBytes      int    `json:"new_bytes"`
    Trigger       string `json:"trigger"`
}
```

Extend Record:

```go
type Record struct {
    /* existing */
    Compact *CompactRecord `json:"compact,omitempty"`
}
```

Add a Kind constant if Record uses string kinds: `KindCompact = "compact"`.

- [ ] **Step 4.2: Replay handling**

In `Replay` (or `Messages`): when iterating records, if `Kind == "compact"` is seen, **discard all accumulated messages** and continue. The next records will be the summary message + any new user/assistant turns.

```go
case "compact":
    accumulated = accumulated[:0] // discard prior messages
```

This means resume after a compact starts from the compact point forward. Older compacts are effectively rolled into the most-recent compact's summary, which is correct.

- [ ] **Step 4.3: Test**

In `internal/session/resume_test.go` or `record_test.go` (whichever covers Replay), add:

```go
func TestReplay_DropsMessagesBeforeCompact(t *testing.T) {
    // Build a JSONL with: 3 message records, 1 compact record, 2 more message records.
    // Replay should return only the 2 messages after compact.
    // (Implementation details depend on existing test scaffold — adapt as needed.)
}
```

Skeleton; the implementer fills based on existing replay test pattern.

- [ ] **Step 4.4: Run + stage**

```bash
go test ./internal/session/... -count=1
git add internal/session/
```

---

## Task 5: Version + CHANGELOG + README

**Files:**
- Modify: `internal/version/version.go`
- Modify: `CHANGELOG.md`
- Modify: `README.md`

- [ ] **Step 5.1: Bump**

`internal/version/version.go`: `Version = "0.4.1-dev"`

- [ ] **Step 5.2: CHANGELOG entry**

Prepend after `# Changelog`:

```markdown
## [0.4.1-dev] — 2026-05-20

M4.2 — Real `/compact` (MCP-aware history compaction).

### Added
- `pkg/compact/` package: pure summarization via existing provider.Provider.
- `query.Engine.Compact(ctx, opts) (Summary, error)` — fires PreCompact hook, calls compact.Run, swaps messages.
- `/compact` now actually summarizes earlier turns (default keep 10 most-recent; `--keep N` to override).
- MCP-aware preservation: messages containing `mcp__*` tool_use blocks survive compaction verbatim.
- Session JSONL gains `compact` record kind; replay discards messages before the latest compact.

### Changed
- `query.HookSink` interface gains `FirePreCompact(ctx, trigger)`.

### Known issues / deferred
- Auto-compact on token threshold (M5).
- Byte-count proxy used instead of real tokenizer (M6 with multi-provider).
- Older compacts pre-replay (before the latest compact) are not preserved on resume — `--resume` always rebuilds from the most recent compact forward.
```

- [ ] **Step 5.3: README**

In the existing Hooks section, append: "`PreCompact` fires synchronously before `/compact` runs (M4.2)."

Add a "Compaction" subsection under "Configuration" (or wherever feels natural):

```markdown
## Compaction

For long sessions, `/compact` summarizes earlier turns to cut token cost:

\`\`\`
/compact            # keeps the 10 most-recent messages, summarizes the rest
/compact --keep 20  # keeps 20 most-recent
\`\`\`

Messages containing MCP tool invocations (`mcp__*`) survive compaction verbatim so the model retains tool-call history. `PreCompact` hooks (configured under `hooks.PreCompact`) fire before each compact.
```

- [ ] **Step 5.4: Stage**

```bash
git add internal/version/version.go CHANGELOG.md README.md
```

---

## Task 6: Acceptance

- [ ] **Step 6.1: Sweep**

```bash
make build
./bin/anthrogo --version    # expect: anthrogo 0.4.1-dev
go build ./...
go vet ./...
go test ./...
go test -race -count=2 ./pkg/compact ./pkg/query ./pkg/command/builtins ./internal/hooks ./internal/session ./internal/tui ./pkg/permissions
```

- [ ] **Step 6.2: 3x uncached sweep**

```bash
for i in 1 2 3; do go clean -testcache; go test ./... 2>&1 | grep -E "FAIL|^FAIL" || echo "run $i clean"; done
```

Expected: 3× "clean".

---

## Self-review

- Coverage: §3 algorithm = T1; §4 prompt = T1; §5.1 Engine.Compact = T2; §5.2 pkg/compact = T1; §5.3 builtin = T3; §5.4 session = T4; §5.5 TUI — dropped (slash command formats result directly via host.AppendUIMessage; no event-routing complexity); §6 error handling = T1, T2; §7 testing = T1, T2, T3, T4; §9 acceptance = T6.
- Type consistency: `compact.Output` → `query.Summary` projection; `session.CompactRecord` distinct from `compact.Output`; ok.
- Placeholder scan: no TBDs. T4 has a "fills based on existing replay test pattern" instruction — that's expected leeway, not a placeholder.

# anthrogo M5.1 — Subagents

**Status:** approved (self-authorized)
**Date:** 2026-05-20
**Predecessor:** M4.5

## 1. Goal

Port upstream's "subagent" mechanism. The model invokes a built-in `Task` tool with a description + prompt + agent type; anthrogo spawns a fresh `query.Engine` instance (independent message history, optionally restricted tool subset, agent-specific system prompt suffix), runs it to completion, and returns the sub-engine's final assistant text as the parent's tool_result. Fires the `SubagentStop` hook M4.1 defined but never wired.

This is the first multi-engine surface in anthrogo. Keep M5.1 narrow: one general-purpose subagent type, serial execution (one Task at a time), depth-bounded recursion, shared permission gate.

## 2. Scope

**In:**
- `Task` built-in tool: `{description, prompt, subagent_type}` → returns sub-engine's final assistant text.
- `pkg/query.Engine.RunSubagent(ctx, opts) (string, error)` orchestration method.
- `pkg/subagent/` package: `Type` registry (just "general-purpose" for M5.1) + `Spec{Name, Description, SystemPromptSuffix, ToolAllowlist}`.
- Nested depth limit (default 3): every Task call increments a counter on the engine; exceeding refuses with a clear error.
- Fires `SubagentStop(ctx, reason)` at end of every subagent run (success or error).
- Subagent runs **share** the parent's `permissions.Context` and `hooks.Manager` (so PreToolUse hooks still fire inside subagents; user rules still apply).
- TUI shows subagent output indented under a `[Task: <description>]` header.

**Out (deferred to M5.2 / M5.3):**
- Concurrent subagents (parent spawning N in parallel).
- Independent permission context per subagent (today inherits parent).
- Subagent gets its own JSONL session record (today nested under parent).
- Subagent type plugins (`pkg/subagent/*.yaml` user-defined types).
- KAIROS / cross-process subagent dispatch.

## 3. Code organization

```
pkg/subagent/
├── subagent.go         # Spec + Registry + builtin defs
├── prompts.go          # systemPromptForType helpers (embedded)
├── subagent_test.go
```

```go
// pkg/subagent/subagent.go
type Spec struct {
    Name                 string   // "general-purpose"
    Description          string   // shown to model in Task tool schema
    SystemPromptSuffix   string   // appended to parent system prompt
    ToolAllowlist        []string // empty = inherit all parent tools
}

func DefaultRegistry() *Registry  // returns Registry pre-populated with general-purpose
type Registry struct { /* map[name]Spec, sorted List, Get */ }
```

## 4. `Task` tool

`pkg/tool/task.go`:

```go
type Task struct {
    DefaultPermission
    runner func(ctx context.Context, opts TaskOptions) (string, error)
    reg    *subagent.Registry
}

type TaskOptions struct {
    Description  string
    Prompt       string
    SubagentType string
}

func NewTask(reg *subagent.Registry, runner func(...) (string, error)) *Task
```

Schema:

```json
{
  "type": "object",
  "properties": {
    "description": {"type": "string", "description": "Short 5-10 word summary of the task (shown in UI)."},
    "prompt":      {"type": "string", "description": "Self-contained instructions to the subagent. Brief the subagent like a colleague — it has no memory of this conversation."},
    "subagent_type": {"type": "string", "description": "Subagent type name (e.g., \"general-purpose\")."}
  },
  "required": ["description", "prompt", "subagent_type"]
}
```

Description() emitted to the model lists the registered types with their descriptions (model needs to know which to pick).

`Call()` delegates to the injected `runner` callback (in cmd/anthrogo's wiring, this is `engine.RunSubagent`).

## 5. `Engine.RunSubagent`

```go
type SubagentOptions struct {
    Type        string
    Description string
    Prompt      string
}

func (e *Engine) RunSubagent(ctx context.Context, opts SubagentOptions) (string, error)
```

Flow:
1. Increment `e.subagentDepth` under lock. If > MaxSubagentDepth (default 3), decrement + return error "subagent depth limit exceeded".
2. Look up the subagent Spec from `e.cfg.SubagentRegistry`. If unknown name, decrement + return error.
3. Build subagent Config:
   - same Provider + Model
   - SystemPrompt = parent's + "\n\n" + spec.SystemPromptSuffix
   - Tools = if spec.ToolAllowlist non-empty, filter parent's registry to those names; else inherit
   - Permissions, Hooks, RecordHook = inherit (M5.1 shares state)
   - SubagentDepth = e.subagentDepth (so the child's RunSubagent sees the bumped value)
4. Construct child Engine via NewEngine.
5. Run one turn: `ch := child.SubmitMessage(ctx, opts.Prompt)`. Drain channel; accumulate text from EventTextDelta blocks attributed to the final assistant message. Capture final text on EventMessageStop. Pass through tool-use events implicitly (the child runs its own loop).
6. After drain, fire SubagentStop hook: `if e.cfg.Hooks != nil { e.cfg.Hooks.FireSubagentStop(ctx, "end_turn") }`.
7. Decrement subagentDepth.
8. Return the collected text.

`Engine` gets a new field `subagentDepth int` (under `mu`).

`Config` gets:
- `SubagentRegistry *subagent.Registry` (nil-safe; Task tool errors gracefully if nil)
- `SubagentDepth int` (set by parent; 0 at top level)
- `MaxSubagentDepth int` (default 3 if zero)

## 6. SystemPrompt + tool list injection

`internal/system.BuildSystemPrompt` gets a new optional section listing subagent types:

```
Available subagent types (invoke via the Task tool):
- general-purpose: General-purpose agent for complex multi-step tasks.
```

Add `Options.Subagents []subagent.Spec`.

## 7. Plan-mode interaction

Plan mode currently blocks every `mcp__*` and the static Write/Edit/NotebookEdit set (M4.5). The Task tool itself is read-only at the API level (returns text), but the **subagent** it spawns can call any tool. Solution: in plan mode, Task tool's Permission() returns Deny with message "plan mode blocks subagent dispatch; switch to default mode to run Task." This keeps the safety contract crisp.

Implement by giving Task a custom `Permission(ctx, input) permissions.Decision` that checks if `c.Mode == ModePlan` (need access to the gate context — pass it via Tool.Context field, or just defer to the gate by returning Ask and letting the gate use `permissions.IsWriteTool` extended to include "Task"). **Simpler:** add "Task" to `IsWriteTool`. Then plan mode blocks Task automatically by the existing M4.5 mechanism.

## 8. Testing

- `pkg/subagent/subagent_test.go`:
  - Registry default contains "general-purpose"
  - Get returns false for unknown
- `pkg/tool/task_test.go`:
  - Schema sanity
  - Call delegates to runner with parsed opts
  - Missing subagent_type → IsError
- `pkg/query/engine_test.go`:
  - `TestEngine_RunSubagent_HappyPath` — fake provider scripted to emit one assistant text + end_turn for the child; assert returned string + SubagentStop fired
  - `TestEngine_RunSubagent_DepthLimit` — call RunSubagent recursively to depth+1; assert depth-3 returns "depth limit" error
  - `TestEngine_RunSubagent_UnknownType` — registry has no "weird" → error

## 9. Acceptance

- `go build/vet/test/-race` clean
- 3× uncached sweep clean
- Version 0.5.0-dev
- CHANGELOG + README "Subagents" section
- Bump roadmap table in README to reflect M5.1 shipped

## 10. Deferred to M5.2 / M5.3

- WebSocket / OAuth / Elicitations / Resources MCP debt (M5.2)
- Concurrent multi-subagent dispatch (M5.3)
- User-defined subagent types via YAML (M5.3)
- Independent permission context per subagent (M5.3)
- Subagent JSONL session isolation (M5.3)
- KAIROS coordinator (M5.3)

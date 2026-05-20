# anthrogo M5.3 — Subagent polish (concurrent + isolated perms + YAML types)

**Status:** approved (self-authorized)
**Date:** 2026-05-20
**Predecessor:** M5.2

## 1. Goal

Three M5.3 deliverables:
- **Concurrent multi-Task dispatch** — model can spawn N parallel subagents in one turn; engine no longer serializes.
- **Independent permission context per subagent** — shallow-clone `permissions.Context` so subagents can toggle Mode (e.g. enter plan mode internally) without affecting parent. Rules and HookDecide are shared by-reference (immutable in practice).
- **User-defined subagent types via YAML** — `~/.anthrogo/subagents/<name>.yaml` files merge into the registry alongside the built-in `general-purpose` type.

Out of scope (defer to M6): independent JSONL per subagent; MCP resources/list_changed; real TUI form elicitation; WebSocket; OAuth; KAIROS coordinator. Those are independent large-ish projects.

## 2. Concurrent Task dispatch

### Current state
`pkg/tool/task.go`: `Task.IsConcurrencySafe() == false`. The engine's tool execution loop won't dispatch multiple Tasks concurrently within one turn.

### Change
- Set `Task.IsConcurrencySafe() == true`.
- `pkg/query.Engine.RunSubagent` already uses `e.mu` for the depth counter and creates an isolated child Engine; the actual subagent work happens on independent goroutines spawned by the tool-execution loop. The depth counter is the only piece of shared mutable state during subagent run, and it's already mutex-protected.
- **But:** `e.subagentDepth` is a SINGLE counter shared by all concurrent subagents. The depth-limit semantics should mean "any single chain of subagents can be at most N deep" rather than "no more than N subagents running concurrently". To preserve that semantics under concurrency: keep `subagentDepth` as a counter that represents the *maximum currently-active* depth via increment/decrement. Each goroutine increments at start, decrements at end. A peer-parallel call increments to N concurrently with another, but each individual call sees its OWN current depth via the increment return. Refactor:
  ```go
  e.mu.Lock()
  if e.subagentDepth >= maxDepth { e.mu.Unlock(); return error }
  e.subagentDepth++
  current := e.subagentDepth
  e.mu.Unlock()
  defer e.mu.Lock(); e.subagentDepth--; e.mu.Unlock()
  ```
  This already exists from M5.1. But the depth-limit check is "compare against the absolute current value", which under concurrency caps the *concurrent total*, not the *recursive chain depth*. Need to track "depth relative to root" via passing the bumped value through child's Config.SubagentDepth.
  
  Actually, that's what M5.1 already does: child Config gets `SubagentDepth: currentDepth`, so when the child calls `RunSubagent`, its counter starts where parent left off. Confirms the chain depth is tracked correctly. The parent's counter going to 3 means "3 concurrent active descendants OR a 3-deep chain". This is acceptable — both interpretations are useful for safety.

### Test for concurrent dispatch
`TestEngine_RunSubagent_ConcurrentTwoTasksFromOneTurn`: scripted parent emits a single turn with two `Task` tool_use blocks. The engine loop dispatches them via goroutines (because IsConcurrencySafe=true). Each runs a child engine that emits "FOO" and "BAR" respectively. Assert both tool_results are present.

### Behavior change disclosure
CHANGELOG must note: "Subagents now run concurrently when the model invokes multiple Task tool_use blocks in one turn. Per-subagent stack ordering of stderr/log output may interleave; hooks see hooks from all in-flight subagents merged."

## 3. Independent permission context per subagent

### Current state
`pkg/query.Engine.RunSubagent` passes `Permissions: e.cfg.Permissions` to the child Config — the same pointer. Means a subagent that mutates Mode (via EnterPlanMode tool) affects the parent.

### Change
Add a shallow-clone method to `permissions.Context`:

```go
// Clone returns a copy of c suitable for handing to a subagent. Rules and
// HookDecide are shared by reference (treated as immutable); Mode, PrePlanMode,
// ShouldAvoidPrompts, IsBypassAvailable are copied so subagent toggles don't
// affect the parent.
func (c *Context) Clone() *Context {
    cp := *c
    return &cp
}
```

(`RulesBySource` is `map[Source][]Rule` — shared by reference; rule add/remove from a subagent would affect parent if any tool did that, but no current tool does. Acceptable.)

`Engine.RunSubagent`: use `e.cfg.Permissions.Clone()` for the child Config.

### Test
`TestEngine_RunSubagent_ChildModeChangeDoesntAffectParent`: child uses EnterPlanMode in its turn; assert parent's `Permissions.Mode` is unchanged after RunSubagent returns.

## 4. User-defined subagent types via YAML

### Layout

```
~/.anthrogo/subagents/code-reviewer.yaml         # home
<cwd>/.anthrogo/subagents/code-reviewer.yaml     # project; overrides home
```

YAML schema:

```yaml
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

If `tool_allowlist` is empty, subagent inherits parent's full tool registry.

### Loader

`pkg/subagent/loader.go`:

```go
func LoadAll(homeRoot, cwdRoot string) ([]Spec, []string, error)
```

Scans `*.yaml` in each root (non-recursive). Parses each into a Spec. Validates: name matches `^[a-z][a-z0-9-]{0,63}$`, name matches filename stem, description + system_prompt_suffix non-empty (allow either to be empty if both not; but at least one must be non-empty so the subagent has SOME differentiation from general-purpose). cwd wins on duplicate name (warning).

Build-in `general-purpose` always present (cannot be overridden — name reserved).

### cmd/anthrogo wiring

```go
subagentReg := subagent.DefaultRegistry()
homeSubRoot := filepath.Join(os.Getenv("HOME"), ".anthrogo", "subagents")
cwdSubRoot := filepath.Join(cwd, ".anthrogo", "subagents")
userSubs, swarn, _ := subagent.LoadAll(homeSubRoot, cwdSubRoot)
for _, w := range swarn { fmt.Fprintln(os.Stderr, "subagents:", w) }
for _, s := range userSubs {
    if s.Name == "general-purpose" { continue /* warned in loader */ }
    subagentReg.Register(s)
}
```

`/subagents` slash command (optional but easy):
- `/subagents` — list all registered types
- `/subagents show <name>` — print Spec details
- `/subagents reload` — re-scan + reload

(M5.3 ships only the `list` and `show` variants if `reload` is annoying to wire; document in CHANGELOG. **Decision: ship all three, mirroring `/skills` pattern.**)

## 5. Tests

- `pkg/subagent/loader_test.go` — valid home, cwd overrides home with warning, bad name (reserved "general-purpose" rejected), empty description, file not matching name.
- `pkg/subagent/subagent_test.go` — extend with reload semantics on registry.
- `pkg/query/engine_test.go`:
  - TestEngine_RunSubagent_ChildModeChangeDoesntAffectParent
  - TestEngine_RunSubagent_ConcurrentTwoTasksFromOneTurn
- `pkg/command/builtins/subagents_test.go` — new builtin coverage.

## 6. Code organization

```
pkg/subagent/loader.go               # new
pkg/subagent/loader_test.go          # new
pkg/subagent/testdata/...            # new
pkg/permissions/context.go           # add Clone() method
pkg/query/engine.go                  # use Clone() in RunSubagent
pkg/tool/task.go                     # IsConcurrencySafe → true
pkg/command/builtins/subagents.go    # new
pkg/command/builtins/subagents_test.go
cmd/anthrogo/main.go                 # LoadAll + Register loop + builtin
internal/version/version.go          # 0.5.2-dev
CHANGELOG.md, README.md
```

## 7. Acceptance

- `go build/vet/test/-race` clean
- 3× uncached sweep clean
- Version `0.5.2-dev`
- CHANGELOG + README updated; README "Subagents" section gets a "Custom types" subsection

## 8. Deferred to M6

- Independent JSONL per subagent
- MCP resources/list_changed + subscription
- Real TUI form elicitation handler
- WebSocket MCP transport
- OAuth 2.1 client flow
- KAIROS coordinator / remote sessions

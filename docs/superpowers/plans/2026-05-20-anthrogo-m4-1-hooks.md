# M4.1 Hooks Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Port upstream claude-code@2.1.88's hook framework to anthrogo — 9 event types, JSON-over-stdin protocol, sync/async execution model, integration with permission gate / query loop / TUI.

**Architecture:** New `internal/hooks` package with `Config / Event / Runner / Manager / decision`. Wired into existing `pkg/permissions/gate.go` (PreToolUse), `pkg/query/loop.go` (PostToolUse, Stop), `internal/tui/app.go` and `internal/headless/runner.go` (UserPromptSubmit, SessionStart/End, Notification). Function-pointer indirection on `permissions.Context` to avoid import cycle.

**Tech Stack:** Go 1.25, `os/exec`, `encoding/json`, `regexp`, `context.WithTimeout`. No new third-party deps.

---

# Phase A — Config + Event types (3 tasks)

## Task A1: Config schema + path expansion

**Files:**
- Create: `internal/hooks/config.go`
- Create: `internal/hooks/config_test.go`
- Modify: `internal/config/loader.go` (add `Hooks hooks.Config` field)

- [ ] **Step A1.1: Write failing config test**

Create `internal/hooks/config_test.go`:

```go
package hooks

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestConfig_ParsesAndExpands(t *testing.T) {
	t.Setenv("HOME", "/Users/test")
	t.Setenv("ANTHROGO_FOO", "bar")
	raw := []byte(`
PreToolUse:
  - matcher: "Bash"
    command: ~/.anthrogo/hooks/audit.sh
    timeout: 15s
  - matcher: "Write|Edit"
    command: $ANTHROGO_FOO/scripts/x.sh
PostToolUse:
  - command: /abs/path/no-matcher.sh
UserPromptSubmit:
  - command: ~/inject.sh
`)
	var c Config
	require.NoError(t, yaml.Unmarshal(raw, &c))
	require.NoError(t, c.Expand())

	require.Equal(t, "/Users/test/.anthrogo/hooks/audit.sh", c.PreToolUse[0].Command)
	require.Equal(t, 15*time.Second, c.PreToolUse[0].Timeout)
	require.Equal(t, "bar/scripts/x.sh", c.PreToolUse[1].Command)
	require.Equal(t, "/abs/path/no-matcher.sh", c.PostToolUse[0].Command)
	require.Equal(t, "/Users/test/inject.sh", c.UserPromptSubmit[0].Command)
}

func TestConfig_DefaultsTimeoutByEvent(t *testing.T) {
	var c Config
	require.NoError(t, yaml.Unmarshal([]byte(`
PreToolUse:
  - command: /x.sh
Stop:
  - command: /y.sh
Notification:
  - command: /z.sh
`), &c))
	require.NoError(t, c.Expand())
	require.Equal(t, 30*time.Second, c.PreToolUse[0].Timeout)
	require.Equal(t, 10*time.Second, c.Stop[0].Timeout)
	require.Equal(t, 5*time.Second, c.Notification[0].Timeout)
}

func TestConfig_InvalidRegexpIsDroppedWithWarn(t *testing.T) {
	var c Config
	require.NoError(t, yaml.Unmarshal([]byte(`
PreToolUse:
  - matcher: "(unclosed"
    command: /a.sh
  - matcher: "Bash"
    command: /b.sh
`), &c))
	warnings := c.Validate()
	require.Len(t, warnings, 1)
	require.Contains(t, warnings[0], "(unclosed")
	require.Len(t, c.PreToolUse, 1)
	require.Equal(t, "/b.sh", c.PreToolUse[0].Command)
}

func TestConfig_AppendOverlay(t *testing.T) {
	base := Config{
		PreToolUse: []Spec{{Matcher: "Bash", Command: "/a.sh"}},
	}
	overlay := Config{
		PreToolUse: []Spec{{Matcher: "Write", Command: "/b.sh"}},
		Stop:       []Spec{{Command: "/c.sh"}},
	}
	merged := base.AppendOverlay(overlay)
	require.Len(t, merged.PreToolUse, 2)
	require.Equal(t, "/a.sh", merged.PreToolUse[0].Command)
	require.Equal(t, "/b.sh", merged.PreToolUse[1].Command)
	require.Equal(t, "/c.sh", merged.Stop[0].Command)
}
```

- [ ] **Step A1.2: Run — expect FAIL**

```bash
cd /Users/ricardo/Documents/公司学习文件/我自己的agent的cli/anthrogo
go test ./internal/hooks/...
```

Expected: build failure ("no such package internal/hooks" or "Config undefined").

- [ ] **Step A1.3: Implement config.go**

Create `internal/hooks/config.go`:

```go
package hooks

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

// Spec is one hook entry under a given event.
type Spec struct {
	Matcher string        `yaml:"matcher,omitempty"`
	Command string        `yaml:"command"`
	Timeout time.Duration `yaml:"timeout,omitempty"`

	matcherRE *regexp.Regexp // compiled lazily in Validate
}

// Config holds per-event hook lists. Field names match event names so YAML
// keys are PascalCase (PreToolUse, etc.).
type Config struct {
	PreToolUse       []Spec `yaml:"PreToolUse,omitempty"`
	PostToolUse      []Spec `yaml:"PostToolUse,omitempty"`
	UserPromptSubmit []Spec `yaml:"UserPromptSubmit,omitempty"`
	Stop             []Spec `yaml:"Stop,omitempty"`
	SubagentStop     []Spec `yaml:"SubagentStop,omitempty"`
	Notification     []Spec `yaml:"Notification,omitempty"`
	PreCompact       []Spec `yaml:"PreCompact,omitempty"`
	SessionStart     []Spec `yaml:"SessionStart,omitempty"`
	SessionEnd       []Spec `yaml:"SessionEnd,omitempty"`
}

func (c *Config) allLists() []*[]Spec {
	return []*[]Spec{
		&c.PreToolUse, &c.PostToolUse, &c.UserPromptSubmit,
		&c.Stop, &c.SubagentStop, &c.Notification,
		&c.PreCompact, &c.SessionStart, &c.SessionEnd,
	}
}

// Expand replaces ~/ and $VAR in every Command, fills in default Timeout per event.
func (c *Config) Expand() error {
	home, _ := os.UserHomeDir()
	defaults := map[string]time.Duration{
		"PreToolUse":       30 * time.Second,
		"PostToolUse":      30 * time.Second,
		"UserPromptSubmit": 30 * time.Second,
		"Stop":             10 * time.Second,
		"SubagentStop":     10 * time.Second,
		"Notification":     5 * time.Second,
		"PreCompact":       30 * time.Second,
		"SessionStart":     5 * time.Second,
		"SessionEnd":       5 * time.Second,
	}
	apply := func(list []Spec, defTimeout time.Duration) []Spec {
		out := make([]Spec, 0, len(list))
		for _, s := range list {
			s.Command = expandPath(s.Command, home)
			if s.Timeout == 0 {
				s.Timeout = defTimeout
			}
			out = append(out, s)
		}
		return out
	}
	c.PreToolUse = apply(c.PreToolUse, defaults["PreToolUse"])
	c.PostToolUse = apply(c.PostToolUse, defaults["PostToolUse"])
	c.UserPromptSubmit = apply(c.UserPromptSubmit, defaults["UserPromptSubmit"])
	c.Stop = apply(c.Stop, defaults["Stop"])
	c.SubagentStop = apply(c.SubagentStop, defaults["SubagentStop"])
	c.Notification = apply(c.Notification, defaults["Notification"])
	c.PreCompact = apply(c.PreCompact, defaults["PreCompact"])
	c.SessionStart = apply(c.SessionStart, defaults["SessionStart"])
	c.SessionEnd = apply(c.SessionEnd, defaults["SessionEnd"])
	return nil
}

// Validate compiles all matchers; invalid ones drop their spec and append a warning.
func (c *Config) Validate() []string {
	var warnings []string
	for _, listPtr := range c.allLists() {
		filtered := (*listPtr)[:0]
		for _, s := range *listPtr {
			if s.Matcher != "" {
				re, err := regexp.Compile(s.Matcher)
				if err != nil {
					warnings = append(warnings,
						fmt.Sprintf("dropped hook %s: bad matcher %q (%v)", s.Command, s.Matcher, err))
					continue
				}
				s.matcherRE = re
			}
			filtered = append(filtered, s)
		}
		*listPtr = filtered
	}
	return warnings
}

// AppendOverlay returns a new Config = c with each event's list extended by overlay's list.
func (c Config) AppendOverlay(overlay Config) Config {
	out := c
	out.PreToolUse = append(out.PreToolUse, overlay.PreToolUse...)
	out.PostToolUse = append(out.PostToolUse, overlay.PostToolUse...)
	out.UserPromptSubmit = append(out.UserPromptSubmit, overlay.UserPromptSubmit...)
	out.Stop = append(out.Stop, overlay.Stop...)
	out.SubagentStop = append(out.SubagentStop, overlay.SubagentStop...)
	out.Notification = append(out.Notification, overlay.Notification...)
	out.PreCompact = append(out.PreCompact, overlay.PreCompact...)
	out.SessionStart = append(out.SessionStart, overlay.SessionStart...)
	out.SessionEnd = append(out.SessionEnd, overlay.SessionEnd...)
	return out
}

func expandPath(p, home string) string {
	if strings.HasPrefix(p, "~/") {
		p = home + p[1:]
	}
	return os.ExpandEnv(p)
}
```

- [ ] **Step A1.4: Run — expect PASS**

```bash
go test ./internal/hooks/...
```

Expected: 4 PASS.

- [ ] **Step A1.5: Wire into config loader**

Modify `internal/config/loader.go`:
- Add import `"github.com/ricardo/anthrogo/internal/hooks"`
- Add field `Hooks hooks.Config \`yaml:"hooks,omitempty"\`` to `Config` struct
- After existing YAML unmarshal, also call `cfg.Hooks.Expand()` and discard warnings here (warnings logged later by caller)

- [ ] **Step A1.6: Stage**

```bash
git add internal/hooks/ internal/config/loader.go
```

---

## Task A2: Event types and payload structs

**Files:**
- Create: `internal/hooks/event.go`
- Create: `internal/hooks/event_test.go`

- [ ] **Step A2.1: Write failing test**

Create `internal/hooks/event_test.go`:

```go
package hooks

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEventName_StringRoundtrip(t *testing.T) {
	require.Equal(t, "PreToolUse", string(EventPreToolUse))
	require.Equal(t, "SessionEnd", string(EventSessionEnd))
}

func TestPayload_PreToolUse_MarshalsAllFields(t *testing.T) {
	p := PreToolUsePayload{
		Common: Common{
			HookEventName: EventPreToolUse,
			SessionID:     "abc",
			Cwd:           "/x",
			Version:       "0.4.0-dev",
		},
		ToolName:  "Bash",
		ToolInput: map[string]any{"command": "ls"},
	}
	raw, err := json.Marshal(p)
	require.NoError(t, err)
	var back map[string]any
	require.NoError(t, json.Unmarshal(raw, &back))
	require.Equal(t, "PreToolUse", back["hook_event_name"])
	require.Equal(t, "abc", back["session_id"])
	require.Equal(t, "/x", back["cwd"])
	require.Equal(t, "Bash", back["tool_name"])
	ti, ok := back["tool_input"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "ls", ti["command"])
}
```

- [ ] **Step A2.2: Run — expect FAIL**

```bash
go test ./internal/hooks/...
```

Expected: build failure on `EventPreToolUse`, `PreToolUsePayload`, `Common`.

- [ ] **Step A2.3: Implement event.go**

Create `internal/hooks/event.go`:

```go
package hooks

// EventName is one of the 9 hook event names.
type EventName string

const (
	EventPreToolUse       EventName = "PreToolUse"
	EventPostToolUse      EventName = "PostToolUse"
	EventUserPromptSubmit EventName = "UserPromptSubmit"
	EventStop             EventName = "Stop"
	EventSubagentStop     EventName = "SubagentStop"
	EventNotification     EventName = "Notification"
	EventPreCompact       EventName = "PreCompact"
	EventSessionStart     EventName = "SessionStart"
	EventSessionEnd       EventName = "SessionEnd"
)

// Common is the envelope every payload carries.
type Common struct {
	HookEventName EventName `json:"hook_event_name"`
	SessionID     string    `json:"session_id"`
	Cwd           string    `json:"cwd"`
	Version       string    `json:"anthrogo_version"`
}

type PreToolUsePayload struct {
	Common
	ToolName  string         `json:"tool_name"`
	ToolInput map[string]any `json:"tool_input"`
}

type PostToolUsePayload struct {
	Common
	ToolName     string         `json:"tool_name"`
	ToolInput    map[string]any `json:"tool_input"`
	ToolResponse map[string]any `json:"tool_response"`
}

type UserPromptSubmitPayload struct {
	Common
	Prompt string `json:"prompt"`
}

type StopPayload struct {
	Common
	StopReason string `json:"stop_reason"`
}

type NotificationPayload struct {
	Common
	Message string `json:"message"`
	Kind    string `json:"kind"`
}

type PreCompactPayload struct {
	Common
	Trigger string `json:"trigger"`
}

type SessionStartPayload struct {
	Common
	Kind string `json:"kind"`
}

type SessionEndPayload struct {
	Common
	Kind string `json:"kind"`
}

// Output is what the hook writes to stdout. All fields optional.
type Output struct {
	Continue           *bool                `json:"continue,omitempty"`
	StopReason         string               `json:"stopReason,omitempty"`
	HookSpecificOutput *HookSpecificOutput  `json:"hookSpecificOutput,omitempty"`
}

type HookSpecificOutput struct {
	HookEventName            EventName      `json:"hookEventName"`
	PermissionDecision       string         `json:"permissionDecision,omitempty"`
	PermissionDecisionReason string         `json:"permissionDecisionReason,omitempty"`
	ModifiedInput            map[string]any `json:"modifiedInput,omitempty"`
	AdditionalContext        string         `json:"additionalContext,omitempty"`
}
```

- [ ] **Step A2.4: Run — expect PASS**

```bash
go test ./internal/hooks/...
```

Expected: 6 PASS (4 from A1 + 2 from A2).

- [ ] **Step A2.5: Stage**

```bash
git add internal/hooks/event.go internal/hooks/event_test.go
```

---

## Task A3: testdata shell scripts

**Files:**
- Create: `internal/hooks/testdata/allow.sh`
- Create: `internal/hooks/testdata/deny.sh`
- Create: `internal/hooks/testdata/inject-context.sh`
- Create: `internal/hooks/testdata/mutate-input.sh`
- Create: `internal/hooks/testdata/slow.sh`
- Create: `internal/hooks/testdata/crash.sh`
- Create: `internal/hooks/testdata/passthrough.sh`

- [ ] **Step A3.1: Create scripts**

`internal/hooks/testdata/allow.sh`:
```bash
#!/usr/bin/env bash
# read & discard stdin, exit 0
cat > /dev/null
exit 0
```

`internal/hooks/testdata/deny.sh`:
```bash
#!/usr/bin/env bash
cat > /dev/null
echo "denied by policy" 1>&2
exit 2
```

`internal/hooks/testdata/inject-context.sh`:
```bash
#!/usr/bin/env bash
cat > /dev/null
cat <<'JSON'
{"hookSpecificOutput":{"hookEventName":"UserPromptSubmit","additionalContext":"injected ctx"}}
JSON
exit 0
```

`internal/hooks/testdata/mutate-input.sh`:
```bash
#!/usr/bin/env bash
cat > /dev/null
cat <<'JSON'
{"hookSpecificOutput":{"hookEventName":"PreToolUse","modifiedInput":{"command":"ls -al"}}}
JSON
exit 0
```

`internal/hooks/testdata/slow.sh`:
```bash
#!/usr/bin/env bash
cat > /dev/null
sleep 60
exit 0
```

`internal/hooks/testdata/crash.sh`:
```bash
#!/usr/bin/env bash
cat > /dev/null
echo "boom" 1>&2
exit 7
```

`internal/hooks/testdata/passthrough.sh`:
```bash
#!/usr/bin/env bash
# Echo the received JSON to stderr so tests can assert on it.
in=$(cat)
echo "$in" 1>&2
exit 0
```

- [ ] **Step A3.2: Make executable + stage**

```bash
chmod +x internal/hooks/testdata/*.sh
git add internal/hooks/testdata/
```

---

# Phase B — Runner (3 tasks)

## Task B4: Runner — happy path

**Files:**
- Create: `internal/hooks/runner.go`
- Create: `internal/hooks/runner_test.go`

- [ ] **Step B4.1: Write failing test**

Create `internal/hooks/runner_test.go`:

```go
package hooks

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func script(name string) string {
	abs, _ := filepath.Abs(filepath.Join("testdata", name))
	return abs
}

func TestRunner_AllowExit0(t *testing.T) {
	r, err := RunHook(context.Background(), Spec{Command: script("allow.sh"), Timeout: 3 * time.Second}, map[string]any{
		"hook_event_name": "PreToolUse",
	})
	require.NoError(t, err)
	require.Equal(t, 0, r.ExitCode)
	require.Empty(t, r.Stderr)
	require.Nil(t, r.Output)
}

func TestRunner_PassesStdinJSON(t *testing.T) {
	r, err := RunHook(context.Background(), Spec{Command: script("passthrough.sh"), Timeout: 3 * time.Second}, map[string]any{
		"hook_event_name": "PreToolUse",
		"tool_name":       "Bash",
	})
	require.NoError(t, err)
	require.Equal(t, 0, r.ExitCode)
	require.Contains(t, string(r.Stderr), `"tool_name":"Bash"`)
}
```

- [ ] **Step B4.2: Run — expect FAIL**

```bash
go test ./internal/hooks/... -run TestRunner_
```

Expected: build failure on `RunHook`, `Result`.

- [ ] **Step B4.3: Implement runner.go**

Create `internal/hooks/runner.go`:

```go
package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"syscall"
	"time"
)

const (
	maxStdoutBytes = 1 << 20 // 1 MiB
	maxStderrBytes = 1 << 20
)

// Result is what the runner reports back. Output is nil if stdout was empty
// or not parseable JSON.
type Result struct {
	ExitCode int
	Stdout   []byte
	Stderr   []byte
	Output   *Output
	TimedOut bool
}

// RunHook spawns `spec.Command`, feeds `payload` (marshaled JSON) to stdin,
// waits up to spec.Timeout. Always returns a Result on completion or timeout;
// returns a non-nil error only on setup failure (e.g. command not found).
func RunHook(ctx context.Context, spec Spec, payload any) (*Result, error) {
	timeout := spec.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, "sh", "-c", spec.Command)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		if pgid, err := syscall.Getpgid(cmd.Process.Pid); err == nil {
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
		} else {
			_ = cmd.Process.Kill()
		}
		return nil
	}

	in, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &limitedWriter{w: &stdoutBuf, n: maxStdoutBytes}
	cmd.Stderr = &limitedWriter{w: &stderrBuf, n: maxStderrBytes}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start hook %s: %w", spec.Command, err)
	}

	// Write payload + close stdin.
	encoded, _ := json.Marshal(payload)
	go func() {
		defer in.Close()
		_, _ = io.Copy(in, bytes.NewReader(encoded))
	}()

	waitErr := cmd.Wait()

	r := &Result{
		Stdout: stdoutBuf.Bytes(),
		Stderr: stderrBuf.Bytes(),
	}

	switch {
	case waitErr == nil:
		r.ExitCode = 0
	case errors.Is(runCtx.Err(), context.DeadlineExceeded):
		r.TimedOut = true
		r.ExitCode = -1
	default:
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			r.ExitCode = exitErr.ExitCode()
		} else {
			return nil, waitErr
		}
	}

	if len(r.Stdout) > 0 {
		var out Output
		if err := json.Unmarshal(r.Stdout, &out); err == nil {
			r.Output = &out
		}
	}
	return r, nil
}

// limitedWriter caps how much we collect; further writes are dropped silently.
type limitedWriter struct {
	w *bytes.Buffer
	n int
}

func (l *limitedWriter) Write(p []byte) (int, error) {
	if l.n <= 0 {
		return len(p), nil
	}
	if len(p) > l.n {
		p = p[:l.n]
	}
	n, err := l.w.Write(p)
	l.n -= n
	return n, err
}
```

- [ ] **Step B4.4: Run — expect PASS**

```bash
go test ./internal/hooks/... -run TestRunner_
```

Expected: 2 PASS.

- [ ] **Step B4.5: Stage**

```bash
git add internal/hooks/runner.go internal/hooks/runner_test.go
```

---

## Task B5: Runner — non-zero / timeout / malformed JSON

**Files:**
- Modify: `internal/hooks/runner_test.go`

- [ ] **Step B5.1: Add failing tests**

Append to `runner_test.go`:

```go
func TestRunner_Exit2Block(t *testing.T) {
	r, err := RunHook(context.Background(), Spec{Command: script("deny.sh"), Timeout: 3 * time.Second}, map[string]any{})
	require.NoError(t, err)
	require.Equal(t, 2, r.ExitCode)
	require.Contains(t, string(r.Stderr), "denied by policy")
}

func TestRunner_ExitOtherNonZero(t *testing.T) {
	r, err := RunHook(context.Background(), Spec{Command: script("crash.sh"), Timeout: 3 * time.Second}, map[string]any{})
	require.NoError(t, err)
	require.Equal(t, 7, r.ExitCode)
	require.Contains(t, string(r.Stderr), "boom")
}

func TestRunner_Timeout(t *testing.T) {
	r, err := RunHook(context.Background(), Spec{Command: script("slow.sh"), Timeout: 200 * time.Millisecond}, map[string]any{})
	require.NoError(t, err)
	require.True(t, r.TimedOut)
	require.Equal(t, -1, r.ExitCode)
}

func TestRunner_ParsesJSONOutput(t *testing.T) {
	r, err := RunHook(context.Background(), Spec{Command: script("mutate-input.sh"), Timeout: 3 * time.Second}, map[string]any{
		"tool_name": "Bash",
	})
	require.NoError(t, err)
	require.Equal(t, 0, r.ExitCode)
	require.NotNil(t, r.Output)
	require.NotNil(t, r.Output.HookSpecificOutput)
	require.Equal(t, "ls -al", r.Output.HookSpecificOutput.ModifiedInput["command"])
}

func TestRunner_CommandNotFound(t *testing.T) {
	_, err := RunHook(context.Background(), Spec{Command: "/nonexistent/binary", Timeout: 1 * time.Second}, map[string]any{})
	// `sh -c` with non-existent binary exits 127, doesn't error in Start.
	// So we expect no setup error but exit code 127.
	require.NoError(t, err)
}
```

- [ ] **Step B5.2: Run — expect PASS**

```bash
go test ./internal/hooks/... -run TestRunner_
```

Expected: 7 PASS total in runner tests.

- [ ] **Step B5.3: Stage**

```bash
git add internal/hooks/runner_test.go
```

---

## Task B6: Runner — race-detector clean

- [ ] **Step B6.1: Run race detector**

```bash
go test -race ./internal/hooks/... -count=3
```

Expected: clean, 3 consecutive uncached passes. If anything fires, debug — likely the stdin goroutine leaking the writer.

- [ ] **Step B6.2: Stage if any fix needed**

If you fixed something in runner.go, `git add internal/hooks/runner.go`. Otherwise nothing to stage.

---

# Phase C — Manager + decision combining (3 tasks)

## Task C7: Manager skeleton

**Files:**
- Create: `internal/hooks/manager.go`
- Create: `internal/hooks/manager_test.go`

- [ ] **Step C7.1: Write failing tests**

Create `internal/hooks/manager_test.go`:

```go
package hooks

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func mgr(t *testing.T, cfg Config) *Manager {
	t.Helper()
	require.Empty(t, cfg.Validate())
	return NewManager(cfg, ManagerOptions{
		SessionID: "test-session",
		Cwd:       "/test",
		Version:   "0.4.0-dev",
	})
}

func TestManager_FirePreToolUse_MatchAndAllow(t *testing.T) {
	m := mgr(t, Config{
		PreToolUse: []Spec{
			{Matcher: "Bash", Command: filepath.Join("testdata", "allow.sh"), Timeout: 3 * time.Second},
			{Matcher: "Write", Command: filepath.Join("testdata", "deny.sh"), Timeout: 3 * time.Second},
		},
	})
	dec := m.FirePreToolUse(context.Background(), "Bash", map[string]any{"command": "ls"})
	require.Equal(t, DecisionAllow, dec.Behavior)
	require.Empty(t, dec.Reason)
	require.Nil(t, dec.ModifiedInput)
}

func TestManager_FirePreToolUse_DenyShortCircuits(t *testing.T) {
	m := mgr(t, Config{
		PreToolUse: []Spec{
			{Matcher: "Bash", Command: filepath.Join("testdata", "deny.sh"), Timeout: 3 * time.Second},
			{Matcher: "Bash", Command: filepath.Join("testdata", "allow.sh"), Timeout: 3 * time.Second},
		},
	})
	dec := m.FirePreToolUse(context.Background(), "Bash", map[string]any{"command": "ls"})
	require.Equal(t, DecisionDeny, dec.Behavior)
	require.Contains(t, dec.Reason, "denied by policy")
}

func TestManager_FirePreToolUse_ModifiedInputApplied(t *testing.T) {
	m := mgr(t, Config{
		PreToolUse: []Spec{
			{Matcher: "Bash", Command: filepath.Join("testdata", "mutate-input.sh"), Timeout: 3 * time.Second},
		},
	})
	dec := m.FirePreToolUse(context.Background(), "Bash", map[string]any{"command": "rm -rf /"})
	require.NotNil(t, dec.ModifiedInput)
	require.Equal(t, "ls -al", dec.ModifiedInput["command"])
}

func TestManager_FirePreToolUse_NoMatchersNoHooks(t *testing.T) {
	m := mgr(t, Config{})
	dec := m.FirePreToolUse(context.Background(), "Bash", nil)
	require.Equal(t, DecisionPass, dec.Behavior)
}

func TestManager_FireUserPromptSubmit_AdditionalContextConcat(t *testing.T) {
	m := mgr(t, Config{
		UserPromptSubmit: []Spec{
			{Command: filepath.Join("testdata", "inject-context.sh"), Timeout: 3 * time.Second},
			{Command: filepath.Join("testdata", "inject-context.sh"), Timeout: 3 * time.Second},
		},
	})
	ctx, finalPrompt, abort, reason := m.FireUserPromptSubmit(context.Background(), "hello")
	_ = ctx
	require.False(t, abort)
	require.Empty(t, reason)
	require.Equal(t, "hello\n\ninjected ctx\n\ninjected ctx", finalPrompt)
}

func TestManager_FireUserPromptSubmit_DenyAborts(t *testing.T) {
	m := mgr(t, Config{
		UserPromptSubmit: []Spec{
			{Command: filepath.Join("testdata", "deny.sh"), Timeout: 3 * time.Second},
		},
	})
	_, _, abort, reason := m.FireUserPromptSubmit(context.Background(), "hello")
	require.True(t, abort)
	require.Contains(t, reason, "denied by policy")
}

func TestManager_FireStopAsync_NonBlocking(t *testing.T) {
	m := mgr(t, Config{
		Stop: []Spec{
			{Command: filepath.Join("testdata", "slow.sh"), Timeout: 100 * time.Millisecond},
		},
	})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		m.FireStop(context.Background(), "end_turn")
	}()
	// Should return without blocking on the slow hook because it's async fire-and-forget.
	wg.Wait()
}

func TestManager_FirePreToolUse_Timeout_ReturnsDeny(t *testing.T) {
	m := mgr(t, Config{
		PreToolUse: []Spec{
			{Matcher: "Bash", Command: filepath.Join("testdata", "slow.sh"), Timeout: 100 * time.Millisecond},
		},
	})
	dec := m.FirePreToolUse(context.Background(), "Bash", nil)
	require.Equal(t, DecisionDeny, dec.Behavior)
	require.Contains(t, dec.Reason, "timeout")
}
```

- [ ] **Step C7.2: Run — expect FAIL**

```bash
go test ./internal/hooks/... -run TestManager_
```

Expected: build failure on `Manager`, `NewManager`, `ManagerOptions`, `Decision`, `DecisionAllow`, etc.

- [ ] **Step C7.3: Implement decision.go**

Create `internal/hooks/decision.go`:

```go
package hooks

// Behavior is what the gate should do based on the combined PreToolUse hook results.
type Behavior int

const (
	// DecisionPass means hooks made no decision; the gate continues its normal flow.
	DecisionPass Behavior = iota
	// DecisionAllow means at least one hook explicitly allowed and none denied.
	DecisionAllow
	// DecisionDeny means at least one hook explicitly denied (or exited 2 / timed out).
	DecisionDeny
)

// Decision is what FirePreToolUse returns to the permission gate.
type Decision struct {
	Behavior      Behavior
	Reason        string
	ModifiedInput map[string]any
}
```

- [ ] **Step C7.4: Implement manager.go**

Create `internal/hooks/manager.go`:

```go
package hooks

import (
	"context"
	"fmt"
	"strings"
)

// ManagerOptions are values that don't change between fires.
type ManagerOptions struct {
	SessionID string
	Cwd       string
	Version   string
	LogSink   func(eventName, msg string) // nil-safe
}

// Manager fires hooks in response to events. Stateless aside from config + opts.
type Manager struct {
	cfg  Config
	opts ManagerOptions
}

func NewManager(cfg Config, opts ManagerOptions) *Manager {
	return &Manager{cfg: cfg, opts: opts}
}

func (m *Manager) log(event, msg string) {
	if m.opts.LogSink != nil {
		m.opts.LogSink(event, msg)
	}
}

func (m *Manager) common(name EventName) Common {
	return Common{
		HookEventName: name,
		SessionID:     m.opts.SessionID,
		Cwd:           m.opts.Cwd,
		Version:       m.opts.Version,
	}
}

func (m *Manager) matchingHooks(list []Spec, toolName string) []Spec {
	var out []Spec
	for _, s := range list {
		if s.matcherRE == nil || s.matcherRE.MatchString(toolName) {
			out = append(out, s)
		}
	}
	return out
}

// FirePreToolUse runs every matching PreToolUse hook in order.
// First hook that denies (output, exit 2, or timeout) short-circuits the chain.
// ModifiedInput from hooks is merged left-to-right.
func (m *Manager) FirePreToolUse(ctx context.Context, toolName string, input map[string]any) Decision {
	hooks := m.matchingHooks(m.cfg.PreToolUse, toolName)
	if len(hooks) == 0 {
		return Decision{Behavior: DecisionPass}
	}
	d := Decision{Behavior: DecisionPass}
	cur := input
	for _, h := range hooks {
		payload := PreToolUsePayload{Common: m.common(EventPreToolUse), ToolName: toolName, ToolInput: cur}
		r, err := RunHook(ctx, h, payload)
		if err != nil {
			m.log("PreToolUse", fmt.Sprintf("%s: setup error: %v", h.Command, err))
			continue
		}
		if r.TimedOut {
			return Decision{Behavior: DecisionDeny, Reason: fmt.Sprintf("hook %s timeout", h.Command)}
		}
		if len(r.Stderr) > 0 {
			m.log("PreToolUse", strings.TrimSpace(string(r.Stderr)))
		}
		if r.ExitCode == 2 {
			return Decision{Behavior: DecisionDeny, Reason: strings.TrimSpace(string(r.Stderr))}
		}
		if r.ExitCode != 0 {
			// non-zero non-2 → deny (safety bias)
			return Decision{Behavior: DecisionDeny, Reason: fmt.Sprintf("hook %s exited %d", h.Command, r.ExitCode)}
		}
		// exit 0 — inspect JSON
		if r.Output != nil && r.Output.HookSpecificOutput != nil {
			hso := r.Output.HookSpecificOutput
			if hso.ModifiedInput != nil {
				cur = hso.ModifiedInput
				d.ModifiedInput = cur
			}
			switch hso.PermissionDecision {
			case "deny":
				return Decision{Behavior: DecisionDeny, Reason: hso.PermissionDecisionReason}
			case "allow":
				d.Behavior = DecisionAllow
				if d.Reason == "" {
					d.Reason = hso.PermissionDecisionReason
				}
			}
		}
	}
	return d
}

// FirePostToolUse runs PostToolUse hooks. Errors are logged but never block.
// Returns concatenated additionalContext (if any hook injected) to append to tool_result.
func (m *Manager) FirePostToolUse(ctx context.Context, toolName string, input, response map[string]any) string {
	hooks := m.matchingHooks(m.cfg.PostToolUse, toolName)
	if len(hooks) == 0 {
		return ""
	}
	var extras []string
	for _, h := range hooks {
		payload := PostToolUsePayload{Common: m.common(EventPostToolUse), ToolName: toolName, ToolInput: input, ToolResponse: response}
		r, err := RunHook(ctx, h, payload)
		if err != nil {
			m.log("PostToolUse", fmt.Sprintf("%s: setup error: %v", h.Command, err))
			continue
		}
		if r.TimedOut {
			m.log("PostToolUse", fmt.Sprintf("%s: timeout", h.Command))
			continue
		}
		if len(r.Stderr) > 0 {
			m.log("PostToolUse", strings.TrimSpace(string(r.Stderr)))
		}
		if r.Output != nil && r.Output.HookSpecificOutput != nil && r.Output.HookSpecificOutput.AdditionalContext != "" {
			extras = append(extras, r.Output.HookSpecificOutput.AdditionalContext)
		}
	}
	return strings.Join(extras, "\n")
}

// FireUserPromptSubmit runs every UserPromptSubmit hook. Returns:
//   - finalPrompt: prompt + concatenated additionalContext blocks
//   - abort: true if any hook denied (exit 2)
//   - reason: stderr from the denying hook
func (m *Manager) FireUserPromptSubmit(ctx context.Context, prompt string) (context.Context, string, bool, string) {
	hooks := m.cfg.UserPromptSubmit
	if len(hooks) == 0 {
		return ctx, prompt, false, ""
	}
	parts := []string{prompt}
	for _, h := range hooks {
		payload := UserPromptSubmitPayload{Common: m.common(EventUserPromptSubmit), Prompt: prompt}
		r, err := RunHook(ctx, h, payload)
		if err != nil {
			m.log("UserPromptSubmit", fmt.Sprintf("%s: setup error: %v", h.Command, err))
			continue
		}
		if r.TimedOut {
			return ctx, "", true, fmt.Sprintf("hook %s timeout", h.Command)
		}
		if len(r.Stderr) > 0 {
			m.log("UserPromptSubmit", strings.TrimSpace(string(r.Stderr)))
		}
		if r.ExitCode == 2 {
			return ctx, "", true, strings.TrimSpace(string(r.Stderr))
		}
		if r.ExitCode != 0 {
			m.log("UserPromptSubmit", fmt.Sprintf("%s exited %d", h.Command, r.ExitCode))
			continue
		}
		if r.Output != nil && r.Output.HookSpecificOutput != nil && r.Output.HookSpecificOutput.AdditionalContext != "" {
			parts = append(parts, r.Output.HookSpecificOutput.AdditionalContext)
		}
	}
	return ctx, strings.Join(parts, "\n\n"), false, ""
}

// FireStop / FireSubagentStop / FireNotification / FirePreCompact / FireSessionStart / FireSessionEnd
// All return immediately; the hook runs in a background goroutine.

func (m *Manager) FireStop(ctx context.Context, reason string) {
	m.fireAsync(ctx, m.cfg.Stop, "Stop", StopPayload{Common: m.common(EventStop), StopReason: reason})
}

func (m *Manager) FireSubagentStop(ctx context.Context, reason string) {
	m.fireAsync(ctx, m.cfg.SubagentStop, "SubagentStop", StopPayload{Common: m.common(EventSubagentStop), StopReason: reason})
}

func (m *Manager) FireNotification(ctx context.Context, message, kind string) {
	m.fireAsync(ctx, m.cfg.Notification, "Notification", NotificationPayload{Common: m.common(EventNotification), Message: message, Kind: kind})
}

func (m *Manager) FirePreCompact(ctx context.Context, trigger string) {
	// PreCompact is sync but only logs (per spec §5.2). Caller waits.
	for _, h := range m.cfg.PreCompact {
		payload := PreCompactPayload{Common: m.common(EventPreCompact), Trigger: trigger}
		r, err := RunHook(ctx, h, payload)
		if err != nil {
			m.log("PreCompact", fmt.Sprintf("%s: setup error: %v", h.Command, err))
			continue
		}
		if len(r.Stderr) > 0 {
			m.log("PreCompact", strings.TrimSpace(string(r.Stderr)))
		}
	}
}

func (m *Manager) FireSessionStart(ctx context.Context, kind string) {
	m.fireAsync(ctx, m.cfg.SessionStart, "SessionStart", SessionStartPayload{Common: m.common(EventSessionStart), Kind: kind})
}

func (m *Manager) FireSessionEnd(ctx context.Context, kind string) {
	m.fireAsync(ctx, m.cfg.SessionEnd, "SessionEnd", SessionEndPayload{Common: m.common(EventSessionEnd), Kind: kind})
}

func (m *Manager) fireAsync(ctx context.Context, hooks []Spec, label string, payload any) {
	if len(hooks) == 0 {
		return
	}
	go func() {
		for _, h := range hooks {
			r, err := RunHook(ctx, h, payload)
			if err != nil {
				m.log(label, fmt.Sprintf("%s: setup error: %v", h.Command, err))
				continue
			}
			if r.TimedOut {
				m.log(label, fmt.Sprintf("%s: timeout", h.Command))
				continue
			}
			if len(r.Stderr) > 0 {
				m.log(label, strings.TrimSpace(string(r.Stderr)))
			}
		}
	}()
}
```

- [ ] **Step C7.5: Run — expect PASS**

```bash
go test ./internal/hooks/... -run TestManager_
```

Expected: 8 PASS.

- [ ] **Step C7.6: Stage**

```bash
git add internal/hooks/manager.go internal/hooks/manager_test.go internal/hooks/decision.go
```

---

## Task C8: Full unit sweep + race

- [ ] **Step C8.1: Sweep**

```bash
go test -race -count=3 ./internal/hooks/...
```

Expected: all PASS three times in a row, race clean.

- [ ] **Step C8.2: Vet**

```bash
go vet ./internal/hooks/...
```

Expected: clean.

---

# Phase D — Wire into permission gate + query loop + TUI + headless (5 tasks)

## Task D9: Permissions.Context gains a hook-Fire callback

**Files:**
- Modify: `pkg/permissions/gate.go`
- Modify: `pkg/permissions/context.go` (if it exists; else gate.go)
- Modify: `pkg/permissions/gate_test.go`

The constraint: `pkg/permissions` cannot import `internal/hooks` (would create an import cycle through `cmd/anthrogo`). Solution: add a function-typed field on `permissions.Context` that `cmd/anthrogo` sets at startup.

- [ ] **Step D9.1: Find current Context definition**

```bash
grep -n "type Context " pkg/permissions/*.go
```

Note the file and line.

- [ ] **Step D9.2: Add HookDecide callback to Context**

Add to the Context struct:

```go
// HookDecide is called by Decide before consulting any rule. nil-safe.
// Implementations: cmd/anthrogo wires hooks.Manager.FirePreToolUse here.
HookDecide func(toolName string, input map[string]any) HookOutcome
```

Add a new type in the same file:

```go
// HookOutcome is the gate-visible projection of a hook PreToolUse decision.
type HookOutcome struct {
    Pass          bool             // true = hooks said nothing; continue normal flow
    Allow         bool             // hook allowed (only meaningful when Pass=false)
    Deny          bool             // hook denied (only meaningful when Pass=false)
    Reason        string
    ModifiedInput map[string]any
}
```

- [ ] **Step D9.3: Update Decide() to consult hooks first**

In `Decide(c *Context, toolName string, input map[string]any) Decision`:

After the bypass check, BEFORE the existing rule lookup, add:

```go
if c.HookDecide != nil {
    out := c.HookDecide(toolName, input)
    if out.Deny {
        return Decision{Behavior: BehaviorDeny, Reason: out.Reason, ModifiedInput: out.ModifiedInput}
    }
    if out.Allow {
        // Plan-mode write-tools still override hook allow.
        if c.Mode == ModePlan && isPlanLockedTool(toolName, input) {
            return Decision{Behavior: BehaviorDeny, Reason: "plan mode blocks " + toolName}
        }
        return Decision{Behavior: BehaviorAllow, Reason: out.Reason, ModifiedInput: out.ModifiedInput}
    }
    // out.Pass → fall through; remember ModifiedInput for any downstream rule
    if out.ModifiedInput != nil {
        input = out.ModifiedInput
    }
}
```

And add `ModifiedInput map[string]any` to the `Decision` struct.

- [ ] **Step D9.4: Add a test**

Append to `pkg/permissions/gate_test.go`:

```go
func TestGate_HookDeny_ShortCircuits(t *testing.T) {
    c := ctx()
    c.AlwaysAllowRules[SourceUser] = []Rule{{Tool: "Bash"}}
    c.HookDecide = func(string, map[string]any) HookOutcome {
        return HookOutcome{Deny: true, Reason: "secret pattern"}
    }
    d := Decide(c, "Bash", map[string]any{"command": "echo secret"})
    require.Equal(t, BehaviorDeny, d.Behavior)
    require.Contains(t, d.Reason, "secret pattern")
}

func TestGate_HookAllow_OverridesAsk(t *testing.T) {
    c := ctx()
    c.HookDecide = func(string, map[string]any) HookOutcome {
        return HookOutcome{Allow: true, Reason: "hook ok"}
    }
    d := Decide(c, "Bash", map[string]any{"command": "ls"})
    require.Equal(t, BehaviorAllow, d.Behavior)
}

func TestGate_HookAllow_DoesNotUnlockPlanModeWriteTool(t *testing.T) {
    c := ctx()
    c.Mode = ModePlan
    c.HookDecide = func(string, map[string]any) HookOutcome {
        return HookOutcome{Allow: true}
    }
    d := Decide(c, "Write", map[string]any{"path": "/x"})
    require.Equal(t, BehaviorDeny, d.Behavior)
}

func TestGate_HookModifiedInput_Visible(t *testing.T) {
    c := ctx()
    c.HookDecide = func(string, map[string]any) HookOutcome {
        return HookOutcome{Pass: true, ModifiedInput: map[string]any{"command": "ls -al"}}
    }
    d := Decide(c, "Bash", map[string]any{"command": "ls"})
    require.Equal(t, "ls -al", d.ModifiedInput["command"])
}
```

- [ ] **Step D9.5: Run**

```bash
go test ./pkg/permissions/...
```

Expected: all PASS, including 4 new tests.

- [ ] **Step D9.6: Stage**

```bash
git add pkg/permissions/
```

---

## Task D10: PostToolUse + Stop wiring in query loop

**Files:**
- Modify: `pkg/query/loop.go` (or wherever the turn loop is — verify with `grep -rn "Stop\|tool_use" pkg/query/`)
- Modify: `pkg/query/Engine` (add Hooks field to Config)
- Modify: `pkg/query` tests as needed

- [ ] **Step D10.1: Add Hooks field to query.Config**

In `pkg/query/loop.go` (or wherever `type Config struct` is defined), add:

```go
// Hooks is consulted at PostToolUse and turn-end (Stop) if non-nil.
// Type is opaque to query — set via interface so we don't import hooks here.
Hooks HookSink
```

And define `HookSink` in the same file:

```go
// HookSink is the projection of internal/hooks.Manager that the engine uses.
// Defined here so pkg/query doesn't import internal/hooks.
type HookSink interface {
    FirePostToolUse(ctx context.Context, toolName string, input, response map[string]any) string
    FireStop(ctx context.Context, reason string)
}
```

- [ ] **Step D10.2: Wire FirePostToolUse**

Find the place where tool execution returns a Result and the engine appends a tool_result block to messages. Right before that append, if `e.cfg.Hooks != nil`, call `e.cfg.Hooks.FirePostToolUse(ctx, toolName, input, responseMap)`. The returned string, if non-empty, gets appended to the tool_result text with a leading `"\n\n"`.

- [ ] **Step D10.3: Wire FireStop**

Find the place where the engine detects `end_turn` / closes the turn channel. Add (before the channel close):

```go
if e.cfg.Hooks != nil {
    e.cfg.Hooks.FireStop(ctx, "end_turn")
}
```

- [ ] **Step D10.4: Verify existing tests pass**

```bash
go test ./pkg/query/... -count=1
```

Expected: PASS (we didn't break anything; Hooks is nil-safe).

- [ ] **Step D10.5: Stage**

```bash
git add pkg/query/
```

---

## Task D11: UserPromptSubmit wiring in TUI + headless

**Files:**
- Modify: `internal/tui/app.go`
- Modify: `internal/headless/runner.go`

- [ ] **Step D11.1: Add Hooks option to tui.Options + headless.Options**

In `internal/tui/app.go`:

```go
type Options struct {
    /* existing */
    Hooks PromptHookSink
}

type PromptHookSink interface {
    FireUserPromptSubmit(ctx context.Context, prompt string) (context.Context, string, bool, string)
    FireSessionStart(ctx context.Context, kind string)
    FireSessionEnd(ctx context.Context, kind string)
    FireNotification(ctx context.Context, message, kind string)
}
```

Same `PromptHookSink` in `internal/headless/runner.go` (re-declare, or move to a shared package — simpler to re-declare to keep the import graph flat).

- [ ] **Step D11.2: TUI prompt submit handler**

Find the existing prompt-submit path in `app.go` (look for where Enter triggers the message-submit). Before submitting, if `a.opts.Hooks != nil`:

```go
_, finalPrompt, abort, reason := a.opts.Hooks.FireUserPromptSubmit(ctx, raw)
if abort {
    a.chat.appendError("user prompt blocked: " + reason)
    return
}
raw = finalPrompt
```

- [ ] **Step D11.3: Headless prompt path**

In `headless/runner.go`, where the prompt arg is parsed into `prompt`, do the same FireUserPromptSubmit call. If abort, return an error.

- [ ] **Step D11.4: SessionStart / SessionEnd**

In `tui.New`, after construction, call `opts.Hooks.FireSessionStart(ctx, "new"|"resume")` if non-nil. (`"resume"` if `len(opts.InitialMessages) > 0`.) Same in `headless.Run`.

For SessionEnd, add `defer a.opts.Hooks.FireSessionEnd(...)` in the program-run wrapper inside `cmd/anthrogo/main.go` (wired in D12).

- [ ] **Step D11.5: Notification**

In `internal/tui/app.go`, where the permission modal is raised (search for `permissionAsk`), call `Hooks.FireNotification(ctx, "permission ask: "+toolName, "permission_ask")` if non-nil.

- [ ] **Step D11.6: Stage**

```bash
git add internal/tui/app.go internal/headless/runner.go
```

---

## Task D12: cmd/anthrogo glue

**Files:**
- Modify: `cmd/anthrogo/main.go`

- [ ] **Step D12.1: Build Manager and inject everywhere**

After config load + perms construction, add:

```go
warnings := cfg.Hooks.Validate()
for _, w := range warnings {
    fmt.Fprintln(os.Stderr, "hooks:", w)
}
hookMgr := hooks.NewManager(cfg.Hooks, hooks.ManagerOptions{
    SessionID: sess.ID(),
    Cwd:       cwd,
    Version:   version.Version,
    LogSink: func(event, msg string) {
        if logSinkRef.Load() != nil {
            (*logSinkRef.Load())("hook:"+event, msg)
            return
        }
        fmt.Fprintf(os.Stderr, "[hook:%s] %s\n", event, msg)
    },
})

// Wire into permissions
perms.HookDecide = func(toolName string, input map[string]any) permissions.HookOutcome {
    d := hookMgr.FirePreToolUse(context.Background(), toolName, input)
    switch d.Behavior {
    case hooks.DecisionAllow:
        return permissions.HookOutcome{Allow: true, Reason: d.Reason, ModifiedInput: d.ModifiedInput}
    case hooks.DecisionDeny:
        return permissions.HookOutcome{Deny: true, Reason: d.Reason, ModifiedInput: d.ModifiedInput}
    default:
        return permissions.HookOutcome{Pass: true, ModifiedInput: d.ModifiedInput}
    }
}
```

Inject hookMgr into headless.Options.Hooks, tui.Options.Hooks, and into `query.Config.Hooks` via the existing wiring in tui.New (it already builds a `query.Engine` — pass the hookMgr through).

- [ ] **Step D12.2: SessionEnd**

Add `defer hookMgr.FireSessionEnd(context.Background(), "user_quit")` after construction.

- [ ] **Step D12.3: Build + test**

```bash
go build ./...
go test ./...
```

Expected: clean.

- [ ] **Step D12.4: Stage**

```bash
git add cmd/anthrogo/main.go
```

---

## Task D13: TUI hook log lines

**Files:**
- Modify: `internal/tui/app.go`
- Modify: `internal/tui/chat.go`

- [ ] **Step D13.1: Hook log → chat dim line**

Hook log_sink already routes through `logSinkRef` (set up in cmd D12.1). It will call `app.AppendServerLog("hook:PreToolUse", msg)` which renders dim via existing chat.appendServerLog. No new code needed beyond what D12.1 wired.

Verify by reading the existing AppendServerLog path — it should just work because we prefix the event with `"hook:"` and the chat function doesn't care about the prefix string.

- [ ] **Step D13.2: Stage**

If you needed to change anything, `git add internal/tui/`. Otherwise nothing.

---

# Phase E — Debt cleanup + acceptance (3 tasks)

## Task E14: chat AppendServerLog race regression test (debt #8)

**Files:**
- Create: `internal/tui/chat_concurrent_test.go`

- [ ] **Step E14.1: Test**

Create `internal/tui/chat_concurrent_test.go`:

```go
package tui

import (
    "sync"
    "testing"

    tea "github.com/charmbracelet/bubbletea"
)

// TestApp_AppendServerLog_ConcurrentSafe verifies that AppendServerLog
// can be called from many goroutines while the tea program runs, without
// triggering a -race detector failure. Regression test for the MCP review fix.
func TestApp_AppendServerLog_ConcurrentSafe(t *testing.T) {
    app := New(Options{})
    // Note: we don't actually Run() the program — just call SetProgram with
    // a nil-safe value and rely on the fallback path.
    var wg sync.WaitGroup
    for i := 0; i < 50; i++ {
        wg.Add(1)
        go func(n int) {
            defer wg.Done()
            app.AppendServerLog("srv", "msg")
        }(i)
    }
    wg.Wait()
    _ = tea.KeyMsg{} // keep tea import live if we need it later
}
```

- [ ] **Step E14.2: Run with race**

```bash
go test -race ./internal/tui/... -run TestApp_AppendServerLog_ConcurrentSafe -count=3
```

Expected: PASS three times.

- [ ] **Step E14.3: Stage**

```bash
git add internal/tui/chat_concurrent_test.go
```

---

## Task E15: Server.Start state-reset regression test (debt #9)

**Files:**
- Modify: `internal/mcp/manager_test.go` (or create `internal/mcp/server_test.go`)

- [ ] **Step E15.1: Test**

Append to `internal/mcp/manager_test.go`:

```go
func TestServer_Start_ResetsStateAfterClose(t *testing.T) {
    bin := buildEchoServer(t)
    s := NewServer("echo", MCPServerConfig{Command: bin, Timeout: 30 * time.Second}, nil)
    require.NoError(t, s.Start(context.Background()))
    require.Equal(t, StateReady, s.State())

    require.NoError(t, s.Close())
    require.Equal(t, StateClosed, s.State())

    // Re-Start the same server — state must reset to StateReady.
    require.NoError(t, s.Start(context.Background()))
    require.Equal(t, StateReady, s.State())
    require.Nil(t, s.Err())
    require.NoError(t, s.Close())
}
```

- [ ] **Step E15.2: Run**

```bash
go test ./internal/mcp/... -run TestServer_Start_ResetsStateAfterClose -count=3
```

Expected: PASS three times.

- [ ] **Step E15.3: Stage**

```bash
git add internal/mcp/manager_test.go
```

---

## Task E16: Full acceptance sweep + version bump + docs

**Files:**
- Modify: `internal/version/version.go`
- Modify: `CHANGELOG.md`
- Modify: `README.md`

- [ ] **Step E16.1: Bump version**

`internal/version/version.go`:

```go
var Version = "0.4.0-dev"
```

- [ ] **Step E16.2: CHANGELOG**

Prepend after `# Changelog`:

```markdown
## [0.4.0-dev] — 2026-05-20

M4.1 — Hooks (9 event types: PreToolUse / PostToolUse / UserPromptSubmit / Stop / SubagentStop / Notification / PreCompact / SessionStart / SessionEnd).

### Added
- `internal/hooks/` package: Config, Event payloads, Runner, Manager, Decision.
- `hooks:` YAML stanza in `settings.yaml` (per-event lists with matcher + command + timeout).
- JSON-over-stdin / JSON-on-stdout / exit-code-2-blocks protocol matching upstream claude-code@2.1.88.
- Permission gate consults `PreToolUse` hooks before rule lookup; hooks can allow / deny / mutate input.
- Plan-mode hard-lock still overrides hook-allow for write tools.
- `PostToolUse` hooks can append `additionalContext` to tool_result text.
- `UserPromptSubmit` hooks can inject context or abort the prompt (exit 2).
- Async fire-and-forget: `Stop` / `SubagentStop` / `Notification` / `SessionStart` / `SessionEnd`.
- Sync but log-only: `PreCompact` (wires to M4.2 `/compact`).
- TUI dim-styled `[hook:<event>] <msg>` log lines via the existing logSink rail.
- `chat_concurrent_test.go` race-regression test for AppendServerLog.
- `Server.Start` state-reset regression test (covers `/mcp reload`).

### Changed
- `permissions.Context` gains `HookDecide func(toolName, input) HookOutcome` (nil-safe).
- `permissions.Decision` gains `ModifiedInput map[string]any`.
- `query.Config` gains `Hooks HookSink` interface (PostToolUse, Stop).
- `tui.Options` and `headless.Options` gain `Hooks PromptHookSink` interface.

### Known issues / deferred
- `SubagentStop` payload is wired but never fires (no subagents until M5).
- `PreCompact` is wired but `/compact` itself is still a placeholder (M4.2 lands real compaction).
- Hook processes run unsandboxed in the user's privilege; M5 plugin model will introduce a sandbox.
```

- [ ] **Step E16.3: README**

Add a "Hooks" section between "MCP servers" and "Tools (M1)":

```markdown
## Hooks

anthrogo runs user-defined shell commands at 9 lifecycle events. Add to `~/.anthrogo/settings.yaml`:

\`\`\`yaml
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
\`\`\`

Each hook gets one JSON object on stdin describing the event. Exit code 2 blocks the action (PreToolUse → deny, UserPromptSubmit → abort prompt). Exit code 0 + JSON on stdout can `permissionDecision: "allow"|"deny"`, `modifiedInput: {...}` (PreToolUse), or `additionalContext: "..."` (UserPromptSubmit / PostToolUse).

Matcher is a Go regexp against the tool name (PreToolUse / PostToolUse only). Project-level `.anthrogo/hooks.yaml` appends to home-level `hooks:` block.

Default timeouts: 30s for blocking events, 5–10s for async. Async events (Stop / Notification / Session*) fire-and-forget on a background goroutine.

Plan-mode hard-lock still overrides hook-allow for write tools.
```

(Use real triple-backticks; this README snippet uses escaped ones for the plan only.)

- [ ] **Step E16.4: Sweep**

```bash
make build && ./bin/anthrogo --version
go build ./...
go vet ./...
go test ./...
go test -race ./pkg/query ./pkg/tool ./internal/tui ./internal/session ./pkg/command ./internal/mcp ./internal/system ./internal/hooks ./pkg/permissions ./pkg/command/builtins
```

Expected: every line clean, version = `anthrogo 0.4.0-dev`.

- [ ] **Step E16.5: 3× uncached full-repo runs**

```bash
for i in 1 2 3; do go clean -testcache; go test ./... 2>&1 | grep -E "FAIL|^FAIL"; done
```

Expected: no output (no FAILs across 3 runs).

- [ ] **Step E16.6: Stage**

```bash
git add CHANGELOG.md README.md internal/version/version.go
```

---

## Self-review

**1. Spec coverage**

| Spec section | Tasks |
|---|---|
| §2 9-event scope | A2 (consts) + C7 (dispatch) + D9-D13 (wiring) |
| §3 Config + matcher + overlay + expansion | A1 |
| §4.1 Input payload | A2 |
| §4.2 Exit codes | B4, B5, C7 |
| §4.3 Output JSON | A2, B5 (parse) |
| §4.4 Per-event policy | C7 (Fire* per event) |
| §5.1 Concurrency | C7 (sync vs fireAsync), E14 (race test) |
| §5.3 Permission gate integration | D9 |
| §5.4 UserPromptSubmit | C7 + D11 |
| §5.5 PostToolUse | C7 + D10 |
| §6 Code organization | every Phase A-C file |
| §7 Edge cases | B4 (limited writer), B5 (timeout/not-found), A1 (invalid regexp) |
| §8 Testing strategy | A1, A2, B4, B5, C7, C8, E14, E15 |
| §9 Debt | E14, E15 |
| §10 CHANGELOG / version | E16 |
| §11 Acceptance | E16 |

**2. Placeholder scan**

- D11 has "find the existing prompt-submit path" / "find the place where Enter triggers" — these are read-the-code instructions, not placeholders. The implementer subagent must do this grep before editing.
- D10 has "find the place where tool execution returns a Result" — same. Both files are well-known (`internal/tui/app.go` and `pkg/query/loop.go`).
- D13 is mostly a verify-no-code-needed task. Acceptable.
- No "TBD" / "TODO" / "fill in" / "similar to" anywhere.

**3. Type consistency**

- `Decision.Behavior` (`hooks.Behavior`) → projected to `permissions.HookOutcome.Allow/Deny/Pass` in D9.4 + cmd D12.1.
- `Decision.ModifiedInput` flows through `permissions.Decision.ModifiedInput` (added in D9.3) but consumers in `pkg/query/loop.go` need to actually swap `input` before calling the tool — verify in D10.
- `HookSink` interface in `pkg/query` and `PromptHookSink` interface in `internal/tui` / `internal/headless` both match what `hooks.Manager` provides. The implementer should cross-check by name.

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-05-20-anthrogo-m4-1-hooks.md`. Execution will proceed via `superpowers:subagent-driven-development` (user's prior preference for M1/M2/M3 — continuing the same pattern).

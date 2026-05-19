package permissions

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func ctx() *Context {
	return &Context{
		Mode:             ModeDefault,
		AlwaysAllowRules: RulesBySource{},
		AlwaysDenyRules:  RulesBySource{},
		AlwaysAskRules:   RulesBySource{},
	}
}

func TestGate_BypassMode_AlwaysAllow(t *testing.T) {
	c := ctx()
	c.Mode = ModeBypassPermissions
	c.IsBypassAvailable = true
	d := Decide(c, "Bash", map[string]any{"command": "rm -rf /"})
	require.Equal(t, BehaviorAllow, d.Behavior)
}

func TestGate_DenyTrumpsAllow(t *testing.T) {
	c := ctx()
	c.AlwaysAllowRules[SourceUser] = []Rule{{Tool: "Bash"}}
	c.AlwaysDenyRules[SourceUser] = []Rule{{Tool: "Bash", Pattern: "rm -rf*"}}
	d := Decide(c, "Bash", map[string]any{"command": "rm -rf /"})
	require.Equal(t, BehaviorDeny, d.Behavior)
}

func TestGate_AllowMatched(t *testing.T) {
	c := ctx()
	c.AlwaysAllowRules[SourceUser] = []Rule{{Tool: "Bash", Pattern: "git status*"}}
	d := Decide(c, "Bash", map[string]any{"command": "git status -s"})
	require.Equal(t, BehaviorAllow, d.Behavior)
}

func TestGate_NoRule_FallsBackToAsk(t *testing.T) {
	c := ctx()
	d := Decide(c, "Bash", map[string]any{"command": "git push"})
	require.Equal(t, BehaviorAsk, d.Behavior)
}

func TestGate_AskAvoided_ReturnsDeny(t *testing.T) {
	c := ctx()
	c.ShouldAvoidPrompts = true
	d := Decide(c, "Bash", map[string]any{"command": "git push"})
	require.Equal(t, BehaviorDeny, d.Behavior)
}

func TestGate_AcceptEdits_AllowsWriteEdit(t *testing.T) {
	c := ctx()
	c.Mode = ModeAcceptEdits
	for _, tool := range []string{"Write", "Edit"} {
		d := Decide(c, tool, map[string]any{"path": "/tmp/x"})
		require.Equalf(t, BehaviorAllow, d.Behavior, "tool=%s", tool)
	}
	// other tools still go to ask
	d := Decide(c, "Bash", map[string]any{"command": "ls"})
	require.Equal(t, BehaviorAsk, d.Behavior)
}

func TestGate_PlanMode_DeniesWriteTools(t *testing.T) {
	c := ctx()
	c.Mode = ModePlan
	for _, tool := range []string{"Write", "Edit", "NotebookEdit"} {
		d := Decide(c, tool, map[string]any{"path": "/x"})
		require.Equalf(t, BehaviorDeny, d.Behavior, "tool=%s", tool)
	}
}

func TestGate_PlanMode_AllowsReadOnlyBash(t *testing.T) {
	c := ctx()
	c.Mode = ModePlan
	c.AlwaysAllowRules[SourceCLI] = []Rule{{Tool: "Bash"}}
	d := Decide(c, "Bash", map[string]any{"command": "git status"})
	require.Equal(t, BehaviorAllow, d.Behavior)
}

func TestGate_PlanMode_DeniesWriteBash(t *testing.T) {
	c := ctx()
	c.Mode = ModePlan
	c.AlwaysAllowRules[SourceCLI] = []Rule{{Tool: "Bash"}}
	d := Decide(c, "Bash", map[string]any{"command": "rm foo"})
	require.Equal(t, BehaviorDeny, d.Behavior)
}

func TestGate_PlanMode_AllowsReadTools(t *testing.T) {
	c := ctx()
	c.Mode = ModePlan
	c.AlwaysAllowRules[SourceCLI] = []Rule{{Tool: "Read"}}
	d := Decide(c, "Read", map[string]any{"file_path": "/x"})
	require.Equal(t, BehaviorAllow, d.Behavior)
}

// Plan mode must hard-lock write tools even when an alwaysAllow rule would
// otherwise let them through. Locks in the reviewer's M2 spec-compliance ask.
func TestGate_PlanMode_HardLocksWriteEvenWithAllowRule(t *testing.T) {
	c := ctx()
	c.Mode = ModePlan
	c.AlwaysAllowRules[SourceCLI] = []Rule{
		{Tool: "Write"},
		{Tool: "Edit"},
		{Tool: "NotebookEdit"},
	}
	for _, tool := range []string{"Write", "Edit", "NotebookEdit"} {
		d := Decide(c, tool, map[string]any{"path": "/x"})
		require.Equalf(t, BehaviorDeny, d.Behavior, "tool=%s should be denied even with allow rule", tool)
	}
}

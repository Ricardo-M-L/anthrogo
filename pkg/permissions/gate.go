package permissions

// Behavior is the gate's verdict.
type Behavior string

const (
	BehaviorAllow Behavior = "allow"
	BehaviorDeny  Behavior = "deny"
	BehaviorAsk   Behavior = "ask"
)

// Decision is the gate's output for one tool invocation.
type Decision struct {
	Behavior      Behavior
	Reason        string
	UpdatedInput  map[string]any
	SuggestedRule *Rule
}

// Decide evaluates a tool invocation against the context's rules.
//
// Order (mirrors src/utils/permissions evaluation):
//  1. bypassPermissions mode → allow always.
//  2. deny rules win unconditionally.
//  3. acceptEdits mode allows Write/Edit/NotebookEdit by default.
//  4. allow rules → allow.
//  5. ask rules → ask (unless ShouldAvoidPrompts).
//  6. fallback → ask (default mode) / deny (ShouldAvoidPrompts).
func Decide(c *Context, tool string, input map[string]any) Decision {
	if c.Mode == ModeBypassPermissions && c.IsBypassAvailable {
		return Decision{Behavior: BehaviorAllow, Reason: "bypassPermissions mode"}
	}
	if c.Mode == ModePlan {
		if IsWriteTool(tool) {
			return Decision{Behavior: BehaviorDeny, Reason: "plan mode: write tools disabled"}
		}
		if tool == "Bash" {
			cmd, _ := input["command"].(string)
			if !IsReadOnlyBashCommand(cmd) {
				return Decision{Behavior: BehaviorDeny, Reason: "plan mode: bash command not in read-only allowlist"}
			}
		}
	}
	if rule, ok := firstMatch(c.AlwaysDenyRules, tool, input); ok {
		return Decision{Behavior: BehaviorDeny, Reason: "matched deny rule", SuggestedRule: rule}
	}
	if c.Mode == ModeAcceptEdits && isEditingTool(tool) {
		return Decision{Behavior: BehaviorAllow, Reason: "acceptEdits mode"}
	}
	if rule, ok := firstMatch(c.AlwaysAllowRules, tool, input); ok {
		return Decision{Behavior: BehaviorAllow, Reason: "matched allow rule", SuggestedRule: rule}
	}
	if rule, ok := firstMatch(c.AlwaysAskRules, tool, input); ok {
		return ask(c, "matched ask rule", rule)
	}
	return ask(c, "no matching rule", nil)
}

func ask(c *Context, reason string, rule *Rule) Decision {
	if c.ShouldAvoidPrompts {
		return Decision{Behavior: BehaviorDeny, Reason: reason + " (prompts disabled)", SuggestedRule: rule}
	}
	return Decision{Behavior: BehaviorAsk, Reason: reason, SuggestedRule: rule}
}

func firstMatch(rules RulesBySource, tool string, input map[string]any) (*Rule, bool) {
	for _, src := range []Source{SourceCLI, SourceManaged, SourceUser, SourceProject} {
		for i := range rules[src] {
			if rules[src][i].Match(tool, input) {
				return &rules[src][i], true
			}
		}
	}
	return nil, false
}

func isEditingTool(t string) bool {
	switch t {
	case "Write", "Edit", "NotebookEdit":
		return true
	}
	return false
}

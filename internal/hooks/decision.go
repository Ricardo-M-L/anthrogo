package hooks

// Behavior describes what the manager decided for a PreToolUse check.
type Behavior int

const (
	// DecisionPass means no hook expressed an opinion; proceed normally.
	DecisionPass Behavior = iota
	// DecisionAllow means at least one hook explicitly allowed the action.
	DecisionAllow
	// DecisionDeny means at least one hook denied the action.
	DecisionDeny
)

// Decision is the result of FirePreToolUse.
type Decision struct {
	Behavior      Behavior
	Reason        string
	ModifiedInput map[string]any
}

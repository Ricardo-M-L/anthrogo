package permissions

// Mode controls how the gate evaluates tool calls.
// Mirrors src/types/permissions.ts PermissionMode.
type Mode string

const (
	ModeDefault           Mode = "default"
	ModePlan              Mode = "plan"
	ModeAcceptEdits       Mode = "acceptEdits"
	ModeBypassPermissions Mode = "bypassPermissions"
)

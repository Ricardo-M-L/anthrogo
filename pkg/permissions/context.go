package permissions

// AdditionalDir is a working directory that's been opted into via --add-dir.
type AdditionalDir struct {
	Path string
}

// HookOutcome is the result returned by HookDecide.
type HookOutcome struct {
	Pass          bool
	Allow         bool
	Deny          bool
	Reason        string
	ModifiedInput map[string]any
}

// Context holds permission state for one conversation.
// Mirrors ToolPermissionContext (src/Tool.ts:123).
type Context struct {
	Mode                         Mode
	AdditionalWorkingDirectories map[string]AdditionalDir
	AlwaysAllowRules             RulesBySource
	AlwaysDenyRules              RulesBySource
	AlwaysAskRules               RulesBySource
	IsBypassAvailable            bool
	ShouldAvoidPrompts           bool
	PrePlanMode                  Mode

	// HookDecide is consulted by Decide before any rule lookup. nil-safe.
	HookDecide func(toolName string, input map[string]any) HookOutcome
}

// Empty returns a Context with all rule maps initialised, in default mode.
func Empty() *Context {
	return &Context{
		Mode:                         ModeDefault,
		AdditionalWorkingDirectories: map[string]AdditionalDir{},
		AlwaysAllowRules:             RulesBySource{},
		AlwaysDenyRules:              RulesBySource{},
		AlwaysAskRules:               RulesBySource{},
	}
}

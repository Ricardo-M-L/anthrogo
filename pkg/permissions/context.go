package permissions

// AdditionalDir is a working directory that's been opted into via --add-dir.
type AdditionalDir struct {
	Path string
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

package permissions

import "context"

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
	// ctx is the per-tool-call context; if the engine cancels the call
	// (Ctrl-C, timeout), HookDecide implementations must propagate it to
	// any subprocess/network they fire (the runtime hook runner already
	// uses exec.CommandContext).
	HookDecide func(ctx context.Context, toolName string, input map[string]any) HookOutcome
}

// Clone returns a shallow copy of c suitable for handing to a subagent.
// RulesBySource maps and HookDecide are shared by reference (treated as
// immutable). Mode, PrePlanMode, ShouldAvoidPrompts, and IsBypassAvailable
// are copied so subagent toggles don't affect the parent.
func (c *Context) Clone() *Context {
	if c == nil {
		return nil
	}
	cp := *c
	return &cp
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

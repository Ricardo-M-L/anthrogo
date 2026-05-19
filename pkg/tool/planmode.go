package tool

import (
	"context"

	"github.com/ricardo/anthrogo/pkg/permissions"
)

type EnterPlanMode struct{ DefaultPermission }

func (EnterPlanMode) Name() string                         { return "EnterPlanMode" }
func (EnterPlanMode) Description(context.Context) string   { return enterPlanModeDescription }
func (EnterPlanMode) UserFacingName(map[string]any) string { return "EnterPlanMode" }
func (EnterPlanMode) IsReadOnly() bool                     { return true }
func (EnterPlanMode) IsConcurrencySafe() bool              { return false }
func (EnterPlanMode) Schema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}

func (EnterPlanMode) Call(_ context.Context, _ map[string]any, tcx *Context) (Result, error) {
	if tcx == nil || tcx.Permissions == nil {
		return errResult("no permission context available"), nil
	}
	tcx.Permissions.PrePlanMode = tcx.Permissions.Mode
	tcx.Permissions.Mode = permissions.ModePlan
	msg := "entered plan mode — write tools are now disabled until ExitPlanMode is called"
	return Result{Type: ResultText, Text: msg, ForLLM: msg}, nil
}

type ExitPlanMode struct{ DefaultPermission }

func (ExitPlanMode) Name() string                         { return "ExitPlanMode" }
func (ExitPlanMode) Description(context.Context) string   { return exitPlanModeDescription }
func (ExitPlanMode) UserFacingName(map[string]any) string { return "ExitPlanMode" }
func (ExitPlanMode) IsReadOnly() bool                     { return true }
func (ExitPlanMode) IsConcurrencySafe() bool              { return false }
func (ExitPlanMode) Schema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}

func (ExitPlanMode) Call(_ context.Context, _ map[string]any, tcx *Context) (Result, error) {
	if tcx == nil || tcx.Permissions == nil {
		return errResult("no permission context available"), nil
	}
	prev := tcx.Permissions.PrePlanMode
	if prev == "" {
		prev = permissions.ModeDefault
	}
	tcx.Permissions.Mode = prev
	tcx.Permissions.PrePlanMode = ""
	msg := "exited plan mode — execution permission restored to " + string(prev)
	return Result{Type: ResultText, Text: msg, ForLLM: msg}, nil
}

const enterPlanModeDescription = `Switch the conversation into plan mode. While plan mode is active, the permission gate blocks any modifying action (Write/Edit/NotebookEdit, non-read-only Bash). Use plan mode to investigate, then call ExitPlanMode before executing.`
const exitPlanModeDescription = `Exit plan mode. The permission mode is restored to whatever it was before plan mode was entered.`

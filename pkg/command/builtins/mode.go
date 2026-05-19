package builtins

import (
	"context"
	"strings"

	"github.com/ricardo/anthrogo/pkg/command"
	"github.com/ricardo/anthrogo/pkg/permissions"
)

type Mode struct{}

func (Mode) Name() string        { return "/mode" }
func (Mode) Aliases() []string   { return nil }
func (Mode) Description() string { return "Set permission mode: plan | default | acceptEdits | bypass" }
func (Mode) Type() command.Type  { return command.TypeLocal }

func (Mode) Run(_ context.Context, args string, host command.Host) (command.Result, error) {
	raw := strings.TrimSpace(args)
	if raw == "bypass" { // accept short alias matching the description
		raw = "bypassPermissions"
	}
	want := permissions.Mode(raw)
	switch want {
	case permissions.ModeDefault, permissions.ModePlan, permissions.ModeAcceptEdits, permissions.ModeBypassPermissions:
	default:
		return command.Result{Text: "usage: /mode <default|plan|acceptEdits|bypass>"}, nil
	}
	host.Permissions().Mode = want
	if want == permissions.ModeBypassPermissions {
		host.Permissions().IsBypassAvailable = true
	}
	return command.Result{Text: "permission mode: " + string(want)}, nil
}

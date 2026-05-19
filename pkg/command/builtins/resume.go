package builtins

import (
	"context"
	"fmt"
	"strings"

	"github.com/ricardo/anthrogo/internal/session"
	"github.com/ricardo/anthrogo/pkg/command"
)

type Resume struct{}

func (Resume) Name() string        { return "/resume" }
func (Resume) Aliases() []string   { return nil }
func (Resume) Description() string { return "Resume an earlier session (prefix-match by id)" }
func (Resume) Type() command.Type  { return command.TypeLocal }

func (Resume) Run(_ context.Context, args string, host command.Host) (command.Result, error) {
	args = strings.TrimSpace(args)
	if args == "" {
		dir, err := session.ProjectDir(host.Cwd())
		if err != nil {
			return command.Result{}, err
		}
		return command.Result{Text: fmt.Sprintf("recent sessions live in: %s\n(use `/resume <id-prefix>` to load one)", dir)}, nil
	}
	id, err := session.ResolveResume(host.Cwd(), args)
	if err != nil {
		return command.Result{}, err
	}
	host.AppendUIMessage("resuming session " + id + " — restart with `anthrogo --resume " + id + "` to load it into the engine")
	return command.Result{Text: "resolved: " + id}, nil
}

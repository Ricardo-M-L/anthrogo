package builtins

import (
	"context"
	"errors"
	"strings"

	"github.com/ricardo/anthrogo/pkg/command"
	"github.com/ricardo/anthrogo/pkg/permissions"
)

type AddDir struct{}

func (AddDir) Name() string        { return "/add-dir" }
func (AddDir) Aliases() []string   { return nil }
func (AddDir) Description() string { return "Add a directory to additionalWorkingDirectories" }
func (AddDir) Type() command.Type  { return command.TypeLocal }

func (AddDir) Run(_ context.Context, args string, host command.Host) (command.Result, error) {
	path := strings.TrimSpace(args)
	if path == "" {
		return command.Result{}, errors.New("usage: /add-dir <path>")
	}
	host.Permissions().AdditionalWorkingDirectories[path] = permissions.AdditionalDir{Path: path}
	return command.Result{Text: "added " + path}, nil
}

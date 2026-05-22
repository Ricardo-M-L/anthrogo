package builtins

import (
	"context"
	"fmt"
	"strings"

	"github.com/ricardo/anthrogo/pkg/command"
	"github.com/ricardo/anthrogo/pkg/skill"
)

type Skills struct {
	HomeRoot string
	CwdRoot  string
}

func (Skills) Name() string      { return "/skills" }
func (Skills) Aliases() []string { return nil }
func (Skills) Description() string {
	return "List loaded skills (subcommands: show <name>, reload, install <src>)"
}
func (Skills) Type() command.Type { return command.TypeLocal }

func (s Skills) Run(ctx context.Context, args string, host command.Host) (command.Result, error) {
	reg := host.Skills()
	if reg == nil {
		return command.Result{Text: "no skill registry configured"}, nil
	}
	args = strings.TrimSpace(args)
	switch {
	case args == "":
		return listSkills(reg), nil
	case args == "reload":
		warnings, err := reg.Reload(s.HomeRoot, s.CwdRoot)
		if err != nil {
			return command.Result{}, err
		}
		out := fmt.Sprintf("reloaded skills (now %d)", len(reg.List()))
		if len(warnings) > 0 {
			out += "\nwarnings:\n" + strings.Join(warnings, "\n")
		}
		out += "\n\nnote: the model's system prompt was built at startup and still lists the original skills. Restart anthrogo to expose newly-added skills to the model. Existing skills can still be invoked by name."
		return command.Result{Text: out}, nil
	case strings.HasPrefix(args, "show "):
		name := strings.TrimSpace(strings.TrimPrefix(args, "show "))
		return showSkill(reg, name), nil
	case strings.HasPrefix(args, "install "):
		src := strings.TrimSpace(strings.TrimPrefix(args, "install "))
		sk, warnings, err := reg.Install(src, s.HomeRoot)
		if err != nil {
			return command.Result{Text: "install failed: " + err.Error()}, nil
		}
		out := fmt.Sprintf("installed skill: %s", sk.Name)
		if len(warnings) > 0 {
			out += "\nwarnings:\n  " + strings.Join(warnings, "\n  ")
		}
		return command.Result{Text: out}, nil
	default:
		return command.Result{Text: "usage: /skills [show <name> | reload | install <src>]"}, nil
	}
}

func listSkills(reg *skill.Registry) command.Result {
	list := reg.List()
	if len(list) == 0 {
		return command.Result{Text: "(no skills loaded)"}
	}
	var b strings.Builder
	for _, sk := range list {
		fmt.Fprintf(&b, "%-25s  [%s]  %s\n", sk.Name, sk.Source, sk.Description)
	}
	return command.Result{Text: b.String()}
}

func showSkill(reg *skill.Registry, name string) command.Result {
	sk, ok := reg.Get(name)
	if !ok {
		return command.Result{Text: "skill not found: " + name}
	}
	return command.Result{Text: fmt.Sprintf("# %s\n[source: %s, base: %s]\n\n%s", sk.Name, sk.Source, sk.BasePath, sk.Body)}
}

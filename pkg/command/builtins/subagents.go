package builtins

import (
	"context"
	"fmt"
	"strings"

	"github.com/ricardo/anthrogo/pkg/command"
	"github.com/ricardo/anthrogo/pkg/subagent"
)

// Subagents is the /subagents slash command. It lists, shows, and reloads
// user-defined subagent types loaded from YAML files.
type Subagents struct {
	HomeRoot string
	CwdRoot  string
}

func (Subagents) Name() string        { return "/subagents" }
func (Subagents) Aliases() []string   { return nil }
func (Subagents) Description() string { return "List registered subagent types (subcommands: show <name>, reload)" }
func (Subagents) Type() command.Type  { return command.TypeLocal }

func (s Subagents) Run(ctx context.Context, args string, host command.Host) (command.Result, error) {
	reg := host.Subagents()
	if reg == nil {
		return command.Result{Text: "no subagent registry configured"}, nil
	}
	args = strings.TrimSpace(args)
	switch {
	case args == "":
		return listSubagents(reg), nil
	case args == "reload":
		return reloadSubagents(reg, s.HomeRoot, s.CwdRoot), nil
	case strings.HasPrefix(args, "show "):
		return showSubagent(reg, strings.TrimSpace(strings.TrimPrefix(args, "show "))), nil
	default:
		return command.Result{Text: "usage: /subagents [show <name> | reload]"}, nil
	}
}

func listSubagents(reg *subagent.Registry) command.Result {
	list := reg.List()
	if len(list) == 0 {
		return command.Result{Text: "(no subagent types)"}
	}
	var b strings.Builder
	for _, s := range list {
		fmt.Fprintf(&b, "%-25s  %s\n", s.Name, s.Description)
	}
	return command.Result{Text: b.String()}
}

func showSubagent(reg *subagent.Registry, name string) command.Result {
	s, ok := reg.Get(name)
	if !ok {
		return command.Result{Text: "subagent type not found: " + name}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "name: %s\n", s.Name)
	fmt.Fprintf(&b, "description: %s\n", s.Description)
	if s.Model != "" {
		fmt.Fprintf(&b, "model: %s\n", s.Model)
	}
	if len(s.ToolAllowlist) > 0 {
		fmt.Fprintf(&b, "tool_allowlist: %s\n", strings.Join(s.ToolAllowlist, ", "))
	} else {
		b.WriteString("tool_allowlist: (inherit all)\n")
	}
	if s.SystemPromptSuffix != "" {
		b.WriteString("system_prompt_suffix:\n")
		b.WriteString(s.SystemPromptSuffix)
	}
	return command.Result{Text: b.String()}
}

func reloadSubagents(reg *subagent.Registry, homeRoot, cwdRoot string) command.Result {
	// Start from a fresh default registry (preserves built-in general-purpose).
	fresh := subagent.DefaultRegistry()
	specs, warnings, err := subagent.LoadAll(homeRoot, cwdRoot)
	if err != nil {
		return command.Result{Text: "reload failed: " + err.Error()}
	}
	for _, s := range specs {
		if s.Name == "general-purpose" {
			continue // reserved; loader already warned
		}
		fresh.Register(s)
	}
	reg.Replace(fresh)

	out := fmt.Sprintf("reloaded subagent registry (now %d types)", len(reg.List()))
	if len(warnings) > 0 {
		out += "\nwarnings:\n  " + strings.Join(warnings, "\n  ")
	}
	out += "\n\nnote: system prompt was built at startup and still lists original types; restart anthrogo to surface newly-added types to the model."
	return command.Result{Text: out}
}

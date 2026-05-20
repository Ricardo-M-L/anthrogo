package builtins

import (
	"context"
	"fmt"
	"strings"

	"github.com/ricardo/anthrogo/pkg/command"
	"github.com/ricardo/anthrogo/pkg/plugin"
)

// Plugin implements the /plugin slash command.
type Plugin struct {
	HomeRoot string
	CwdRoot  string
}

func (Plugin) Name() string        { return "/plugin" }
func (Plugin) Aliases() []string   { return nil }
func (Plugin) Description() string {
	return "Manage installed plugins (subcommands: info <name>, reload, install <path>, remove <name>)"
}
func (Plugin) Type() command.Type { return command.TypeLocal }

func (p Plugin) Run(_ context.Context, args string, host command.Host) (command.Result, error) {
	raw := host.Plugins()
	if raw == nil {
		return command.Result{Text: "no plugin registry configured"}, nil
	}
	reg, ok := raw.(*plugin.Registry)
	if !ok {
		return command.Result{Text: "plugin registry has unexpected type"}, nil
	}
	args = strings.TrimSpace(args)
	switch {
	case args == "":
		return listPlugins(reg), nil
	case args == "reload":
		return p.doReload(reg), nil
	case strings.HasPrefix(args, "info "):
		name := strings.TrimSpace(strings.TrimPrefix(args, "info "))
		return infoPlugin(reg, name), nil
	case strings.HasPrefix(args, "install "):
		src := strings.TrimSpace(strings.TrimPrefix(args, "install "))
		return p.doInstall(reg, src), nil
	case strings.HasPrefix(args, "remove "):
		name := strings.TrimSpace(strings.TrimPrefix(args, "remove "))
		return p.doRemove(reg, name), nil
	default:
		return command.Result{Text: "usage: /plugin [info <name> | reload | install <path> | remove <name>]"}, nil
	}
}

func (p Plugin) doReload(reg *plugin.Registry) command.Result {
	warnings, err := reg.Reload(p.HomeRoot, p.CwdRoot)
	if err != nil {
		return command.Result{Text: "reload failed: " + err.Error()}
	}
	out := fmt.Sprintf("reloaded plugins (now %d)", len(reg.List()))
	if len(warnings) > 0 {
		out += "\nwarnings:\n" + strings.Join(warnings, "\n")
	}
	out += "\n\nnote: anthrogo must restart for command / skill / MCP-server / hook contributions to take effect at runtime; reload only refreshes the registry view."
	return command.Result{Text: out}
}

func (p Plugin) doInstall(reg *plugin.Registry, src string) command.Result {
	plg, warnings, err := reg.Install(src, p.HomeRoot)
	if err != nil {
		return command.Result{Text: "install failed: " + err.Error()}
	}
	out := fmt.Sprintf("installed %s (%d commands, %d skills, %d MCP servers)",
		plg.Name, len(plg.Commands), len(plg.Skills), len(plg.MCPServers))
	if len(warnings) > 0 {
		out += "\nwarnings:\n  " + strings.Join(warnings, "\n  ")
	}
	return command.Result{Text: out}
}

func (p Plugin) doRemove(reg *plugin.Registry, name string) command.Result {
	if err := reg.Remove(name, p.HomeRoot); err != nil {
		return command.Result{Text: "remove failed: " + err.Error()}
	}
	return command.Result{Text: "removed " + name}
}

func listPlugins(reg *plugin.Registry) command.Result {
	list := reg.List()
	if len(list) == 0 {
		return command.Result{Text: "(no plugins installed)"}
	}
	var b strings.Builder
	for _, pl := range list {
		fmt.Fprintf(&b, "%-20s  v%-8s  [%s]  %s\n", pl.Name, pl.Manifest.Version, pl.Source, pl.Manifest.Description)
	}
	return command.Result{Text: b.String()}
}

func infoPlugin(reg *plugin.Registry, name string) command.Result {
	pl, ok := reg.Get(name)
	if !ok {
		return command.Result{Text: "plugin not found: " + name}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s v%s\n", pl.Name, pl.Manifest.Version)
	fmt.Fprintf(&b, "source:       %s\n", pl.Source)
	fmt.Fprintf(&b, "base:         %s\n", pl.BasePath)
	fmt.Fprintf(&b, "description:  %s\n", pl.Manifest.Description)
	fmt.Fprintf(&b, "commands:     %d\n", len(pl.Commands))
	fmt.Fprintf(&b, "skills:       %d\n", len(pl.Skills))
	fmt.Fprintf(&b, "mcp servers:  %d\n", len(pl.MCPServers))
	hookCount := len(pl.Hooks.PreToolUse) + len(pl.Hooks.PostToolUse) + len(pl.Hooks.UserPromptSubmit) +
		len(pl.Hooks.Stop) + len(pl.Hooks.SubagentStop) + len(pl.Hooks.Notification) +
		len(pl.Hooks.PreCompact) + len(pl.Hooks.SessionStart) + len(pl.Hooks.SessionEnd)
	fmt.Fprintf(&b, "hooks:        %d\n", hookCount)
	return command.Result{Text: b.String()}
}

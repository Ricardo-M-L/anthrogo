package builtins

import (
	"context"
	"fmt"
	"strings"

	"github.com/ricardo/anthrogo/internal/mcp"
	"github.com/ricardo/anthrogo/pkg/command"
)

type MCP struct{}

func (MCP) Name() string        { return "/mcp" }
func (MCP) Aliases() []string   { return nil }
func (MCP) Description() string { return "List MCP servers (subcommands: reload, status <name>)" }
func (MCP) Type() command.Type  { return command.TypeLocal }

func (MCP) Run(ctx context.Context, args string, host command.Host) (command.Result, error) {
	mgr := host.MCP()
	if mgr == nil {
		return command.Result{Text: "no MCP manager configured"}, nil
	}
	args = strings.TrimSpace(args)
	switch {
	case args == "":
		return listServers(mgr, host), nil
	case args == "reload":
		_ = mgr.Close()
		if err := mgr.Start(ctx); err != nil {
			return command.Result{}, err
		}
		removed := host.Tools().RemoveByPrefix("mcp__")
		added := 0
		for _, t := range mgr.AllTools() {
			host.Tools().Register(t)
			added++
		}
		out := fmt.Sprintf("MCP servers reloaded (removed %d MCP tools, registered %d)", removed, added)
		out += "\n\nnote: the model's system prompt was built at startup and still lists original tools; restart anthrogo to refresh model awareness of newly-added MCP tools."
		return command.Result{Text: out}, nil
	case strings.HasPrefix(args, "status "):
		name := strings.TrimSpace(strings.TrimPrefix(args, "status "))
		return command.Result{Text: statusDetail(mgr, name)}, nil
	default:
		return command.Result{Text: "usage: /mcp [reload | status <name>]"}, nil
	}
}

func listServers(mgr *mcp.Manager, host command.Host) command.Result {
	names := mgr.Names()
	if len(names) == 0 {
		return command.Result{Text: "(no MCP servers configured)"}
	}
	var b strings.Builder
	b.WriteString("MCP servers:\n")
	for _, n := range names {
		state := mgr.State(n)
		toolCount := 0
		for _, t := range host.Tools().All() {
			if strings.HasPrefix(t.Name(), "mcp__"+n+"__") {
				toolCount++
			}
		}
		fmt.Fprintf(&b, "  %s  state=%s  tools=%d\n", n, state, toolCount)
	}
	return command.Result{Text: b.String()}
}

func statusDetail(mgr *mcp.Manager, name string) string {
	state := mgr.State(name)
	if state == "" {
		return "no such MCP server: " + name
	}
	err := mgr.Err(name)
	out := fmt.Sprintf("server: %s\nstate: %s\n", name, state)
	if err != nil {
		out += "last error: " + err.Error() + "\n"
	}
	if scfg, ok := mgr.ServerConfig(name); ok && len(scfg.Headers) > 0 {
		out += "headers:\n"
		keys := make([]string, 0, len(scfg.Headers))
		for k := range scfg.Headers {
			keys = append(keys, k)
		}
		// stable output
		for i := 0; i < len(keys); i++ {
			for j := i + 1; j < len(keys); j++ {
				if keys[i] > keys[j] {
					keys[i], keys[j] = keys[j], keys[i]
				}
			}
		}
		for _, k := range keys {
			out += fmt.Sprintf("  %s: %s\n", k, redactValue(k, scfg.Headers[k]))
		}
	}
	return out
}

// redactValue returns "<redacted>" when the header key name looks like it holds
// a credential (case-insensitive substring match). Empty values pass through.
func redactValue(key, value string) string {
	if value == "" {
		return ""
	}
	lower := strings.ToLower(key)
	for _, s := range []string{"authorization", "auth", "token", "key", "secret", "password", "bearer"} {
		if strings.Contains(lower, s) {
			return "<redacted>"
		}
	}
	return value
}

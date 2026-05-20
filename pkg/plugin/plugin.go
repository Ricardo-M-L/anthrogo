package plugin

import (
	"github.com/ricardo/anthrogo/internal/hooks"
	"github.com/ricardo/anthrogo/internal/mcp"
	"github.com/ricardo/anthrogo/pkg/command"
	"github.com/ricardo/anthrogo/pkg/skill"
)

// Plugin is a resolved, loaded plugin.
type Plugin struct {
	Name     string
	Manifest Manifest
	BasePath string
	Source   string // "home" | "cwd"

	// Resolved contributions — populated by the loader after path resolution.
	Commands   []command.Command
	Skills     []skill.Skill
	Hooks      hooks.Config
	MCPServers map[string]mcp.MCPServerConfig
}

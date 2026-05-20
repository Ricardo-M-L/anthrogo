package plugin

import (
	"github.com/ricardo/anthrogo/internal/hooks"
	"github.com/ricardo/anthrogo/internal/mcp"
)

// Manifest is the parsed plugin.yaml.
type Manifest struct {
	Name        string                         `yaml:"name"`
	Version     string                         `yaml:"version,omitempty"`
	Description string                         `yaml:"description,omitempty"`
	Author      string                         `yaml:"author,omitempty"`
	Commands    []CommandSpec                  `yaml:"commands,omitempty"`
	Skills      []SkillRef                     `yaml:"skills,omitempty"`
	Hooks       hooks.Config                   `yaml:"hooks,omitempty"`
	MCPServers  map[string]mcp.MCPServerConfig `yaml:"mcpServers,omitempty"`
}

// CommandSpec describes a single plugin-contributed slash command.
type CommandSpec struct {
	Name        string   `yaml:"name"`
	Aliases     []string `yaml:"aliases,omitempty"`
	Description string   `yaml:"description,omitempty"`
	Type        string   `yaml:"type"`
	Body        string   `yaml:"body"`
}

// SkillRef points to a skill directory relative to the plugin root.
type SkillRef struct {
	Dir string `yaml:"dir"`
}

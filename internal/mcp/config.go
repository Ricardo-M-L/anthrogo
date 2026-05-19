package mcp

import "time"

// MCPServerConfig describes one stdio-spawned MCP server.
type MCPServerConfig struct {
	Command string            `yaml:"command"`
	Args    []string          `yaml:"args"`
	Env     map[string]string `yaml:"env,omitempty"`
	Cwd     string            `yaml:"cwd,omitempty"`
	// Timeout for the initial handshake (initialize + tools/list). Defaults to 10s.
	Timeout time.Duration `yaml:"timeout,omitempty"`
}

// DefaultInitTimeout is applied when MCPServerConfig.Timeout == 0.
const DefaultInitTimeout = 10 * time.Second

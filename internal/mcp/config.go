package mcp

import "time"

// MCPServerConfig describes one MCP server. Type selects the transport:
//   - "" or "stdio": spawn a subprocess via Command/Args (default)
//   - "sse": connect to a remote 2024-11-05 SSE endpoint
//   - "streamable": connect to a remote streamable HTTP endpoint
type MCPServerConfig struct {
	// Type selects the transport. Defaults to "stdio".
	Type string `yaml:"type,omitempty"`

	// stdio fields
	Command string            `yaml:"command,omitempty"`
	Args    []string          `yaml:"args,omitempty"`
	Env     map[string]string `yaml:"env,omitempty"`
	Cwd     string            `yaml:"cwd,omitempty"`

	// HTTP transport fields (sse / streamable)
	Endpoint   string `yaml:"endpoint,omitempty"`
	MaxRetries int    `yaml:"max_retries,omitempty"`

	// Timeout for the initial handshake (initialize + tools/list). Defaults to 10s.
	Timeout time.Duration `yaml:"timeout,omitempty"`
}

// DefaultInitTimeout is applied when MCPServerConfig.Timeout == 0.
const DefaultInitTimeout = 10 * time.Second

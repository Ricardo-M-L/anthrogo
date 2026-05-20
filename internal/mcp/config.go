package mcp

import "time"

// MCPServerConfig describes one MCP server. Type selects the transport:
//   - "" or "stdio": spawn a subprocess via Command/Args (default)
//   - "sse": connect to a remote 2024-11-05 SSE endpoint
//   - "streamable": connect to a remote streamable HTTP endpoint
//   - "websocket": connect via WebSocket (ws:// or wss://)
type MCPServerConfig struct {
	// Type selects the transport. Defaults to "stdio".
	Type string `yaml:"type,omitempty"`

	// stdio fields
	Command string            `yaml:"command,omitempty"`
	Args    []string          `yaml:"args,omitempty"`
	Env     map[string]string `yaml:"env,omitempty"`
	Cwd     string            `yaml:"cwd,omitempty"`

	// HTTP transport fields (sse / streamable / websocket)
	Endpoint   string `yaml:"endpoint,omitempty"`
	MaxRetries int    `yaml:"max_retries,omitempty"`

	// Timeout for the initial handshake (initialize + tools/list). Defaults to 10s.
	Timeout time.Duration `yaml:"timeout,omitempty"`

	// ElicitationMode controls how elicitation requests from this server are handled.
	//   - "" or "decline" (default): handler is registered (capability advertised);
	//     all requests are declined with Action="decline".
	//   - "disabled": handler is not registered (capability not advertised).
	// Any other value is treated as "decline" with a log warning.
	ElicitationMode string `yaml:"elicitation_mode,omitempty"`

	// Subprotocols is the list of WebSocket subprotocols to advertise during the
	// handshake (websocket transport only). The server's selected subprotocol is
	// returned in the Sec-WebSocket-Protocol response header.
	Subprotocols []string `yaml:"subprotocols,omitempty"`

	// Headers are additional HTTP headers injected on every outgoing request.
	// Applies to websocket (via DialOptions.HTTPHeader), sse, and streamable
	// transports (via a headerInjector RoundTripper). Takes no effect on stdio.
	Headers map[string]string `yaml:"headers,omitempty"`

	// OAuth, when non-nil, enables the OAuth 2.1 PKCE authorization-code flow for
	// sse / streamable / websocket transports. The fetched token is injected as
	// "Authorization: Bearer <token>" on every outgoing HTTP request.
	OAuth *OAuthConfig `yaml:"oauth,omitempty"`
}

// OAuthConfig holds the OAuth 2.1 parameters needed to obtain an access token.
type OAuthConfig struct {
	AuthorizationURL string   `yaml:"authorization_url"`
	TokenURL         string   `yaml:"token_url"`
	ClientID         string   `yaml:"client_id"`
	ClientSecret     string   `yaml:"client_secret,omitempty"` // optional (PKCE-only public clients)
	Scopes           []string `yaml:"scopes,omitempty"`
	RedirectPort     int      `yaml:"redirect_port,omitempty"` // local loopback port; default 8765
}

// DefaultInitTimeout is applied when MCPServerConfig.Timeout == 0.
const DefaultInitTimeout = 10 * time.Second

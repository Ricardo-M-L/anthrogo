# `github.com/ricardo/anthrogo/internal/mcp`

```go
package mcp // import "github.com/ricardo/anthrogo/internal/mcp"


CONSTANTS

const DefaultInitTimeout = 10 * time.Second
    DefaultInitTimeout is applied when MCPServerConfig.Timeout == 0.


TYPES

type ElicitFn func(serverName string, message string, schema map[string]any) (action string, formData map[string]any, err error)
    ElicitFn is a callback invoked when an MCP server requests elicitation.
    Returns action ("accept"|"decline"|"cancel"), optional form data, and error.
    When nil the server falls back to returning "decline".

type LogSink func(serverName, message string)
    LogSink receives notifications/message lines from any Server in the Manager.
    nil-safe: the Manager treats a nil LogSink as a no-op.

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
    MCPServerConfig describes one MCP server. Type selects the transport:
      - "" or "stdio": spawn a subprocess via Command/Args (default)
      - "sse": connect to a remote 2024-11-05 SSE endpoint
      - "streamable": connect to a remote streamable HTTP endpoint
      - "websocket": connect via WebSocket (ws:// or wss://)

func (c *MCPServerConfig) Expand()
    Expand resolves any env: prefixed values in Headers to their actual
    environment variable values. This mirrors the env:VARNAME expansion used for
    APIKey (M5.3 subagent loader, M6.5 OAuth).

type Manager struct {
	// Has unexported fields.
}
    Manager owns N MCP Servers and exposes their tools through tool.Tool
    adapters. Construct with NewManager; call Start before AllTools.

func NewManager(logSink LogSink) *Manager
    NewManager constructs a Manager with an optional LogSink.

func (m *Manager) AddServer(name string, cfg MCPServerConfig)
    AddServer registers a server config. Server is not started until Start().

func (m *Manager) AllResources(ctx context.Context) map[string][]*sdk.Resource
    AllResources lists resources from every Ready server. Per-server errors are
    logged via LogSink and omitted from the result map.

func (m *Manager) AllTools() []tool.Tool
    AllTools returns one MCPAdapter per (ready server, advertised tool).

func (m *Manager) Close() error
    Close terminates every server.

func (m *Manager) Err(name string) error
    Err returns the last error for one server.

func (m *Manager) Names() []string
    Names returns server names in deterministic (sorted) order.

func (m *Manager) ReadResource(ctx context.Context, server, uri string) (*sdk.ReadResourceResult, error)
    ReadResource reads a resource by URI from a named server.

func (m *Manager) ServerConfig(name string) (MCPServerConfig, bool)
    ServerConfig returns the MCPServerConfig for the named server, and whether
    the server exists. Used by status display to show (redacted) headers.

func (m *Manager) SetElicitationHandler(fn ElicitFn)
    SetElicitationHandler installs fn as the elicitation callback for
    all current and future servers. Call before Start for best effect;
    any already- started servers update their elicitFn immediately (affects next
    elicitation).

func (m *Manager) Start(ctx context.Context) error
    Start spawns every server in parallel and waits for all of them to settle
    (ready or failed). Returns nil even if some servers failed — inspect with
    State(name).

func (m *Manager) State(name string) State
    State returns the state of one server, or "" if no such name.

type OAuthConfig struct {
	AuthorizationURL string   `yaml:"authorization_url"`
	TokenURL         string   `yaml:"token_url"`
	ClientID         string   `yaml:"client_id"`
	ClientSecret     string   `yaml:"client_secret,omitempty"` // optional (PKCE-only public clients)
	Scopes           []string `yaml:"scopes,omitempty"`
	RedirectPort     int      `yaml:"redirect_port,omitempty"` // local loopback port; default 8765
}
    OAuthConfig holds the OAuth 2.1 parameters needed to obtain an access token.

type Server struct {
	Name string

	// Has unexported fields.
}
    Server wraps one MCP stdio subprocess + its SDK ClientSession.

func NewServer(name string, cfg MCPServerConfig, notifyLog func(string, string), elicit ElicitFn) *Server
    NewServer constructs (but does not start) a Server. elicit may be nil;
    when non-nil it is called instead of auto-declining elicitation requests.

func (s *Server) CallTool(ctx context.Context, name string, args map[string]any) (*sdk.CallToolResult, error)
    CallTool invokes a tool on this server by name.

func (s *Server) Close() error
    Close gracefully terminates the server. The SDK's CommandTransport owns the
    cmd lifecycle (SIGTERM → wait → SIGKILL) via TerminateDuration, so we only
    need to call sess.Close().

func (s *Server) Err() error
    Err returns the last error if state is Failed.

func (s *Server) ListResources(ctx context.Context) ([]*sdk.Resource, error)
    ListResources returns the server's currently-advertised resources. Follows
    pagination via NextCursor. Returns an error if the server is not Ready.

func (s *Server) ReadResource(ctx context.Context, uri string) (*sdk.ReadResourceResult, error)
    ReadResource reads a resource by URI. Returns an error if the server is not
    Ready.

func (s *Server) Start(parent context.Context) error
    Start spawns or connects the server, performs the initialize handshake,
    and calls tools/list. State is StateReady on success or StateFailed on any
    error. It may be called again after Close to reload.

func (s *Server) State() State
    State returns the current state. Concurrent-safe.

func (s *Server) Tools() []*sdk.Tool
    Tools returns a copy of the most recent tools/list snapshot.

type State string
    State is the public lifecycle phase of one MCP server.

const (
	StateInit   State = "init"
	StateReady  State = "ready"
	StateFailed State = "failed"
	StateClosed State = "closed"
)
type WebSocketClientTransport struct {
	Endpoint     string
	HTTPHeader   http.Header
	Subprotocols []string
}
    WebSocketClientTransport opens a websocket to Endpoint and frames each
    JSON-RPC message as one text-mode websocket message. Endpoint must use the
    ws:// or wss:// scheme. HTTPHeader is optional; use it for auth tokens or
    custom headers. Subprotocols is optional; when non-empty the values are
    advertised in the Sec-WebSocket-Protocol handshake header.

func (t *WebSocketClientTransport) Connect(ctx context.Context) (sdk.Connection, error)
    Connect dials the WebSocket endpoint and returns a Connection. Implements
    sdk.Transport.

```

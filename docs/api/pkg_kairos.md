# `github.com/ricardo/anthrogo/pkg/kairos`

```go
package kairos // import "github.com/ricardo/anthrogo/pkg/kairos"


CONSTANTS

const MaxHops = 2
    MaxHops is the maximum number of KAIROS hops allowed before Remote-typed
    subagents are excluded from a worker's registry.


FUNCTIONS

func DispatchRemote(ctx context.Context, endpoint, authToken, subagentType, description, prompt string) (string, error)
    DispatchRemote sends a RunRequest to a KAIROS worker at endpoint, streams
    the SSE response, and returns the accumulated final text.

    The client accumulates text deltas and returns the "final" field from the
    event: done frame (preferred) or the accumulated deltas as a fallback.
    On event: error the returned error contains the remote error message.

    DispatchRemote is a thin wrapper around DispatchRemoteWithOptions with no
    exec-tools-locally behaviour.

func DispatchRemoteWithOptions(ctx context.Context, endpoint string, req RunRequest, opts ClientOptions) (string, error)
    DispatchRemoteWithOptions is the full client entry-point. When
    opts.ExecToolsLocally is true it:
     1. Sends X-Anthrogo-Exec-Tools-Locally: true on the request.
     2. Captures the run_id from event: run_id.
     3. On each event: tool_use_request, dispatches via opts.ToolRegistry,
        gates via opts.Permissions, and POSTs the ToolResult back to
        /kairos/run/<rid>/tool-result.
     4. Continues consuming text deltas until event: done.

    The client-side permission gate (opts.Permissions) applies independently of
    any gate on the worker. A denied tool call produces an IsError ToolResult
    that the worker's engine sees and may handle as it sees fit.

func GenerateKeyPair() (priv ed25519.PrivateKey, pub ed25519.PublicKey, err error)
    GenerateKeyPair returns (private, public) keys.

func LoadPrivateKey(path string) (ed25519.PrivateKey, error)
    LoadPrivateKey reads a base64-encoded ed25519 private key from a file.

func LoadPublicKey(input string) (ed25519.PublicKey, error)
    LoadPublicKey reads a base64 ed25519 public key from a file or inline
    literal.

    Detection order:
     1. Try to decode input directly as base64. If the result is exactly
        ed25519.PublicKeySize (32) bytes, use it.
     2. Otherwise treat input as a file path and read the file, then decode its
        contents as base64.

    This handles the common case where an inline base64 literal happens to start
    with '/' (valid base64 character) without incorrectly treating it as a path.

func VerifyFrame(pub ed25519.PublicKey, frame SignedFrame) error
    VerifyFrame verifies the signature in the frame against its payload using
    the given public key. Returns nil on success, an error on failure.


TYPES

type ClientOptions struct {
	AuthToken        string
	ExecToolsLocally bool
	// ToolRegistry is required when ExecToolsLocally is true. Tool calls from
	// the remote subagent are dispatched through this registry on the client.
	ToolRegistry *tool.Registry
	// Permissions is required when ExecToolsLocally is true. The client-side
	// permission gate is applied before each tool call.
	Permissions *permissions.Context
	// OnTextDelta, if non-nil, is invoked for each event: text SSE message.
	// Called from the SSE parse loop goroutine; implementations must be thread-safe.
	OnTextDelta func(string)
	// TrustKey, if set, causes every incoming SSE frame to be decoded as a
	// SignedFrame first. The ed25519 signature is verified against this public
	// key before the payload is parsed. Any frame that fails verification causes
	// DispatchRemoteWithOptions to return an error immediately.
	TrustKey ed25519.PublicKey
	// InsecureSkipVerify disables TLS certificate verification. DEV ONLY.
	InsecureSkipVerify bool
	// CACertPath is the path to a PEM-encoded CA certificate bundle used to
	// verify the server's TLS certificate. Useful for internal/self-signed CAs.
	CACertPath string
}
    ClientOptions configures DispatchRemoteWithOptions.

type PermSnapshot struct {
	Mode             string                                    `json:"mode,omitempty"`
	AlwaysAllowRules map[permissions.Source][]permissions.Rule `json:"always_allow,omitempty"`
	AlwaysDenyRules  map[permissions.Source][]permissions.Rule `json:"always_deny,omitempty"`
	AlwaysAskRules   map[permissions.Source][]permissions.Rule `json:"always_ask,omitempty"`
}
    PermSnapshot is a JSON-safe projection of permissions.Context used to
    transmit the caller's permission rules to a KAIROS worker. HookDecide (a
    Go func) is intentionally not serialized; the worker rebuilds it from the
    transmitted Hooks config if non-nil.

type RemoteContext struct {
	HopDepth    int           `json:"hop_depth"`
	Hooks       *hooks.Config `json:"hooks,omitempty"`
	Permissions *PermSnapshot `json:"permissions,omitempty"`
}
    RemoteContext travels from client → worker over /kairos/run JSON body.
    HopDepth counts how many KAIROS hops this request has traversed already;
    workers reject Remote-type subagents when HopDepth >= MaxHops (default 2).
    Hooks + Permissions are best-effort: shell paths inside hooks are taken
    as-is (won't resolve relative to client cwd on worker); permission rules
    apply to the worker's local tool dispatch.

type RunHandler func(ctx context.Context, req RunRequest, emit func(textDelta string)) (finalText string, err error)
    RunHandler is the function signature the server invokes for each request.
    emit is called for each text delta; the returned finalText is sent as the
    event: done payload. On error, event: error is sent instead.

type RunHandlerWithToolForward func(
	ctx context.Context,
	req RunRequest,
	emitText func(string),
	emitToolUse func(req ToolUseRequest),
	waitForResult func(toolUseID string) (ToolResult, error),
) (string, error)
    RunHandlerWithToolForward extends RunHandler for the exec-tools-locally
    mode. The worker invokes emitToolUse to push a tool_use_request SSE event to
    the client, then calls waitForResult which blocks until the client POSTs the
    matching result to /kairos/run/<rid>/tool-result.

    Note: the permission gate is NOT applied on the worker side — the client
    runs its own gate before POSTing the result back. A denied tool call arrives
    as an IsError result.

type RunRequest struct {
	SubagentType  string         `json:"subagent_type"`
	Prompt        string         `json:"prompt"`
	Description   string         `json:"description,omitempty"`
	RemoteContext *RemoteContext `json:"remote_context,omitempty"`
}
    RunRequest is the JSON body for POST /kairos/run.

type Server struct {
	// Has unexported fields.
}
    Server is an HTTP handler that exposes POST /kairos/run as a KAIROS worker
    endpoint.

func NewServer(handler RunHandler, authToken string) *Server
    NewServer creates a Server with the given handler and optional Bearer
    auth token. If authToken is empty, all requests are accepted without
    authentication.

func NewServerWithSigning(handler RunHandler, authToken string, signingKey ed25519.PrivateKey) *Server
    NewServerWithSigning creates a Server with a signing key. Every SSE event
    payload is wrapped in a SignedFrame (signed with the ed25519 private
    key) before transmission. Clients must set ClientOptions.TrustKey to the
    corresponding public key to verify frames.

func NewServerWithToolForward(handler RunHandler, handlerWithForward RunHandlerWithToolForward, authToken string) *Server
    NewServerWithToolForward creates a Server that supports both plain
    RunHandler requests and exec-tools-locally requests (when the client sends
    X-Anthrogo-Exec-Tools-Locally: true). When that header is absent the plain
    handler is used.

func (s *Server) Handler() http.Handler
    Handler returns an http.Handler that serves /kairos/run,
    /kairos/run/{rid}/tool-result, and /kairos/healthz.

func (s *Server) Run(ctx context.Context, addr string) error
    Run starts an HTTP server on addr and blocks until ctx is cancelled or the
    server fails. It shuts down gracefully when ctx is done.

func (s *Server) RunAutocert(ctx context.Context, addr string, domains []string, cacheDir string) error
    RunAutocert starts an HTTPS server on addr using Let's Encrypt
    auto-provisioned certificates for the given domains. Certificates are cached
    under cacheDir. Port 443 must be reachable from the internet for the ACME
    HTTP-01 challenge. It blocks until ctx is cancelled or the server fails,
    and shuts down gracefully when ctx is done.

func (s *Server) RunTLS(ctx context.Context, addr, certFile, keyFile string) error
    RunTLS starts an HTTPS server on addr using the supplied PEM cert and key
    files. It blocks until ctx is cancelled or the server fails, and shuts down
    gracefully when ctx is done.

func (s *Server) SetHandlerWithForward(h RunHandlerWithToolForward)
    SetHandlerWithForward sets the exec-tools-locally handler on an
    existing Server. This is useful when a Server was constructed via
    NewServerWithSigning and also needs to support the exec-tools-locally
    protocol.

type SignedFrame struct {
	Payload   json.RawMessage `json:"payload"`
	Signature string          `json:"sig"` // base64 of Sign(payload)
}
    SignedFrame wraps an SSE payload with a base64 ed25519 signature over its
    canonical JSON marshalling.

func SignFrame(priv ed25519.PrivateKey, payload []byte) SignedFrame
    SignFrame signs the given payload bytes with the private key and returns a
    SignedFrame.

type ToolResult struct {
	ToolUseID string `json:"tool_use_id"`
	Text      string `json:"text"`
	IsError   bool   `json:"is_error"`
}
    ToolResult is the payload the client POSTs back to the worker after
    executing a tool locally. IsError mirrors tool.Result.IsError.

type ToolUseRequest struct {
	ToolUseID string         `json:"tool_use_id"`
	ToolName  string         `json:"tool_name"`
	ToolInput map[string]any `json:"tool_input"`
}
    ToolUseRequest is the payload sent by the worker over SSE when it needs the
    client to execute a tool call. The worker blocks waiting for the matching
    ToolResult to arrive via POST /kairos/run/<rid>/tool-result.

```

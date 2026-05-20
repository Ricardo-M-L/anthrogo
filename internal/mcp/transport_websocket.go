package mcp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/coder/websocket"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// WebSocketClientTransport opens a websocket to Endpoint and frames each
// JSON-RPC message as one text-mode websocket message.
// Endpoint must use the ws:// or wss:// scheme.
// HTTPHeader is optional; use it for auth tokens or custom headers.
// Subprotocols is optional; when non-empty the values are advertised in the
// Sec-WebSocket-Protocol handshake header.
type WebSocketClientTransport struct {
	Endpoint     string
	HTTPHeader   http.Header
	Subprotocols []string
}

// Connect dials the WebSocket endpoint and returns a Connection.
// Implements sdk.Transport.
func (t *WebSocketClientTransport) Connect(ctx context.Context) (sdk.Connection, error) {
	c, _, err := websocket.Dial(ctx, t.Endpoint, &websocket.DialOptions{
		HTTPHeader:   t.HTTPHeader,
		Subprotocols: t.Subprotocols,
	})
	if err != nil {
		return nil, fmt.Errorf("websocket dial %s: %w", t.Endpoint, err)
	}
	// Allow large messages (e.g. large tool results or resource reads).
	c.SetReadLimit(16 * 1024 * 1024)

	return &wsConnection{c: c, sid: randomWSSessionID()}, nil
}

// wsConnection wraps a *websocket.Conn and satisfies sdk.Connection.
type wsConnection struct {
	c   *websocket.Conn
	sid string
	wmu sync.Mutex // serialises concurrent writes
}

// Read blocks until one JSON-RPC message arrives on the websocket.
// Implements sdk.Connection.
func (c *wsConnection) Read(ctx context.Context) (jsonrpc.Message, error) {
	_, data, err := c.c.Read(ctx)
	if err != nil {
		return nil, err
	}
	return jsonrpc.DecodeMessage(data)
}

// Write serialises msg to JSON and sends it as a single text-mode websocket
// message. Write is safe for concurrent use (protected by a mutex).
// Implements sdk.Connection.
func (c *wsConnection) Write(ctx context.Context, msg jsonrpc.Message) error {
	raw, err := jsonrpc.EncodeMessage(msg)
	if err != nil {
		return fmt.Errorf("wsConnection.Write: encode: %w", err)
	}
	c.wmu.Lock()
	defer c.wmu.Unlock()
	return c.c.Write(ctx, websocket.MessageText, raw)
}

// Close sends a normal-closure frame and tears down the connection.
// Implements sdk.Connection.
func (c *wsConnection) Close() error {
	return c.c.Close(websocket.StatusNormalClosure, "")
}

// SessionID returns a randomly-generated session identifier for this connection.
// Implements sdk.Connection.
func (c *wsConnection) SessionID() string { return c.sid }

// randomWSSessionID produces a "ws-<16 hex chars>" string.
func randomWSSessionID() string {
	var b [8]byte
	_, _ = io.ReadFull(rand.Reader, b[:])
	return "ws-" + hex.EncodeToString(b[:])
}

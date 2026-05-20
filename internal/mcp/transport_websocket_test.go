package mcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coder/websocket"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/stretchr/testify/require"
)

// echoHandler accepts a websocket connection and echoes every message back.
func echoHandler(t *testing.T) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			t.Logf("echo accept: %v", err)
			return
		}
		defer c.Close(websocket.StatusNormalClosure, "")
		for {
			_, data, err := c.Read(r.Context())
			if err != nil {
				return
			}
			if werr := c.Write(r.Context(), websocket.MessageText, data); werr != nil {
				return
			}
		}
	}
}

// httpToWS replaces http:// with ws:// in a URL returned by httptest.Server.
func httpToWS(u string) string {
	return "ws" + strings.TrimPrefix(u, "http")
}

// TestWebSocketTransport_Dial verifies that Connect succeeds and Close is clean.
func TestWebSocketTransport_Dial(t *testing.T) {
	srv := httptest.NewServer(echoHandler(t))
	defer srv.Close()

	tp := &WebSocketClientTransport{Endpoint: httpToWS(srv.URL)}
	conn, err := tp.Connect(context.Background())
	require.NoError(t, err)
	require.NotNil(t, conn)

	// SessionID should be non-empty and carry the "ws-" prefix.
	require.True(t, strings.HasPrefix(conn.SessionID(), "ws-"), "session ID: %s", conn.SessionID())

	require.NoError(t, conn.Close())
}

// TestWebSocketTransport_ReadWrite exercises a full Write→Read round-trip.
// We send a valid JSON-RPC request and assert the echoed payload decodes to the
// same message.
func TestWebSocketTransport_ReadWrite(t *testing.T) {
	srv := httptest.NewServer(echoHandler(t))
	defer srv.Close()

	tp := &WebSocketClientTransport{Endpoint: httpToWS(srv.URL)}
	conn, err := tp.Connect(context.Background())
	require.NoError(t, err)
	defer conn.Close()

	// MakeID accepts nil, float64, or string (the valid JSON-decoded ID types).
	id, err := jsonrpc.MakeID(float64(42))
	require.NoError(t, err)
	sent := &jsonrpc.Request{
		ID:     id,
		Method: "ping",
	}

	require.NoError(t, conn.Write(context.Background(), sent))

	got, err := conn.Read(context.Background())
	require.NoError(t, err)
	require.NotNil(t, got)

	// Re-encode both messages to JSON and compare bytes to avoid type-assertion
	// fragility (the decoded concrete type may differ from the sent type).
	sentBytes, err := jsonrpc.EncodeMessage(sent)
	require.NoError(t, err)
	gotBytes, err := jsonrpc.EncodeMessage(got)
	require.NoError(t, err)
	require.Equal(t, string(sentBytes), string(gotBytes))
}

// TestWebSocketTransport_HTTPHeaderAndSubprotocols verifies that HTTPHeader and
// Subprotocols are forwarded to the server during the WebSocket handshake.
func TestWebSocketTransport_HTTPHeaderAndSubprotocols(t *testing.T) {
	var seenHeader, seenSubproto string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenHeader = r.Header.Get("X-Anthrogo-Test")
		seenSubproto = r.Header.Get("Sec-WebSocket-Protocol")
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{"mcp"}})
		if err != nil {
			t.Logf("accept error: %v", err)
			return
		}
		defer c.Close(websocket.StatusNormalClosure, "")
	}))
	defer srv.Close()

	tp := &WebSocketClientTransport{
		Endpoint:     httpToWS(srv.URL),
		HTTPHeader:   http.Header{"X-Anthrogo-Test": []string{"hello"}},
		Subprotocols: []string{"mcp"},
	}
	conn, err := tp.Connect(context.Background())
	require.NoError(t, err)
	conn.Close()
	require.Equal(t, "hello", seenHeader)
	require.Contains(t, seenSubproto, "mcp")
}

// TestWebSocketTransport_ServerCloses verifies that a mid-read server close
// surfaces an error to the caller of Read (not a hang or panic).
func TestWebSocketTransport_ServerCloses(t *testing.T) {
	closeSignal := make(chan struct{})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		// Wait until test says close, then close without sending anything.
		<-closeSignal
		_ = c.Close(websocket.StatusGoingAway, "bye")
	}))
	defer srv.Close()

	tp := &WebSocketClientTransport{Endpoint: httpToWS(srv.URL)}
	conn, err := tp.Connect(context.Background())
	require.NoError(t, err)

	// Signal server to close, then Read should return an error.
	close(closeSignal)
	_, err = conn.Read(context.Background())
	require.Error(t, err, "Read after server close should return an error")
}

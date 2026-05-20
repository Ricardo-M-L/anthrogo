package main

// echo-server: a minimal stdio MCP server used by anthrogo's integration tests.
// Implements just enough of the spec to satisfy initialize / tools/list /
// tools/call for a single "echo" tool.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func main() {
	enc := json.NewEncoder(os.Stdout)
	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var req rpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			continue
		}
		switch req.Method {
		case "initialize":
			_ = enc.Encode(rpcResponse{
				JSONRPC: "2.0", ID: req.ID,
				Result: map[string]any{
					"protocolVersion": "2024-11-05",
					"capabilities":    map[string]any{"tools": map[string]any{}},
					"serverInfo":      map[string]any{"name": "echo", "version": "0.0.1"},
				},
			})
		case "notifications/initialized":
			// no response expected
		case "tools/list":
			_ = enc.Encode(rpcResponse{
				JSONRPC: "2.0", ID: req.ID,
				Result: map[string]any{
					"tools": []map[string]any{
						{
							"name":        "echo",
							"description": "Echo back the input",
							"inputSchema": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"msg": map[string]any{"type": "string"},
								},
								"required": []string{"msg"},
							},
						},
						{
							"name":        "_emit_list_changed",
							"description": "Emit tools/list_changed notification and return.",
							"inputSchema": map[string]any{
								"type":       "object",
								"properties": map[string]any{},
							},
						},
					},
				},
			})
		case "tools/call":
			var p struct {
				Name      string         `json:"name"`
				Arguments map[string]any `json:"arguments"`
			}
			_ = json.Unmarshal(req.Params, &p)
			if p.Name == "_emit_list_changed" {
				// Emit notification first (no id; one-way).
				_ = enc.Encode(map[string]any{
					"jsonrpc": "2.0",
					"method":  "notifications/tools/list_changed",
					"params":  map[string]any{},
				})
				_ = enc.Encode(rpcResponse{
					JSONRPC: "2.0", ID: req.ID,
					Result: map[string]any{
						"content": []map[string]any{
							{"type": "text", "text": "emitted"},
						},
					},
				})
				continue
			}
			msg, _ := p.Arguments["msg"].(string)
			_ = enc.Encode(rpcResponse{
				JSONRPC: "2.0", ID: req.ID,
				Result: map[string]any{
					"content": []map[string]any{
						{"type": "text", "text": "echo: " + msg},
					},
				},
			})
		default:
			_ = enc.Encode(rpcResponse{
				JSONRPC: "2.0", ID: req.ID,
				Error: &rpcError{Code: -32601, Message: fmt.Sprintf("method not found: %s", req.Method)},
			})
		}
	}
}

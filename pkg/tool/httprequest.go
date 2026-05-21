package tool

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const httpRequestDefaultTimeout = 30 * time.Second
const httpRequestDefaultMaxBytes = 5 * 1024 * 1024

var allowedMethods = map[string]bool{
	"GET":    true,
	"POST":   true,
	"PUT":    true,
	"DELETE": true,
	"PATCH":  true,
	"HEAD":   true,
}

// HTTPRequest is a general-purpose HTTP client tool (curl-like).
// Unlike WebFetch (GET-only + HTML→markdown), HTTPRequest supports all
// common HTTP verbs, raw body in/out, configurable timeout and response
// size cap, and optional file-save of the response body.
type HTTPRequest struct{ DefaultPermission }

func (*HTTPRequest) Name() string { return "HTTPRequest" }

func (*HTTPRequest) Description(context.Context) string {
	return "Make a generic HTTP request (GET/POST/PUT/DELETE/PATCH/HEAD). Use WebFetch for GET-only + HTML→markdown convenience."
}

func (*HTTPRequest) UserFacingName(input map[string]any) string {
	if u, _ := input["url"].(string); u != "" {
		method, _ := input["method"].(string)
		if method == "" {
			method = "GET"
		}
		return "HTTPRequest: " + strings.ToUpper(method) + " " + u
	}
	return "HTTPRequest"
}

// IsReadOnly returns false conservatively; the engine treats per-call.
func (*HTTPRequest) IsReadOnly() bool        { return false }
func (*HTTPRequest) IsConcurrencySafe() bool { return true }

func (*HTTPRequest) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"method": map[string]any{
				"type":        "string",
				"description": "GET (default) | POST | PUT | DELETE | PATCH | HEAD",
			},
			"url": map[string]any{
				"type": "string",
			},
			"headers": map[string]any{
				"type":                 "object",
				"additionalProperties": map[string]any{"type": "string"},
			},
			"body": map[string]any{
				"type":        "string",
				"description": "Raw request body. Use Content-Type header to indicate format.",
			},
			"timeout_ms": map[string]any{
				"type":        "integer",
				"description": "Default 30000.",
			},
			"save_to": map[string]any{
				"type":        "string",
				"description": "Optional file path; if set, response body is written here instead of returned inline.",
			},
			"max_response_size": map[string]any{
				"type":        "integer",
				"description": "Hard cap in bytes; default 5MB.",
			},
		},
		"required": []string{"url"},
	}
}

func (*HTTPRequest) Call(ctx context.Context, input map[string]any, _ *Context) (Result, error) {
	url, _ := input["url"].(string)
	if url == "" {
		return errResult("httprequest: url required"), nil
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return errResult("httprequest: only http(s) URLs allowed"), nil
	}

	method, _ := input["method"].(string)
	method = strings.ToUpper(method)
	if method == "" {
		method = "GET"
	}
	if !allowedMethods[method] {
		return errResult("httprequest: method not allowed: " + method), nil
	}

	timeoutMs := intField(input, "timeout_ms", int(httpRequestDefaultTimeout.Milliseconds()))
	maxBytes := int64(intField(input, "max_response_size", httpRequestDefaultMaxBytes))

	reqCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	var bodyReader io.Reader
	if bodyStr, _ := input["body"].(string); bodyStr != "" {
		bodyReader = strings.NewReader(bodyStr)
	}

	req, err := http.NewRequestWithContext(reqCtx, method, url, bodyReader)
	if err != nil {
		return errResult("httprequest: " + err.Error()), nil
	}

	if headers, ok := input["headers"].(map[string]any); ok {
		for k, v := range headers {
			if vs, ok := v.(string); ok {
				req.Header.Set(k, vs)
			}
		}
	}
	req.Header.Set("User-Agent", "anthrogo/0.12.2 HTTPRequest")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errResult("httprequest: " + err.Error()), nil
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return errResult("httprequest: read body: " + err.Error()), nil
	}
	truncated := false
	if int64(len(bodyBytes)) > maxBytes {
		bodyBytes = bodyBytes[:maxBytes]
		truncated = true
	}

	saveTo, _ := input["save_to"].(string)
	contentType := resp.Header.Get("Content-Type")

	data := map[string]any{
		"status":       resp.StatusCode,
		"final_url":    resp.Request.URL.String(),
		"content_type": contentType,
		"bytes":        len(bodyBytes),
		"truncated":    truncated,
	}

	isErr := resp.StatusCode >= 400

	if saveTo != "" {
		if err := os.WriteFile(saveTo, bodyBytes, 0o644); err != nil {
			return errResult("httprequest: save: " + err.Error()), nil
		}
		suffix := ""
		if truncated {
			suffix = " (truncated)"
		}
		msg := fmt.Sprintf("HTTP %d %s; saved %d bytes to %s%s",
			resp.StatusCode, http.StatusText(resp.StatusCode), len(bodyBytes), saveTo, suffix)
		return Result{Type: ResultText, Text: msg, ForLLM: msg, Data: data, IsError: isErr}, nil
	}

	// Inline response
	var text string
	if looksBinary(bodyBytes, contentType) {
		text = fmt.Sprintf("[binary, %d bytes; Content-Type: %s]", len(bodyBytes), contentType)
	} else {
		text = string(bodyBytes)
		if truncated {
			text += fmt.Sprintf("\n\n[truncated at %d bytes]", maxBytes)
		}
	}
	header := fmt.Sprintf("HTTP %d %s (Content-Type: %s, %d bytes)\n\n",
		resp.StatusCode, http.StatusText(resp.StatusCode), contentType, len(bodyBytes))
	full := header + text
	return Result{Type: ResultText, Text: full, ForLLM: full, Data: data, IsError: isErr}, nil
}

// looksBinary returns true when the response body looks like binary data.
// Text content types are always considered non-binary. For unknown types,
// a null-byte scan of the first 512 bytes is used as a heuristic.
func looksBinary(b []byte, contentType string) bool {
	if strings.Contains(contentType, "text/") ||
		strings.Contains(contentType, "application/json") ||
		strings.Contains(contentType, "application/xml") ||
		strings.Contains(contentType, "application/yaml") {
		return false
	}
	n := len(b)
	if n > 512 {
		n = 512
	}
	for i := 0; i < n; i++ {
		if b[i] == 0 {
			return true
		}
	}
	return false
}

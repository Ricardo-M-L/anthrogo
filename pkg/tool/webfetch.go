package tool

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	md "github.com/JohannesKaufmann/html-to-markdown"
)

const (
	webFetchMaxBytes = 5 * 1024 * 1024
	webFetchTTL      = 15 * time.Minute
	webFetchCacheCap = 32
)

type cacheEntry struct {
	body     string
	storedAt time.Time
}

type WebFetch struct {
	DefaultPermission
	mu       sync.Mutex
	cache    map[string]cacheEntry
	order    []string
	conv     *md.Converter
	netGuard *NetGuard // nil → DefaultNetGuard() used per call
}

func NewWebFetch() *WebFetch {
	return &WebFetch{
		cache: make(map[string]cacheEntry, webFetchCacheCap),
		conv:  md.NewConverter("", true, nil),
	}
}

func (*WebFetch) Name() string                       { return "WebFetch" }
func (*WebFetch) Description(context.Context) string { return webFetchDescription }
func (*WebFetch) UserFacingName(input map[string]any) string {
	if u, _ := input["url"].(string); u != "" {
		return "WebFetch " + u
	}
	return "WebFetch"
}
func (*WebFetch) IsReadOnly() bool        { return true }
func (*WebFetch) IsConcurrencySafe() bool { return true }

func (*WebFetch) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"url": map[string]any{"type": "string", "description": "Absolute http(s) URL."},
		},
		"required": []string{"url"},
	}
}

func (w *WebFetch) Call(ctx context.Context, input map[string]any, _ *Context) (Result, error) {
	url, _ := input["url"].(string)
	if url == "" || !(strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")) {
		return errResult("url is required (http or https)"), nil
	}
	if cached := w.getCache(url); cached != "" {
		return Result{Type: ResultText, Text: cached, ForLLM: cached, Data: map[string]any{"cached": true}}, nil
	}

	// SSRF protection: pre-flight URL check.
	guard := w.netGuard
	if guard == nil {
		guard = DefaultNetGuard()
	}
	if err := guard.CheckURL(url); err != nil {
		return errResult("webfetch: " + err.Error()), nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return errResult(err.Error()), nil
	}
	req.Header.Set("User-Agent", "anthrogo/0.2")
	// Use the guarded client (installs Dialer Control for DNS-rebinding defense,
	// and also adds a 30s timeout replacing the no-timeout http.DefaultClient).
	httpClient := guard.HTTPClient(nil)
	resp, err := httpClient.Do(req)
	if err != nil {
		return errResult(err.Error()), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		return errResult(fmt.Sprintf("HTTP %d: %s", resp.StatusCode, resp.Status)), nil
	}

	if resp.ContentLength > webFetchMaxBytes {
		return errResult(fmt.Sprintf("response too large: %d bytes (limit %d)", resp.ContentLength, webFetchMaxBytes)), nil
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, webFetchMaxBytes+1))
	if err != nil {
		return errResult(err.Error()), nil
	}
	if len(body) > webFetchMaxBytes {
		return errResult(fmt.Sprintf("response too large: >%d bytes", webFetchMaxBytes)), nil
	}

	contentType := resp.Header.Get("Content-Type")
	var out string
	if strings.Contains(contentType, "text/html") {
		converted, mErr := w.conv.ConvertString(string(body))
		if mErr == nil {
			out = converted
		} else {
			out = string(body)
		}
	} else {
		out = string(body)
	}

	w.putCache(url, out)
	return Result{Type: ResultText, Text: out, ForLLM: out, Data: map[string]any{"status": resp.StatusCode}}, nil
}

func (w *WebFetch) getCache(url string) string {
	w.mu.Lock()
	defer w.mu.Unlock()
	e, ok := w.cache[url]
	if !ok {
		return ""
	}
	if time.Since(e.storedAt) > webFetchTTL {
		delete(w.cache, url)
		// Keep w.order consistent: drop the URL so a subsequent
		// putCache doesn't push a duplicate entry and corrupt LRU
		// eviction (which trims from w.order[0]).
		for i, u := range w.order {
			if u == url {
				w.order = append(w.order[:i], w.order[i+1:]...)
				break
			}
		}
		return ""
	}
	return e.body
}

func (w *WebFetch) putCache(url, body string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, ok := w.cache[url]; !ok {
		w.order = append(w.order, url)
	}
	w.cache[url] = cacheEntry{body: body, storedAt: time.Now()}
	if len(w.order) > webFetchCacheCap {
		oldest := w.order[0]
		w.order = w.order[1:]
		delete(w.cache, oldest)
	}
}

const webFetchDescription = `Fetch the body of an http(s) URL. HTML responses are converted to markdown. Non-HTML responses are returned verbatim. Per-URL cache (15 min TTL, 32-entry LRU). 5 MB response cap.`

package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type WebSearchConfig struct {
	Backend  string
	APIKey   string
	Endpoint string
}

type WebSearch struct {
	DefaultPermission
	cfg WebSearchConfig
}

func NewWebSearch(cfg WebSearchConfig) *WebSearch { return &WebSearch{cfg: cfg} }

func (*WebSearch) Name() string                       { return "WebSearch" }
func (*WebSearch) Description(context.Context) string { return webSearchDescription }
func (*WebSearch) UserFacingName(i map[string]any) string {
	if q, _ := i["query"].(string); q != "" {
		return "WebSearch " + q
	}
	return "WebSearch"
}
func (*WebSearch) IsReadOnly() bool        { return true }
func (*WebSearch) IsConcurrencySafe() bool { return true }

func (*WebSearch) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{"type": "string"},
			"count": map[string]any{"type": "integer", "default": 10},
		},
		"required": []string{"query"},
	}
}

func (w *WebSearch) Call(ctx context.Context, input map[string]any, _ *Context) (Result, error) {
	query, _ := input["query"].(string)
	count := intField(input, "count", 10)
	if query == "" {
		return errResult("query is required"), nil
	}
	switch strings.ToLower(w.cfg.Backend) {
	case "brave":
		return w.brave(ctx, query, count)
	case "", "disabled":
		return errResult("WebSearch disabled — configure webSearch.backend + webSearch.apiKey in settings.yaml"), nil
	default:
		return errResult("unknown webSearch.backend: " + w.cfg.Backend), nil
	}
}

func (w *WebSearch) brave(ctx context.Context, query string, count int) (Result, error) {
	if w.cfg.APIKey == "" {
		return errResult("brave backend selected but no API key — configure webSearch.apiKey in settings.yaml"), nil
	}
	endpoint := w.cfg.Endpoint
	if endpoint == "" {
		endpoint = "https://api.search.brave.com/res/v1/web/search"
	}
	u, _ := url.Parse(endpoint)
	q := u.Query()
	q.Set("q", query)
	q.Set("count", fmt.Sprint(count))
	u.RawQuery = q.Encode()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	req.Header.Set("X-Subscription-Token", w.cfg.APIKey)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errResult(err.Error()), nil
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return errResult(fmt.Sprintf("brave: HTTP %d", resp.StatusCode)), nil
	}
	var payload struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
			} `json:"results"`
		} `json:"web"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return errResult("brave: decode: " + err.Error()), nil
	}
	var b strings.Builder
	for i, r := range payload.Web.Results {
		fmt.Fprintf(&b, "[%d] %s\n  %s\n  %s\n", i+1, r.Title, r.URL, r.Description)
	}
	out := b.String()
	if out == "" {
		out = "no results"
	}
	return Result{Type: ResultText, Text: out, ForLLM: out, Data: map[string]any{"count": len(payload.Web.Results)}}, nil
}

const webSearchDescription = `Search the web. Returns top results as title + URL + snippet. Backend is configured in settings.yaml (webSearch.backend: brave|tavily|disabled).`

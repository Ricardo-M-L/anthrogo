package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// WebSearchConfig holds configuration for the WebSearch tool.
type WebSearchConfig struct {
	Backend  string
	APIKey   string
	Endpoint string // backend-specific: CSE ID for google, base URL for bing/tavily
	URL      string // optional full URL override (for testing or self-hosted)
}

// WebSearch is the tool that queries a search provider.
type WebSearch struct {
	DefaultPermission
	cfg WebSearchConfig
}

// NewWebSearch creates a WebSearch with the given config.
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

// webSearchResult is the common result shape returned to the LLM.
type webSearchResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
}

// searchBackend is the per-backend function signature.
type searchBackend func(ctx context.Context, cfg WebSearchConfig, query string, count int) ([]webSearchResult, error)

// backends is the dispatch table for all supported search backends.
var backends = map[string]searchBackend{
	"brave":  searchBrave,
	"google": searchGoogle,
	"bing":   searchBing,
	"tavily": searchTavily,
}

// Call dispatches to the configured backend and returns JSON results.
func (w *WebSearch) Call(ctx context.Context, input map[string]any, _ *Context) (Result, error) {
	query, _ := input["query"].(string)
	count := intField(input, "count", 10)
	if query == "" {
		return errResult("websearch: query required"), nil
	}

	backend := strings.ToLower(w.cfg.Backend)
	if backend == "" {
		backend = "brave"
	}
	if backend == "disabled" {
		return errResult("WebSearch disabled — configure webSearch.backend + webSearch.apiKey in settings.yaml"), nil
	}

	fn, ok := backends[backend]
	if !ok {
		msg := fmt.Sprintf("websearch: unknown backend %q (supported: brave, google, bing, tavily)", w.cfg.Backend)
		return errResult(msg), nil
	}

	if w.cfg.APIKey == "" {
		msg := fmt.Sprintf("websearch: backend %q has no apiKey configured", backend)
		return errResult(msg), nil
	}

	results, err := fn(ctx, w.cfg, query, count)
	if err != nil {
		return errResult(err.Error()), nil
	}
	raw, _ := json.MarshalIndent(results, "", "  ")
	out := string(raw)
	return Result{Type: ResultText, Text: out, ForLLM: out, Data: map[string]any{"count": len(results)}}, nil
}

// searchBrave queries the Brave Search API.
func searchBrave(ctx context.Context, cfg WebSearchConfig, query string, count int) ([]webSearchResult, error) {
	endpoint := cfg.URL
	if endpoint == "" {
		endpoint = cfg.Endpoint
	}
	if endpoint == "" {
		endpoint = "https://api.search.brave.com/res/v1/web/search"
	}
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("brave: bad endpoint: %w", err)
	}
	q := u.Query()
	q.Set("q", query)
	q.Set("count", strconv.Itoa(count))
	u.RawQuery = q.Encode()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	req.Header.Set("X-Subscription-Token", cfg.APIKey)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("brave: HTTP %d: %s", resp.StatusCode, string(body))
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
		return nil, fmt.Errorf("brave: decode: %w", err)
	}
	out := make([]webSearchResult, 0, len(payload.Web.Results))
	for _, r := range payload.Web.Results {
		out = append(out, webSearchResult{Title: r.Title, URL: r.URL, Description: r.Description})
	}
	return out, nil
}

// searchGoogle queries the Google Custom Search API.
func searchGoogle(ctx context.Context, cfg WebSearchConfig, query string, count int) ([]webSearchResult, error) {
	// cfg.Endpoint stores the CSE ID (cx). cfg.APIKey is the API key.
	// cfg.URL, if set, overrides the full endpoint (useful for tests).
	if cfg.Endpoint == "" && cfg.URL == "" {
		return nil, fmt.Errorf("google: webSearch.endpoint (CSE ID) required")
	}
	if count > 10 {
		count = 10 // Google max
	}

	var u string
	if cfg.URL != "" {
		// Test / proxy override: use as-is, append query params.
		params := url.Values{}
		params.Set("key", cfg.APIKey)
		params.Set("cx", cfg.Endpoint)
		params.Set("q", query)
		if count > 0 {
			params.Set("num", strconv.Itoa(count))
		}
		u = cfg.URL + "?" + params.Encode()
	} else {
		params := url.Values{}
		params.Set("key", cfg.APIKey)
		params.Set("cx", cfg.Endpoint)
		params.Set("q", query)
		if count > 0 {
			params.Set("num", strconv.Itoa(count))
		}
		u = "https://www.googleapis.com/customsearch/v1?" + params.Encode()
	}

	req, _ := http.NewRequestWithContext(ctx, "GET", u, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("google: HTTP %d: %s", resp.StatusCode, string(body))
	}
	var raw struct {
		Items []struct {
			Title   string `json:"title"`
			Link    string `json:"link"`
			Snippet string `json:"snippet"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("google: decode: %w", err)
	}
	out := make([]webSearchResult, 0, len(raw.Items))
	for _, it := range raw.Items {
		out = append(out, webSearchResult{Title: it.Title, URL: it.Link, Description: it.Snippet})
	}
	return out, nil
}

// searchBing queries the Bing Web Search API (Azure).
func searchBing(ctx context.Context, cfg WebSearchConfig, query string, count int) ([]webSearchResult, error) {
	endpoint := cfg.URL
	if endpoint == "" {
		endpoint = cfg.Endpoint
	}
	if endpoint == "" {
		endpoint = "https://api.bing.microsoft.com/v7.0/search"
	}
	if count > 50 {
		count = 50
	}
	params := url.Values{}
	params.Set("q", query)
	if count > 0 {
		params.Set("count", strconv.Itoa(count))
	}
	req, _ := http.NewRequestWithContext(ctx, "GET", endpoint+"?"+params.Encode(), nil)
	req.Header.Set("Ocp-Apim-Subscription-Key", cfg.APIKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bing: HTTP %d: %s", resp.StatusCode, string(body))
	}
	var raw struct {
		WebPages struct {
			Value []struct {
				Name    string `json:"name"`
				URL     string `json:"url"`
				Snippet string `json:"snippet"`
			} `json:"value"`
		} `json:"webPages"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("bing: decode: %w", err)
	}
	out := make([]webSearchResult, 0, len(raw.WebPages.Value))
	for _, it := range raw.WebPages.Value {
		out = append(out, webSearchResult{Title: it.Name, URL: it.URL, Description: it.Snippet})
	}
	return out, nil
}

// searchTavily queries the Tavily Search API.
func searchTavily(ctx context.Context, cfg WebSearchConfig, query string, count int) ([]webSearchResult, error) {
	endpoint := cfg.URL
	if endpoint == "" {
		endpoint = cfg.Endpoint
	}
	if endpoint == "" {
		endpoint = "https://api.tavily.com/search"
	}
	if count > 20 {
		count = 20
	}
	bodyMap := map[string]any{
		"api_key":      cfg.APIKey,
		"query":        query,
		"max_results":  count,
		"search_depth": "basic",
	}
	body, _ := json.Marshal(bodyMap)
	req, _ := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		rb, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tavily: HTTP %d: %s", resp.StatusCode, string(rb))
	}
	var raw struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("tavily: decode: %w", err)
	}
	out := make([]webSearchResult, 0, len(raw.Results))
	for _, it := range raw.Results {
		out = append(out, webSearchResult{Title: it.Title, URL: it.URL, Description: it.Content})
	}
	return out, nil
}

const webSearchDescription = `Search the web. Returns top results as title + URL + snippet (JSON array). Backend is configured in settings.yaml (webSearch.backend: brave|google|bing|tavily).`

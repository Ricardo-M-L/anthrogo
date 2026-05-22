package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ricardo/anthrogo/pkg/provider"
)

// parseRetryAfter accepts an RFC 7231 Retry-After value: either an integer
// seconds count ("60") or an HTTP-date. Returns the duration to wait or 0
// on parse failure / absent header.
func parseRetryAfter(h string) time.Duration {
	h = strings.TrimSpace(h)
	if h == "" {
		return 0
	}
	if secs, err := strconv.Atoi(h); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(h); err == nil {
		d := time.Until(t)
		if d > 0 {
			return d
		}
	}
	return 0
}

// Provider implements provider.Provider for any OpenAI Chat Completions
// compatible endpoint (DeepSeek, Kimi, MiniMax, GLM, vllm, ollama, etc.).
type Provider struct {
	BaseURL string
	APIKey  string
	// Client is the HTTP client used for requests. When nil, a default client
	// with a 5-minute timeout is used (lazily initialised + cached).
	Client *http.Client

	defaultOnce   sync.Once
	defaultClient *http.Client
}

// New constructs a Provider.
func New(baseURL, apiKey string) *Provider {
	return &Provider{BaseURL: baseURL, APIKey: apiKey}
}

func (p *Provider) httpClient() *http.Client {
	if p.Client != nil {
		return p.Client
	}
	p.defaultOnce.Do(func() {
		p.defaultClient = &http.Client{Timeout: 5 * time.Minute}
	})
	return p.defaultClient
}

// Stream implements provider.Provider.
func (p *Provider) Stream(ctx context.Context, req provider.Request) (<-chan provider.Event, error) {
	model := req.Model
	body := buildRequest(model, req)

	bodyJSON, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("openai provider: marshal request: %w", err)
	}

	url := strings.TrimRight(p.BaseURL, "/") + "/v1/chat/completions"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, fmt.Errorf("openai provider: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.APIKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := p.httpClient().Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("openai provider: http: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests {
			ra := parseRetryAfter(resp.Header.Get("Retry-After"))
			return nil, &provider.RateLimitError{RetryAfter: ra, Body: string(body)}
		}
		return nil, fmt.Errorf("openai provider: server returned %d: %s", resp.StatusCode, string(body))
	}

	out := make(chan provider.Event, 64)
	// Emit EventStart immediately.
	out <- provider.Event{Kind: provider.EventStart}

	go parseSSE(resp, out)
	return out, nil
}

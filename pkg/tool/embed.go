package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
)

// apiKeyPattern matches common API key patterns (e.g. OpenAI sk-... tokens).
var apiKeyPattern = regexp.MustCompile(`sk-[A-Za-z0-9_\-]{16,}`)

// redactAPIKeys replaces API key patterns in s with a safe placeholder.
func redactAPIKeys(s string) string {
	return apiKeyPattern.ReplaceAllString(s, "sk-***REDACTED***")
}

// safeSnippet truncates s to 512 bytes and redacts API keys.
func safeSnippet(s string) string {
	if len(s) > 512 {
		s = s[:512] + "...[truncated]"
	}
	return redactAPIKeys(s)
}

// Embed converts text to embedding vectors via an OpenAI-compatible embeddings endpoint.
type Embed struct {
	DefaultPermission
	httpClient *http.Client
}

func (Embed) Name() string { return "Embed" }
func (Embed) Description(context.Context) string {
	return "Convert text to embedding vectors via an OpenAI-compatible embeddings endpoint."
}
func (Embed) UserFacingName(input map[string]any) string {
	if m, _ := input["model"].(string); m != "" {
		return "Embed: " + m
	}
	return "Embed"
}
func (Embed) IsReadOnly() bool        { return true }
func (Embed) IsConcurrencySafe() bool { return true }

func (Embed) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"input": map[string]any{
				"type":        "string",
				"description": "Text to embed. For multiple texts, use input_list.",
			},
			"input_list": map[string]any{
				"type":        "array",
				"items":       map[string]any{"type": "string"},
				"description": "Multiple texts to embed in one call.",
			},
			"model": map[string]any{
				"type":        "string",
				"description": "Embedding model name. Default text-embedding-3-small.",
			},
			"base_url": map[string]any{
				"type":        "string",
				"description": "Optional. Default $ANTHROGO_EMBED_BASE_URL or https://api.openai.com/v1.",
			},
			"api_key": map[string]any{
				"type":        "string",
				"description": "Optional. Default $ANTHROGO_EMBED_API_KEY or $OPENAI_API_KEY.",
			},
			"out_format": map[string]any{
				"type":        "string",
				"enum":        []string{"summary", "json"},
				"description": "summary returns dimension + first 8 floats per vector. json returns full vectors.",
			},
		},
		"required": []string{},
	}
}

func (e Embed) Call(ctx context.Context, input map[string]any, _ *Context) (Result, error) {
	// Resolve base_url
	baseURL, _ := input["base_url"].(string)
	if baseURL == "" {
		baseURL = os.Getenv("ANTHROGO_EMBED_BASE_URL")
	}
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	// Resolve api_key
	apiKey, _ := input["api_key"].(string)
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROGO_EMBED_API_KEY")
	}
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if apiKey == "" {
		return errResult("embed: api_key is required (set ANTHROGO_EMBED_API_KEY or OPENAI_API_KEY)"), nil
	}

	// Resolve model
	model, _ := input["model"].(string)
	if model == "" {
		model = "text-embedding-3-small"
	}

	// Build input list
	var inputList []string
	if raw, ok := input["input_list"]; ok {
		switch v := raw.(type) {
		case []string:
			inputList = v
		case []any:
			for _, item := range v {
				if s, ok := item.(string); ok && s != "" {
					inputList = append(inputList, s)
				}
			}
		}
	}
	if len(inputList) == 0 {
		if s, _ := input["input"].(string); s != "" {
			inputList = []string{s}
		}
	}
	if len(inputList) == 0 {
		return errResult("embed: input or input_list is required"), nil
	}

	outFormat, _ := input["out_format"].(string)
	if outFormat == "" {
		outFormat = "summary"
	}

	// Build request body
	reqBody := map[string]any{
		"model": model,
		"input": inputList,
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return errResult("embed: marshal request: " + err.Error()), nil
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/embeddings", bytes.NewReader(bodyBytes))
	if err != nil {
		return errResult("embed: create request: " + err.Error()), nil
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	client := e.httpClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return errResult("embed: http: " + err.Error()), nil
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return errResult("embed: read response: " + err.Error()), nil
	}
	if resp.StatusCode != http.StatusOK {
		return errResult(fmt.Sprintf("embed: HTTP %d: %s", resp.StatusCode, safeSnippet(string(respBytes)))), nil
	}

	// Parse OpenAI embeddings response
	var apiResp struct {
		Data []struct {
			Embedding []float64 `json:"embedding"`
			Index     int       `json:"index"`
		} `json:"data"`
		Model string `json:"model"`
		Usage struct {
			PromptTokens int `json:"prompt_tokens"`
			TotalTokens  int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(respBytes, &apiResp); err != nil {
		return errResult("embed: parse response: " + err.Error()), nil
	}
	if len(apiResp.Data) == 0 {
		return errResult("embed: no data in response"), nil
	}

	if outFormat == "json" {
		vectors := make([][]float64, len(apiResp.Data))
		for i, d := range apiResp.Data {
			vectors[i] = d.Embedding
		}
		dim := 0
		if len(vectors) > 0 {
			dim = len(vectors[0])
		}
		outData := map[string]any{
			"vectors": vectors,
			"model":   apiResp.Model,
			"dim":     dim,
		}
		out, err := json.Marshal(outData)
		if err != nil {
			return errResult("embed: marshal output: " + err.Error()), nil
		}
		text := string(out)
		return Result{Type: ResultText, Text: text, ForLLM: text}, nil
	}

	// summary format
	dim := 0
	if len(apiResp.Data) > 0 {
		dim = len(apiResp.Data[0].Embedding)
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "embedded %d text(s) with model %s, dim=%d\n", len(apiResp.Data), apiResp.Model, dim)
	for _, d := range apiResp.Data {
		preview := d.Embedding
		if len(preview) > 8 {
			preview = preview[:8]
		}
		parts := make([]string, len(preview))
		for i, f := range preview {
			parts[i] = fmt.Sprintf("%.3f", f)
		}
		fmt.Fprintf(&sb, "  [%d] preview: %s\n", d.Index, strings.Join(parts, ", "))
	}
	text := strings.TrimRight(sb.String(), "\n")
	return Result{Type: ResultText, Text: text, ForLLM: text}, nil
}

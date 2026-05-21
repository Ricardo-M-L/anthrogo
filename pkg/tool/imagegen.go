package tool

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ImageGen generates an image from a text prompt via an OpenAI-compatible images endpoint.
type ImageGen struct {
	DefaultPermission
	httpClient *http.Client
}

func (ImageGen) Name() string { return "ImageGen" }
func (ImageGen) Description(context.Context) string {
	return "Generate an image from a text prompt via an OpenAI-compatible images endpoint."
}
func (ImageGen) UserFacingName(input map[string]any) string {
	if p, _ := input["prompt"].(string); p != "" {
		if len(p) > 40 {
			p = p[:40] + "..."
		}
		return "ImageGen: " + p
	}
	return "ImageGen"
}
func (ImageGen) IsReadOnly() bool        { return false }
func (ImageGen) IsConcurrencySafe() bool { return true }

func (ImageGen) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"prompt": map[string]any{
				"type": "string",
			},
			"model": map[string]any{
				"type":        "string",
				"description": "Default dall-e-3.",
			},
			"size": map[string]any{
				"type":        "string",
				"description": "Default 1024x1024.",
			},
			"out_path": map[string]any{
				"type":        "string",
				"description": "Absolute path for the PNG. Default $TMPDIR/anthrogo-imagegen-<ts>.png",
			},
			"base_url": map[string]any{
				"type": "string",
			},
			"api_key": map[string]any{
				"type": "string",
			},
		},
		"required": []string{"prompt"},
	}
}

func (ig ImageGen) Call(ctx context.Context, input map[string]any, _ *Context) (Result, error) {
	prompt, _ := input["prompt"].(string)
	if prompt == "" {
		return errResult("imagegen: prompt is required"), nil
	}

	// Resolve base_url
	baseURL, _ := input["base_url"].(string)
	if baseURL == "" {
		baseURL = os.Getenv("ANTHROGO_IMAGE_BASE_URL")
	}
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	// Resolve api_key
	apiKey, _ := input["api_key"].(string)
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROGO_IMAGE_API_KEY")
	}
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if apiKey == "" {
		return errResult("imagegen: api_key is required (set ANTHROGO_IMAGE_API_KEY or OPENAI_API_KEY)"), nil
	}

	model, _ := input["model"].(string)
	if model == "" {
		model = "dall-e-3"
	}
	size, _ := input["size"].(string)
	if size == "" {
		size = "1024x1024"
	}
	outPath, _ := input["out_path"].(string)
	if outPath == "" {
		outPath = filepath.Join(os.TempDir(), fmt.Sprintf("anthrogo-imagegen-%d.png", time.Now().Unix()))
	}

	// Build request body
	reqBody := map[string]any{
		"model":           model,
		"prompt":          prompt,
		"n":               1,
		"size":            size,
		"response_format": "b64_json",
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return errResult("imagegen: marshal request: " + err.Error()), nil
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/images/generations", bytes.NewReader(bodyBytes))
	if err != nil {
		return errResult("imagegen: create request: " + err.Error()), nil
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)

	client := ig.httpClient
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return errResult("imagegen: http: " + err.Error()), nil
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return errResult("imagegen: read response: " + err.Error()), nil
	}
	if resp.StatusCode != http.StatusOK {
		return errResult(fmt.Sprintf("imagegen: HTTP %d: %s", resp.StatusCode, safeSnippet(string(respBytes)))), nil
	}

	// Parse response
	var apiResp struct {
		Data []struct {
			B64JSON string `json:"b64_json"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBytes, &apiResp); err != nil {
		return errResult("imagegen: parse response: " + err.Error()), nil
	}
	if len(apiResp.Data) == 0 || apiResp.Data[0].B64JSON == "" {
		return errResult("imagegen: no image data in response"), nil
	}

	imgBytes, err := base64.StdEncoding.DecodeString(apiResp.Data[0].B64JSON)
	if err != nil {
		return errResult("imagegen: base64 decode: " + err.Error()), nil
	}

	if err := os.WriteFile(outPath, imgBytes, 0o644); err != nil {
		return errResult("imagegen: write file: " + err.Error()), nil
	}

	text := fmt.Sprintf("saved %d bytes to %s (model=%s, size=%s)", len(imgBytes), outPath, model, size)
	return Result{
		Type:   ResultText,
		Text:   text,
		ForLLM: text,
		Data: map[string]any{
			"out_path": outPath,
			"bytes":    len(imgBytes),
			"model":    model,
			"size":     size,
		},
	}, nil
}

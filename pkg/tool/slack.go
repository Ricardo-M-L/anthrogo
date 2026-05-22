package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// slackURLAllowedDefault is the production allow-check for Slack webhook URLs.
func slackURLAllowedDefault(u string) bool {
	return strings.HasPrefix(u, "https://hooks.slack.com/services/")
}

// slackURLAllowed remains for backward compatibility with tests that
// override the package-level check. Prefer SlackPost.urlAllowedFn for
// new tests so per-instance overrides don't race under t.Parallel().
var slackURLAllowed = slackURLAllowedDefault

// SlackPost sends a message to a Slack channel via an Incoming Webhook URL.
type SlackPost struct {
	DefaultPermission
	httpClient   *http.Client      // injectable for testing
	urlAllowedFn func(string) bool // per-instance override; nil → use slackURLAllowed
}

// urlAllowed returns the effective URL allow check: per-instance fn if set,
// else the package-level slackURLAllowed.
func (s SlackPost) urlAllowed(u string) bool {
	if s.urlAllowedFn != nil {
		return s.urlAllowedFn(u)
	}
	return slackURLAllowed(u)
}

func (SlackPost) Name() string { return "SlackPost" }
func (SlackPost) Description(context.Context) string {
	return "Send a message to a Slack channel via an Incoming Webhook URL. Returns Slack's response status."
}
func (SlackPost) UserFacingName(input map[string]any) string {
	if u, _ := input["webhook_url"].(string); u != "" {
		return "SlackPost: " + u
	}
	return "SlackPost"
}
func (SlackPost) IsReadOnly() bool        { return false }
func (SlackPost) IsConcurrencySafe() bool { return true }

func (SlackPost) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"webhook_url": map[string]any{
				"type":        "string",
				"description": "Slack Incoming Webhook URL (https://hooks.slack.com/...). Alternatively, set SLACK_WEBHOOK_URL env var.",
			},
			"text": map[string]any{
				"type":        "string",
				"description": "Message text (markdown supported).",
			},
			"blocks": map[string]any{
				"type":        "string",
				"description": "Optional. Raw JSON for Slack Block Kit; bypasses text rendering.",
			},
			"username": map[string]any{
				"type":        "string",
				"description": "Optional. Override webhook's default username.",
			},
			"icon_emoji": map[string]any{
				"type":        "string",
				"description": "Optional. e.g. \":robot_face:\".",
			},
		},
		"required": []string{"text"},
	}
}

func (s SlackPost) Call(ctx context.Context, input map[string]any, _ *Context) (Result, error) {
	text, _ := input["text"].(string)
	if text == "" {
		return errResult("slackpost: text is required"), nil
	}

	webhookURL, _ := input["webhook_url"].(string)
	if webhookURL == "" {
		webhookURL = os.Getenv("SLACK_WEBHOOK_URL")
	}
	if webhookURL == "" {
		return errResult("slackpost: no webhook URL"), nil
	}
	if !s.urlAllowed(webhookURL) {
		return errResult("slackpost: url must start with https://hooks.slack.com/services/"), nil
	}

	payload := map[string]any{
		"text": text,
	}
	if username, _ := input["username"].(string); username != "" {
		payload["username"] = username
	}
	if iconEmoji, _ := input["icon_emoji"].(string); iconEmoji != "" {
		payload["icon_emoji"] = iconEmoji
	}

	// If blocks provided, parse and merge.
	if blocksJSON, _ := input["blocks"].(string); blocksJSON != "" {
		var blocks []map[string]any
		if err := json.Unmarshal([]byte(blocksJSON), &blocks); err != nil {
			return errResult("slackpost: invalid blocks JSON: " + err.Error()), nil
		}
		payload["blocks"] = blocks
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return errResult("slackpost: marshal: " + err.Error()), nil
	}

	// SSRF defense: even though the prefix check above gates against
	// non-Slack hosts, a hostile Slack-side DNS or redirect could route the
	// connect to a metadata IP. Run the URL through NetGuard's pre-flight
	// + dial-time check, same as WebFetch / HTTPRequest / Embed / ImageGen.
	guard := DefaultNetGuard()
	if err := guard.CheckURL(webhookURL); err != nil {
		return errResult("slackpost: " + err.Error()), nil
	}

	// Use the netguard-wrapped client to enforce the Control hook at dial
	// time. Keep the existing 10s timeout (tighter than the default 30s)
	// when no injected client was provided.
	var client *http.Client
	if s.httpClient != nil {
		client = guard.HTTPClient(s.httpClient)
	} else {
		client = guard.HTTPClient(&http.Client{Timeout: 10 * time.Second})
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return errResult("slackpost: " + err.Error()), nil
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return errResult("slackpost: " + err.Error()), nil
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode != http.StatusOK {
		return errResult(fmt.Sprintf("slackpost: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))), nil
	}

	msg := fmt.Sprintf("ok: HTTP %d", resp.StatusCode)
	return Result{Type: ResultText, Text: msg, ForLLM: msg}, nil
}

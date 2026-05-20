package kairos

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// DispatchRemote sends a RunRequest to a KAIROS worker at endpoint, streams
// the SSE response, and returns the accumulated final text.
//
// The client accumulates text deltas and returns the "final" field from the
// event: done frame (preferred) or the accumulated deltas as a fallback.
// On event: error the returned error contains the remote error message.
func DispatchRemote(ctx context.Context, endpoint, authToken, subagentType, description, prompt string) (string, error) {
	body, _ := json.Marshal(RunRequest{
		SubagentType: subagentType,
		Prompt:       prompt,
		Description:  description,
	})
	req, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(endpoint, "/")+"/kairos/run", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if authToken != "" {
		req.Header.Set("Authorization", "Bearer "+authToken)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		msg, _ := bufio.NewReader(resp.Body).ReadString('\n')
		return "", fmt.Errorf("kairos: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(msg))
	}

	// Parse SSE stream: lines of "event: <name>" and "data: <json>" separated by blank lines.
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 1024), 1024*1024)

	var event, data string
	var accumulated strings.Builder
	var finalText string
	var streamErr error

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			// Blank line: dispatch accumulated event.
			if event != "" {
				switch event {
				case "text":
					var p struct {
						Text string `json:"text"`
					}
					_ = json.Unmarshal([]byte(data), &p)
					accumulated.WriteString(p.Text)
				case "done":
					var p struct {
						Final string `json:"final"`
					}
					_ = json.Unmarshal([]byte(data), &p)
					if p.Final != "" {
						finalText = p.Final
					}
				case "error":
					var p struct {
						Error string `json:"error"`
					}
					_ = json.Unmarshal([]byte(data), &p)
					streamErr = fmt.Errorf("kairos remote error: %s", p.Error)
				}
			}
			event = ""
			data = ""
			continue
		}
		if strings.HasPrefix(line, "event: ") {
			event = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			data = strings.TrimPrefix(line, "data: ")
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	if streamErr != nil {
		return "", streamErr
	}
	// Prefer the explicit "final" from event: done; fall back to accumulated deltas.
	if finalText != "" {
		return finalText, nil
	}
	return accumulated.String(), nil
}

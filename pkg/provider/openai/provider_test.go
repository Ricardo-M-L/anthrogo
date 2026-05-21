package openai_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/pkg/message"
	"github.com/ricardo/anthrogo/pkg/provider"
	"github.com/ricardo/anthrogo/pkg/provider/openai"
)

func collectEvents(ch <-chan provider.Event) []provider.Event {
	var events []provider.Event
	for ev := range ch {
		events = append(events, ev)
	}
	return events
}

func TestProvider_Stream_HappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "text/event-stream")
		f, ok := w.(http.Flusher)
		require.True(t, ok)

		fmt.Fprintf(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Hello\"}}]}\n\n")
		f.Flush()
		fmt.Fprintf(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\" world\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":2}}\n\n")
		f.Flush()
		fmt.Fprintf(w, "data: [DONE]\n\n")
		f.Flush()
	}))
	defer srv.Close()

	p := openai.New(srv.URL, "test-key")
	ch, err := p.Stream(context.Background(), provider.Request{
		Model:        "deepseek-chat",
		SystemPrompt: "you are helpful",
		Messages: []message.Message{
			{Role: message.RoleUser, Content: []message.Block{{Type: message.BlockText, Text: "hi"}}},
		},
	})
	require.NoError(t, err)
	events := collectEvents(ch)

	// Must have: EventStart, EventTextDelta "Hello", EventTextDelta " world",
	// EventBlockStop, EventMessageStop end_turn, EventUsage
	require.GreaterOrEqual(t, len(events), 5)

	require.Equal(t, provider.EventStart, events[0].Kind)

	textEvents := filterKind(events, provider.EventTextDelta)
	require.Len(t, textEvents, 2)
	require.Equal(t, "Hello", textEvents[0].Text)
	require.Equal(t, " world", textEvents[1].Text)

	stops := filterKind(events, provider.EventMessageStop)
	require.Len(t, stops, 1)
	require.Equal(t, "end_turn", stops[0].StopReason)

	usages := filterKind(events, provider.EventUsage)
	require.Len(t, usages, 1)
	require.Equal(t, 10, usages[0].Usage.InputTokens)
	require.Equal(t, 2, usages[0].Usage.OutputTokens)
}

func TestProvider_Stream_ToolCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		f, _ := w.(http.Flusher)

		// First chunk: tool_call start with ID and name
		fmt.Fprintf(w, `data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_abc","type":"function","function":{"name":"bash","arguments":""}}]}}]}`+"\n\n")
		f.Flush()
		// Second chunk: arguments delta
		fmt.Fprintf(w, `data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"cmd\":"}}]}}]}`+"\n\n")
		f.Flush()
		// Third chunk: more arguments + finish
		fmt.Fprintf(w, `data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"ls\"}"}}]},"finish_reason":"tool_calls"}]}`+"\n\n")
		f.Flush()
		fmt.Fprintf(w, "data: [DONE]\n\n")
		f.Flush()
	}))
	defer srv.Close()

	p := openai.New(srv.URL, "test-key")
	ch, err := p.Stream(context.Background(), provider.Request{
		Model:    "deepseek-chat",
		Messages: []message.Message{{Role: message.RoleUser, Content: []message.Block{{Type: message.BlockText, Text: "run ls"}}}},
	})
	require.NoError(t, err)
	events := collectEvents(ch)

	starts := filterKind(events, provider.EventToolUseStart)
	require.Len(t, starts, 1)
	require.Equal(t, "call_abc", starts[0].ToolUseID)
	require.Equal(t, "bash", starts[0].ToolName)

	deltas := filterKind(events, provider.EventToolInputDelta)
	require.GreaterOrEqual(t, len(deltas), 1)

	stops := filterKind(events, provider.EventMessageStop)
	require.Len(t, stops, 1)
	require.Equal(t, "tool_use", stops[0].StopReason)
}

func TestProvider_Stream_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()

	p := openai.New(srv.URL, "bad-key")
	_, err := p.Stream(context.Background(), provider.Request{
		Model:    "deepseek-chat",
		Messages: []message.Message{{Role: message.RoleUser, Content: []message.Block{{Type: message.BlockText, Text: "hi"}}}},
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "401")
}

func TestProvider_Stream_ContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		f, _ := w.(http.Flusher)
		for i := 0; i < 100; i++ {
			select {
			case <-r.Context().Done():
				return
			default:
				fmt.Fprintf(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"chunk\"}}]}\n\n")
				f.Flush()
				time.Sleep(10 * time.Millisecond)
			}
		}
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	p := openai.New(srv.URL, "test-key")
	ch, err := p.Stream(ctx, provider.Request{
		Model:    "deepseek-chat",
		Messages: []message.Message{{Role: message.RoleUser, Content: []message.Block{{Type: message.BlockText, Text: "hi"}}}},
	})
	require.NoError(t, err)

	// Read a couple of events then cancel.
	count := 0
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range ch {
			count++
			if count >= 2 {
				cancel()
			}
		}
	}()

	select {
	case <-done:
		// channel closed cleanly after cancel
	case <-time.After(5 * time.Second):
		t.Fatal("channel did not close after context cancel")
	}
}

func TestOllamaProvider_DefaultBaseURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "POST", r.Method)
		require.Equal(t, "/v1/chat/completions", r.URL.Path)
		// Ollama sentinel key is forwarded as Bearer token.
		require.Equal(t, "Bearer ollama", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "text/event-stream")
		f, _ := w.(http.Flusher)
		fmt.Fprintf(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hi from ollama\"},\"finish_reason\":\"stop\"}]}\n\n")
		f.Flush()
		fmt.Fprintf(w, "data: [DONE]\n\n")
		f.Flush()
	}))
	defer srv.Close()

	p := openai.New(srv.URL, "ollama")
	ch, err := p.Stream(context.Background(), provider.Request{
		Model:        "llama3",
		SystemPrompt: "test",
		Messages: []message.Message{
			{Role: message.RoleUser, Content: []message.Block{{Type: message.BlockText, Text: "hi"}}},
		},
	})
	require.NoError(t, err)

	var sawText bool
	for ev := range ch {
		if ev.Kind == provider.EventTextDelta && ev.Text == "hi from ollama" {
			sawText = true
		}
	}
	require.True(t, sawText, "expected text delta 'hi from ollama' from ollama-like server")
}

func filterKind(events []provider.Event, kind provider.EventKind) []provider.Event {
	var out []provider.Event
	for _, e := range events {
		if e.Kind == kind {
			out = append(out, e)
		}
	}
	return out
}

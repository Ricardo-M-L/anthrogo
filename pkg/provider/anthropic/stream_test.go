package anthropic

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/pkg/message"
	"github.com/ricardo/anthrogo/pkg/provider"
)

func TestProvider_Stream_LiveSmoke(t *testing.T) {
	if os.Getenv("ANTHROGO_LIVE") != "1" || os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("set ANTHROGO_LIVE=1 + ANTHROPIC_API_KEY to enable")
	}
	p := New("", "claude-haiku-4-5-20251001")
	ch, err := p.Stream(context.Background(), provider.Request{
		Messages:  []message.Message{message.Text(message.RoleUser, "Say 'pong' and nothing else.")},
		MaxTokens: 64,
	})
	require.NoError(t, err)
	var sb strings.Builder
	for ev := range ch {
		if ev.Kind == provider.EventTextDelta {
			sb.WriteString(ev.Text)
		}
	}
	require.Contains(t, strings.ToLower(sb.String()), "pong")
}

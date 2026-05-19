package query

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/internal/session"
	"github.com/ricardo/anthrogo/pkg/permissions"
	"github.com/ricardo/anthrogo/pkg/provider"
	"github.com/ricardo/anthrogo/pkg/provider/fake"
	"github.com/ricardo/anthrogo/pkg/tool"
)

func TestEngine_RecordHook_ReceivesAssistantAndUsage(t *testing.T) {
	fp := fake.New([]provider.Event{
		{Kind: provider.EventStart},
		{Kind: provider.EventTextDelta, Text: "hi"},
		{Kind: provider.EventUsage},
		{Kind: provider.EventMessageStop, StopReason: "end_turn"},
	})

	var mu sync.Mutex
	var kinds []session.Kind
	hook := func(r session.Record) {
		mu.Lock()
		defer mu.Unlock()
		kinds = append(kinds, r.Kind)
	}

	e := NewEngine(Config{
		Provider:    fp,
		Tools:       tool.NewRegistry(),
		Permissions: permissions.Empty(),
		Model:       "x",
		RecordHook:  hook,
	})
	for range e.SubmitMessage(context.Background(), "hello") {
	}

	mu.Lock()
	defer mu.Unlock()
	require.Contains(t, kinds, session.KindUserMessage)
	require.Contains(t, kinds, session.KindAssistantMessage)
	require.Contains(t, kinds, session.KindUsage)
	require.Contains(t, kinds, session.KindTurnComplete)
}

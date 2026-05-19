package tool

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTodoWrite_StoresList(t *testing.T) {
	tool := &TodoWrite{}
	_, err := tool.Call(context.Background(), map[string]any{
		"todos": []any{
			map[string]any{"content": "step 1", "status": "pending", "activeForm": "Doing step 1"},
			map[string]any{"content": "step 2", "status": "in_progress", "activeForm": "Doing step 2"},
		},
	}, &Context{})
	require.NoError(t, err)
	require.Len(t, tool.List(), 2)
	require.Equal(t, "in_progress", tool.List()[1].Status)
}

func TestTodoWrite_ReplacesList(t *testing.T) {
	tool := &TodoWrite{}
	_, _ = tool.Call(context.Background(), map[string]any{"todos": []any{
		map[string]any{"content": "a", "status": "pending"},
	}}, &Context{})
	_, _ = tool.Call(context.Background(), map[string]any{"todos": []any{
		map[string]any{"content": "b", "status": "pending"},
	}}, &Context{})
	require.Len(t, tool.List(), 1)
	require.Equal(t, "b", tool.List()[0].Content)
}

func TestTodoWrite_InvalidShape(t *testing.T) {
	res, _ := (&TodoWrite{}).Call(context.Background(), map[string]any{"todos": "nope"}, &Context{})
	require.True(t, res.IsError)
}

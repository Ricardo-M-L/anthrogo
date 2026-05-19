package tool

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAskUserQuestion_RequiresOptions(t *testing.T) {
	res, _ := (&AskUserQuestion{}).Call(context.Background(), map[string]any{
		"question": "X?",
	}, &Context{})
	require.True(t, res.IsError)
	require.Contains(t, res.Text, "options")
}

func TestAskUserQuestion_NoPromptSurface_Errors(t *testing.T) {
	res, _ := (&AskUserQuestion{}).Call(context.Background(), map[string]any{
		"question": "Continue?",
		"options": []any{
			map[string]any{"label": "Yes"},
			map[string]any{"label": "No"},
		},
	}, &Context{})
	require.True(t, res.IsError)
	require.Contains(t, res.Text, "interactive-only")
}

func TestAskUserQuestion_DelegatesToPromptSurface(t *testing.T) {
	tcx := &Context{RequestPrompt: func(_ string, req PromptRequest) (PromptResponse, error) {
		require.Equal(t, PromptQuestion, req.Kind)
		require.Equal(t, "Continue?", req.Question)
		require.Len(t, req.Options, 2)
		return PromptResponse{SelectedLabel: "Yes", Notes: "go ahead"}, nil
	}}
	res, err := (&AskUserQuestion{}).Call(context.Background(), map[string]any{
		"question": "Continue?",
		"options": []any{
			map[string]any{"label": "Yes", "description": "proceed"},
			map[string]any{"label": "No", "description": "stop"},
		},
	}, tcx)
	require.NoError(t, err)
	require.Contains(t, res.Text, "Yes")
	require.Contains(t, res.Text, "go ahead")
}

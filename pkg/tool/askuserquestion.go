package tool

import (
	"context"
	"fmt"
)

type AskUserQuestion struct{ DefaultPermission }

func (AskUserQuestion) Name() string                         { return "AskUserQuestion" }
func (AskUserQuestion) Description(context.Context) string   { return askUserQuestionDescription }
func (AskUserQuestion) UserFacingName(map[string]any) string { return "AskUserQuestion" }
func (AskUserQuestion) IsReadOnly() bool                     { return true }
func (AskUserQuestion) IsConcurrencySafe() bool              { return false }

func (AskUserQuestion) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"question": map[string]any{"type": "string"},
			"options": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"label":       map[string]any{"type": "string"},
						"description": map[string]any{"type": "string"},
					},
					"required": []string{"label"},
				},
				"minItems": 2,
				"maxItems": 4,
			},
		},
		"required": []string{"question", "options"},
	}
}

func (AskUserQuestion) Call(_ context.Context, input map[string]any, tcx *Context) (Result, error) {
	question, _ := input["question"].(string)
	if question == "" {
		return errResult("question is required"), nil
	}
	rawOpts, _ := input["options"].([]any)
	if len(rawOpts) < 2 {
		return errResult("options is required (2–4 entries)"), nil
	}
	opts := make([]PromptOption, 0, len(rawOpts))
	for _, raw := range rawOpts {
		m, _ := raw.(map[string]any)
		label, _ := m["label"].(string)
		desc, _ := m["description"].(string)
		if label == "" {
			return errResult("each option needs a non-empty label"), nil
		}
		opts = append(opts, PromptOption{Label: label, Description: desc})
	}
	if tcx == nil || tcx.RequestPrompt == nil {
		return errResult("askuserquestion: interactive-only; use REPL mode"), nil
	}
	resp, err := tcx.RequestPrompt("AskUserQuestion", PromptRequest{
		Kind:     PromptQuestion,
		Question: question,
		Options:  opts,
	})
	if err != nil {
		return errResult(err.Error()), nil
	}
	out := fmt.Sprintf("selected: %s", resp.SelectedLabel)
	if resp.Notes != "" {
		out += "\nnotes: " + resp.Notes
	}
	return Result{
		Type: ResultText, Text: out, ForLLM: out,
		Data: map[string]any{
			"selected_label": resp.SelectedLabel,
			"notes":          resp.Notes,
		},
	}, nil
}

const askUserQuestionDescription = `Ask the human partner a focused multiple-choice question with 2–4 options. Returns their selected label and optional free-text notes.`

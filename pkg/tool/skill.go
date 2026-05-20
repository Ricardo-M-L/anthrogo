package tool

import (
	"context"
	"fmt"

	"github.com/ricardo/anthrogo/pkg/skill"
)

// Skill is a model-invoked tool that returns the body of a registered skill.
type Skill struct {
	DefaultPermission
	registry *skill.Registry
}

func NewSkill(r *skill.Registry) *Skill { return &Skill{registry: r} }

func (*Skill) Name() string                       { return "Skill" }
func (*Skill) Description(context.Context) string { return skillDescription }
func (*Skill) UserFacingName(input map[string]any) string {
	if s, _ := input["skill"].(string); s != "" {
		return "Skill: " + s
	}
	return "Skill"
}
func (*Skill) IsReadOnly() bool        { return true }
func (*Skill) IsConcurrencySafe() bool { return true }

func (*Skill) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"skill": map[string]any{"type": "string", "description": "Name of the skill to invoke."},
			"args":  map[string]any{"type": "string", "description": "Optional free-text arguments."},
		},
		"required": []string{"skill"},
	}
}

func (s *Skill) Call(ctx context.Context, input map[string]any, _ *Context) (Result, error) {
	name, _ := input["skill"].(string)
	if name == "" {
		msg := "skill: missing 'skill' field"
		return Result{Type: ResultText, Text: msg, ForLLM: msg, IsError: true}, nil
	}
	if s.registry == nil {
		msg := "skill registry not configured"
		return Result{Type: ResultText, Text: msg, ForLLM: msg, IsError: true}, nil
	}
	sk, ok := s.registry.Get(name)
	if !ok {
		msg := fmt.Sprintf("Skill not found: %s", name)
		return Result{Type: ResultText, Text: msg, ForLLM: msg, IsError: true}, nil
	}
	return Result{
		Type:   ResultText,
		Text:   sk.Body,
		ForLLM: sk.Body,
		Data:   map[string]any{"name": sk.Name, "base_path": sk.BasePath, "source": sk.Source},
	}, nil
}

const skillDescription = `Invoke a registered Skill by name. The tool returns the skill's full instructions; you then follow them, using other tools (Read, Bash, etc.) as the skill directs. Available skills are listed in the system prompt.`

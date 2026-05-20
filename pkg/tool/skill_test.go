package tool

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/pkg/permissions"
	"github.com/ricardo/anthrogo/pkg/skill"
)

func TestSkill_ReturnsBody(t *testing.T) {
	r := skill.NewRegistry([]skill.Skill{
		{Name: "git-flow", Description: "x", Body: "DO THE THING", BasePath: "/p", Source: "home"},
	})
	tool := NewSkill(r)
	res, err := tool.Call(context.Background(), map[string]any{"skill": "git-flow"}, nil)
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Equal(t, "DO THE THING", res.ForLLM)
	require.Equal(t, "git-flow", res.Data["name"])
}

func TestSkill_UnknownReturnsError(t *testing.T) {
	r := skill.NewRegistry(nil)
	tool := NewSkill(r)
	res, _ := tool.Call(context.Background(), map[string]any{"skill": "nope"}, nil)
	require.True(t, res.IsError)
	require.Contains(t, res.ForLLM, "not found")
}

func TestSkill_MissingFieldReturnsError(t *testing.T) {
	tool := NewSkill(skill.NewRegistry(nil))
	res, _ := tool.Call(context.Background(), map[string]any{}, nil)
	require.True(t, res.IsError)
}

func TestSkill_PermissionDefersToGate(t *testing.T) {
	tool := NewSkill(skill.NewRegistry(nil))
	d := tool.Permission(context.Background(), nil)
	require.Equal(t, permissions.BehaviorAsk, d.Behavior)
}

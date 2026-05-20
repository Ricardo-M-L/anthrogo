package builtins

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/pkg/skill"
)

func TestSkills_EmptyList(t *testing.T) {
	h := newFakeHost()
	h.skills = skill.NewRegistry(nil)
	res, _ := Skills{}.Run(context.Background(), "", h)
	require.Contains(t, res.Text, "no skills")
}

func TestSkills_ListSorted(t *testing.T) {
	h := newFakeHost()
	h.skills = skill.NewRegistry([]skill.Skill{
		{Name: "b", Description: "B", Source: "home"},
		{Name: "a", Description: "A", Source: "cwd"},
	})
	res, _ := Skills{}.Run(context.Background(), "", h)
	// a before b
	require.Less(t, indexOf(res.Text, "a "), indexOf(res.Text, "b "))
	require.Contains(t, res.Text, "[home]")
	require.Contains(t, res.Text, "[cwd]")
}

func TestSkills_ShowKnown(t *testing.T) {
	h := newFakeHost()
	h.skills = skill.NewRegistry([]skill.Skill{
		{Name: "git-flow", Description: "use it", Body: "BODY", Source: "home"},
	})
	res, _ := Skills{}.Run(context.Background(), "show git-flow", h)
	require.Contains(t, res.Text, "BODY")
	require.Contains(t, res.Text, "git-flow")
}

func TestSkills_ShowUnknown(t *testing.T) {
	h := newFakeHost()
	h.skills = skill.NewRegistry(nil)
	res, _ := Skills{}.Run(context.Background(), "show nope", h)
	require.Contains(t, res.Text, "skill not found")
}

func TestSkills_UnknownSubcommand(t *testing.T) {
	h := newFakeHost()
	h.skills = skill.NewRegistry(nil)
	res, _ := Skills{}.Run(context.Background(), "garbage", h)
	require.Contains(t, res.Text, "usage:")
}

func indexOf(s, sub string) int {
	return strings.Index(s, sub)
}

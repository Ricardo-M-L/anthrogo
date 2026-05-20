package system

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/pkg/skill"
	"github.com/ricardo/anthrogo/pkg/subagent"
)

func TestBuildSystemPrompt_IncludesAllComponents(t *testing.T) {
	got := BuildSystemPrompt(Options{
		ToolNames:   []string{"Bash", "Read"},
		ClaudeMd:    "# project rules\n",
		GitStatus:   "Current branch: main\n",
		CurrentDate: "2026-05-17",
		Cwd:         "/work/anthrogo",
	})
	require.Contains(t, got, "anthrogo")          // header mentions product
	require.Contains(t, got, "Today's date is 2026-05-17")
	require.Contains(t, got, "/work/anthrogo")
	require.Contains(t, got, "Available tools")
	require.Contains(t, got, "Bash, Read")
	require.Contains(t, got, "# project rules")
	require.Contains(t, got, "Current branch: main")
}

func TestBuildSystemPrompt_OmitsEmptySections(t *testing.T) {
	got := BuildSystemPrompt(Options{
		ToolNames:   []string{"Read"},
		CurrentDate: "2026-05-17",
		Cwd:         "/x",
	})
	require.NotContains(t, got, "git status")
	require.NotContains(t, got, "<!-- from")
}

func TestBuildSystemPrompt_PlanModeAddsAddendum(t *testing.T) {
	got := BuildSystemPrompt(Options{Cwd: "/x", PlanModeOn: true})
	require.Contains(t, got, "Plan mode")
	require.Contains(t, got, "MUST NOT modify any file")

	got2 := BuildSystemPrompt(Options{Cwd: "/x", PlanModeOn: false})
	require.NotContains(t, got2, "Plan mode")
}

func TestBuildSystemPrompt_MentionsMCPWhenPresent(t *testing.T) {
	got := BuildSystemPrompt(Options{ToolNames: []string{"Read", "mcp__fs__read_file"}})
	require.Contains(t, got, "mcp__")
	require.Contains(t, got, "external MCP servers")
}

func TestBuildSystemPrompt_OmitsMCPWhenAbsent(t *testing.T) {
	got := BuildSystemPrompt(Options{ToolNames: []string{"Read", "Bash"}})
	require.NotContains(t, got, "external MCP servers")
}

func TestBuildSystemPrompt_ListsSkillsWhenPresent(t *testing.T) {
	got := BuildSystemPrompt(Options{
		Skills: []skill.Skill{
			{Name: "a", Description: "use when X"},
			{Name: "b", Description: "use when Y"},
		},
	})
	require.Contains(t, got, "Available skills")
	require.Contains(t, got, "- a: use when X")
	require.Contains(t, got, "- b: use when Y")
}

func TestBuildSystemPrompt_OmitsSkillsBlockWhenEmpty(t *testing.T) {
	got := BuildSystemPrompt(Options{})
	require.NotContains(t, got, "Available skills")
}

func TestBuildSystemPrompt_ListsSubagentsWhenPresent(t *testing.T) {
	got := BuildSystemPrompt(Options{
		Subagents: []subagent.Spec{
			{Name: "general-purpose", Description: "General-purpose agent for complex tasks."},
		},
	})
	require.Contains(t, got, "Available subagent types")
	require.Contains(t, got, "- general-purpose: General-purpose agent for complex tasks.")
}

func TestBuildSystemPrompt_OmitsSubagentsBlockWhenEmpty(t *testing.T) {
	got := BuildSystemPrompt(Options{})
	require.NotContains(t, got, "Available subagent types")
}

package system

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/ricardo/anthrogo/pkg/skill"
)

//go:embed prompts/core_v2_1_88.txt
var coreSystemPrompt string

//go:embed prompts/plan_mode_addendum.txt
var planModeAddendum string

// Options are everything BuildSystemPrompt needs. The CLI / TUI gathers these
// (via LoadClaudeMd, GitStatusSnapshot, etc.) and hands them off.
type Options struct {
	ToolNames   []string
	ClaudeMd    string
	GitStatus   string
	CurrentDate string
	Cwd         string
	PlanModeOn  bool
	Skills      []skill.Skill
}

// BuildSystemPrompt produces the prompt sent in `system` to the API. It's the
// Go analogue of upstream's `fetchSystemPromptParts` + `getSystemContext` +
// `getUserContext` joined.
func BuildSystemPrompt(opts Options) string {
	var b strings.Builder
	b.WriteString(coreSystemPrompt)
	if !strings.HasSuffix(coreSystemPrompt, "\n") {
		b.WriteByte('\n')
	}
	if opts.Cwd != "" {
		fmt.Fprintf(&b, "Working directory: %s\n", opts.Cwd)
	}
	if opts.CurrentDate != "" {
		fmt.Fprintf(&b, "Today's date is %s.\n", opts.CurrentDate)
	}
	if len(opts.ToolNames) > 0 {
		fmt.Fprintf(&b, "Available tools: %s\n", strings.Join(opts.ToolNames, ", "))
	}
	if hasMCPTool(opts.ToolNames) {
		b.WriteString("Tools prefixed `mcp__` are provided by external MCP servers and may behave differently from built-in tools.\n")
	}
	if len(opts.Skills) > 0 {
		b.WriteString("\nAvailable skills (invoke via the Skill tool, e.g. {\"skill\":\"git-flow\"}):\n")
		for _, sk := range opts.Skills {
			fmt.Fprintf(&b, "- %s: %s\n", sk.Name, sk.Description)
		}
	}
	if opts.PlanModeOn {
		b.WriteString("\n# Plan mode\n")
		b.WriteString(planModeAddendum)
	}
	if opts.GitStatus != "" {
		b.WriteString("\n# Git\n")
		b.WriteString(opts.GitStatus)
	}
	if opts.ClaudeMd != "" {
		b.WriteString("\n# Project memory (CLAUDE.md)\n")
		b.WriteString(opts.ClaudeMd)
	}
	return b.String()
}

func hasMCPTool(names []string) bool {
	for _, n := range names {
		if strings.HasPrefix(n, "mcp__") {
			return true
		}
	}
	return false
}

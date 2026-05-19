package permissions

import (
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// Source labels where the rule came from (settings.json scope analogue).
type Source string

const (
	SourceUser    Source = "user"
	SourceProject Source = "project"
	SourceManaged Source = "managed"
	SourceCLI     Source = "cli"
)

// Rule represents one allow/deny/ask entry.
//
//	{tool: "Bash", match: "git status*"}
//	{tool: "Read", match: "/tmp/**"}
//	{tool: "Read"}  // no match → applies to any input for this tool
type Rule struct {
	Tool    string `yaml:"tool" json:"tool"`
	Pattern string `yaml:"match,omitempty" json:"match,omitempty"`
	Source  Source `yaml:"-" json:"-"`
}

// RulesBySource groups rules by where they were loaded from.
type RulesBySource map[Source][]Rule

// Match returns true if this rule applies to a given tool invocation.
// Matching strategy (M1):
//   - Tool name must match exactly.
//   - If Match is empty, the rule applies to any input.
//   - If the input has a "path" or "file_path" field, doublestar-glob it.
//   - If the input has a "command" field, prefix-match (`*` = trailing wildcard).
//   - Anything else falls through to false (conservative).
func (r Rule) Match(tool string, input map[string]any) bool {
	if r.Tool != tool {
		return false
	}
	if r.Pattern == "" {
		return true
	}
	if v, ok := stringField(input, "path", "file_path"); ok {
		ok, _ := doublestar.PathMatch(r.Pattern, v)
		return ok
	}
	if v, ok := stringField(input, "command"); ok {
		return matchCommandGlob(r.Pattern, v)
	}
	return false
}

func stringField(input map[string]any, keys ...string) (string, bool) {
	for _, k := range keys {
		if v, ok := input[k]; ok {
			if s, ok := v.(string); ok {
				return s, true
			}
		}
	}
	return "", false
}

// matchCommandGlob: bare prefix when ends in '*', else exact.
func matchCommandGlob(pattern, cmd string) bool {
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(cmd, strings.TrimSuffix(pattern, "*"))
	}
	return pattern == cmd
}

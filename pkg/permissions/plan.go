package permissions

import "strings"

// IsWriteTool reports whether a tool name is in the plan-mode write list.
// All mcp__* tools are treated as write tools.
func IsWriteTool(name string) bool {
	switch name {
	case "Write", "Edit", "NotebookEdit":
		return true
	}
	return strings.HasPrefix(name, "mcp__")
}

var readOnlyBashPrefixes = []string{
	"ls", "cat", "grep", "rg", "ag", "find", "git status", "git diff",
	"git log", "git show", "git blame", "git branch", "git remote",
	"pwd", "wc", "stat", "head", "tail", "file", "which", "whereis",
}

// IsReadOnlyBashCommand checks whether a bash command starts with one of the
// allowlisted read-only prefixes and contains no redirect / pipe metacharacters.
func IsReadOnlyBashCommand(cmd string) bool {
	trimmed := strings.TrimSpace(cmd)
	for _, m := range []string{">", "<", "|", "&", ";"} {
		if strings.Contains(trimmed, m) {
			return false
		}
	}
	for _, p := range readOnlyBashPrefixes {
		if trimmed == p || strings.HasPrefix(trimmed, p+" ") {
			return true
		}
	}
	return false
}

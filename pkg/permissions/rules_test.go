package permissions

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRule_NoMatchPattern_MatchesAnyInput(t *testing.T) {
	r := Rule{Tool: "Read"}
	require.True(t, r.Match("Read", map[string]any{"path": "/a"}))
}

func TestRule_MatchPattern_GlobOnPath(t *testing.T) {
	r := Rule{Tool: "Read", Pattern: "/tmp/**"}
	require.True(t, r.Match("Read", map[string]any{"path": "/tmp/foo/bar.txt"}))
	require.False(t, r.Match("Read", map[string]any{"path": "/etc/passwd"}))
}

func TestRule_MatchPattern_ShellPrefix(t *testing.T) {
	r := Rule{Tool: "Bash", Pattern: "git status*"}
	require.True(t, r.Match("Bash", map[string]any{"command": "git status --short"}))
	require.False(t, r.Match("Bash", map[string]any{"command": "rm -rf /"}))
}

func TestRule_DifferentTool_NoMatch(t *testing.T) {
	r := Rule{Tool: "Read"}
	require.False(t, r.Match("Bash", map[string]any{"command": "ls"}))
}

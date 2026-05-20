package permissions

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsWriteTool(t *testing.T) {
	require.True(t, IsWriteTool("Write"))
	require.True(t, IsWriteTool("Edit"))
	require.True(t, IsWriteTool("NotebookEdit"))
	require.False(t, IsWriteTool("Read"))
	require.False(t, IsWriteTool("Grep"))
}

func TestIsWriteTool_MatchesMCPPrefix(t *testing.T) {
	require.True(t, IsWriteTool("mcp__fs__write_file"))
	require.True(t, IsWriteTool("mcp__github__create_pull_request"))
	require.False(t, IsWriteTool("Read"))
	require.False(t, IsWriteTool("Bash"))
	require.True(t, IsWriteTool("Write"))
}

func TestIsReadOnlyBashCommand(t *testing.T) {
	for _, cmd := range []string{
		"ls", "ls -la", "cat foo.txt", "grep -r foo .", "rg foo",
		"git status", "git diff --stat", "git log --oneline", "pwd",
		"head -100 file", "tail -f file", "wc -l file", "find . -name '*.go'",
		"stat foo",
	} {
		require.True(t, IsReadOnlyBashCommand(cmd), cmd)
	}
	for _, cmd := range []string{
		"rm foo", "cp a b", "git push", "make build", "npm install",
		"echo x > foo", "cat foo | tee bar", "go build ./...",
	} {
		require.False(t, IsReadOnlyBashCommand(cmd), cmd)
	}
}

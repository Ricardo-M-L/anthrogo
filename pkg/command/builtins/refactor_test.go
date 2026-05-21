package builtins

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRefactor_NoArgs_ShowsUsage(t *testing.T) {
	h := newFakeHost()
	res, err := (Refactor{}).Run(context.Background(), "", h)
	require.NoError(t, err)
	assert.Contains(t, res.Text, "usage")
	assert.Nil(t, res.AgentTask)
}

func TestRefactor_NoSeparator_ShowsUsage(t *testing.T) {
	h := newFakeHost()
	res, err := (Refactor{}).Run(context.Background(), "**/*.go rename Foo to Bar", h)
	require.NoError(t, err)
	assert.Contains(t, res.Text, "usage")
	assert.Nil(t, res.AgentTask)
}

func TestRefactor_EmptyPattern_ShowsUsage(t *testing.T) {
	h := newFakeHost()
	// separator present but pattern empty
	res, err := (Refactor{}).Run(context.Background(), " -- rename Foo", h)
	require.NoError(t, err)
	assert.Contains(t, res.Text, "usage")
}

func TestRefactor_EmptyInstruction_ShowsUsage(t *testing.T) {
	h := newFakeHost()
	res, err := (Refactor{}).Run(context.Background(), "**/*.go -- ", h)
	require.NoError(t, err)
	assert.Contains(t, res.Text, "usage")
}

func TestRefactor_NoMatches_ReturnsCleanMessage(t *testing.T) {
	dir := t.TempDir()
	h := newFakeHost()
	h.cwd = dir

	res, err := (Refactor{}).Run(context.Background(), "**/*.go -- rename Foo to Bar", h)
	require.NoError(t, err)
	assert.Contains(t, res.Text, "no files match")
	assert.Nil(t, res.AgentTask)
}

func TestRefactor_TooManyMatches_BlocksWithGuidance(t *testing.T) {
	dir := t.TempDir()
	// Create 51 files (> maxFiles=50).
	for i := 0; i < 51; i++ {
		name := filepath.Join(dir, fmt.Sprintf("file%03d.go", i))
		require.NoError(t, os.WriteFile(name, []byte("package main\n"), 0644))
	}
	h := newFakeHost()
	h.cwd = dir

	res, err := (Refactor{}).Run(context.Background(), "*.go -- rename something", h)
	require.NoError(t, err)
	assert.Contains(t, res.Text, "too many matches")
	assert.Contains(t, res.Text, "narrow")
	assert.Nil(t, res.AgentTask)
}

func TestRefactor_HappyPath_BuildsPromptAndAgentTask(t *testing.T) {
	dir := t.TempDir()
	// Create a few small Go files.
	files := []struct {
		name    string
		content string
	}{
		{"alpha.go", "package main\nfunc Foo() {}\n"},
		{"beta.go", "package main\nfunc Bar() {}\n"},
	}
	for _, f := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, f.name), []byte(f.content), 0644))
	}

	// fakeHost has no engine (nil), so Run returns AgentTask without running subagent.
	h := newFakeHost()
	h.cwd = dir

	const instruction = "rename Foo to FooRenamed everywhere"
	res, err := (Refactor{}).Run(context.Background(), "*.go -- "+instruction, h)
	require.NoError(t, err)

	// AgentTask must be set (no engine → deferred dispatch path).
	require.NotNil(t, res.AgentTask, "AgentTask should be set when engine is nil")
	assert.Equal(t, "refactor", res.AgentTask.SubagentType)
	assert.Equal(t, []string{"Read", "Edit", "Write", "Glob", "Grep"}, res.AgentTask.ToolAllowlist)
	assert.Contains(t, res.AgentTask.Description, "refactor 2 file(s)")
	assert.Contains(t, res.AgentTask.Prompt, instruction)

	// Both file paths must appear in the prompt body.
	assert.Contains(t, res.AgentTask.Prompt, filepath.Join(dir, "alpha.go"))
	assert.Contains(t, res.AgentTask.Prompt, filepath.Join(dir, "beta.go"))

	// Result.Text should mention dispatching.
	assert.Contains(t, res.Text, "dispatching")
}

func TestRefactor_TotalBytesWarning(t *testing.T) {
	dir := t.TempDir()
	// Create files whose total exceeds 500 KB.
	bigContent := strings.Repeat("x", 200*1024) // 200 KB each
	for i := 0; i < 3; i++ {
		name := filepath.Join(dir, fmt.Sprintf("big%d.go", i))
		require.NoError(t, os.WriteFile(name, []byte(bigContent), 0644))
	}
	h := newFakeHost()
	h.cwd = dir

	res, err := (Refactor{}).Run(context.Background(), "*.go -- do something", h)
	require.NoError(t, err)
	require.NotNil(t, res.AgentTask)
	assert.Contains(t, res.AgentTask.Prompt, "WARNING")
}

func TestRefactor_GlobError_ReturnsMessage(t *testing.T) {
	h := newFakeHost()
	h.cwd = t.TempDir()

	// An invalid glob pattern causes doublestar.Glob to error.
	res, err := (Refactor{}).Run(context.Background(), "[invalid -- do it", h)
	require.NoError(t, err)
	// Either "glob error" or "no files match" is acceptable depending on
	// whether the pattern error is detected at glob time or silently returns 0.
	assert.True(t,
		strings.Contains(res.Text, "glob error") || strings.Contains(res.Text, "no files match"),
		"expected glob error or no-match message, got: %q", res.Text,
	)
}

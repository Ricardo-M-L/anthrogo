package plugin

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/internal/hooks"
)

func testdata(rel string) string {
	abs, err := filepath.Abs("testdata/" + rel)
	if err != nil {
		panic(err)
	}
	return abs
}

func TestLoadAll_ValidHome(t *testing.T) {
	home := testdata("valid-home")
	plugins, warnings, err := LoadAll(home, "")
	require.NoError(t, err)

	// Filter warnings about bad plugins we don't have in this root.
	require.Len(t, plugins, 1)
	p := plugins[0]
	assert.Equal(t, "git-tools", p.Name)
	assert.Equal(t, "0.1.0", p.Manifest.Version)
	assert.Equal(t, "home", p.Source)

	// Commands
	require.Len(t, p.Commands, 1)
	assert.Equal(t, "/new-branch", p.Commands[0].Name())

	// Skills
	require.Len(t, p.Skills, 1)
	assert.Equal(t, "git-flow", p.Skills[0].Name)
	assert.Equal(t, "Use when starting a feature branch", p.Skills[0].Description)

	// MCP servers — key namespaced
	require.Contains(t, p.MCPServers, "git-tools:fs")

	// Hooks — path is absolute
	require.Len(t, p.Hooks.PreToolUse, 1)
	hookCmd := p.Hooks.PreToolUse[0].Command
	assert.True(t, filepath.IsAbs(hookCmd), "hook command should be absolute, got %q", hookCmd)
	assert.True(t, strings.HasSuffix(hookCmd, "audit.sh"), "expected audit.sh, got %q", hookCmd)
	assert.Equal(t, "Bash", p.Hooks.PreToolUse[0].Matcher)

	_ = warnings // may have none
}

func TestLoadAll_BadNoManifest(t *testing.T) {
	plugins, warnings, err := LoadAll(testdata("bad-no-manifest"), "")
	require.NoError(t, err)
	assert.Empty(t, plugins)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "ghost")
	assert.Contains(t, warnings[0], "no plugin.yaml")
}

func TestLoadAll_BadNameMismatch(t *testing.T) {
	plugins, warnings, err := LoadAll(testdata("bad-name-mismatch"), "")
	require.NoError(t, err)
	assert.Empty(t, plugins)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "doesn't match directory")
}

func TestLoadAll_BadSkillRef(t *testing.T) {
	plugins, warnings, err := LoadAll(testdata("bad-skill-ref"), "")
	require.NoError(t, err)
	// The plugin itself should still load (with empty skills), just a warning
	require.Len(t, plugins, 1)
	assert.Equal(t, "y", plugins[0].Name)
	assert.Empty(t, plugins[0].Skills)
	assert.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "no SKILL.md")
}

func TestLoadAll_CwdOverridesHome(t *testing.T) {
	home := testdata("valid-home")
	// Use the same valid-home as cwd root — same plugin name "git-tools" exists in both.
	cwd := testdata("valid-home")
	plugins, warnings, err := LoadAll(home, cwd)
	require.NoError(t, err)
	// Still one plugin (cwd overrides home)
	require.Len(t, plugins, 1)
	assert.Equal(t, "git-tools", plugins[0].Name)
	// Should have override warning
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "overrides home version") {
			found = true
		}
	}
	assert.True(t, found, "expected override warning, got %v", warnings)
}

func TestLoadAll_MissingRoot(t *testing.T) {
	plugins, warnings, err := LoadAll("/nonexistent-root-abc123", "")
	require.NoError(t, err)
	assert.Empty(t, plugins)
	assert.Empty(t, warnings)
}

func TestAbsolutizeHooks_AlreadyAbsolute(t *testing.T) {
	cfg := hooks.Config{
		PreToolUse: []hooks.Spec{{Matcher: "Bash", Command: "/abs/path/x.sh"}},
		Stop:       []hooks.Spec{{Command: "relative/y.sh"}},
	}
	got := absolutizeHooks(cfg, "/plugin/base")
	require.Equal(t, "/abs/path/x.sh", got.PreToolUse[0].Command) // unchanged
	require.Equal(t, "/plugin/base/relative/y.sh", got.Stop[0].Command)
}

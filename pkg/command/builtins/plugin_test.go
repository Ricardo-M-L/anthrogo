package builtins

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/pkg/plugin"
)

// pluginTestdata returns an absolute path to testdata within the plugin package.
// Test working directory is pkg/command/builtins, so 3 levels up reaches the
// repo root.
func pluginTestdata(t *testing.T, rel string) string {
	t.Helper()
	abs, err := filepath.Abs("../../../pkg/plugin/testdata/" + rel)
	require.NoError(t, err)
	return abs
}

func newPluginHost(reg *plugin.Registry) *fakeHost {
	h := newFakeHost()
	h.plugins = reg
	return h
}

func loadGitTools(t *testing.T) *plugin.Registry {
	t.Helper()
	home := pluginTestdata(t, "valid-home")
	plugins, _, err := plugin.LoadAll(home, "")
	require.NoError(t, err)
	return plugin.NewRegistry(plugins)
}

func TestPlugin_List_Empty(t *testing.T) {
	reg := plugin.NewRegistry(nil)
	h := newPluginHost(reg)
	cmd := Plugin{HomeRoot: t.TempDir(), CwdRoot: ""}
	res, err := cmd.Run(context.Background(), "", h)
	require.NoError(t, err)
	assert.Contains(t, res.Text, "no plugins installed")
}

func TestPlugin_List_WithPlugins(t *testing.T) {
	reg := loadGitTools(t)
	h := newPluginHost(reg)
	cmd := Plugin{}
	res, err := cmd.Run(context.Background(), "", h)
	require.NoError(t, err)
	assert.Contains(t, res.Text, "git-tools")
}

func TestPlugin_Info_Found(t *testing.T) {
	reg := loadGitTools(t)
	h := newPluginHost(reg)
	cmd := Plugin{}
	res, err := cmd.Run(context.Background(), "info git-tools", h)
	require.NoError(t, err)
	assert.Contains(t, res.Text, "git-tools")
	assert.Contains(t, res.Text, "commands:")
	assert.Contains(t, res.Text, "skills:")
}

func TestPlugin_Info_NotFound(t *testing.T) {
	reg := plugin.NewRegistry(nil)
	h := newPluginHost(reg)
	cmd := Plugin{}
	res, err := cmd.Run(context.Background(), "info nonexistent", h)
	require.NoError(t, err)
	assert.Contains(t, res.Text, "not found")
}

func TestPlugin_Reload(t *testing.T) {
	home := pluginTestdata(t, "valid-home")
	reg := plugin.NewRegistry(nil)
	h := newPluginHost(reg)
	cmd := Plugin{HomeRoot: home, CwdRoot: ""}
	res, err := cmd.Run(context.Background(), "reload", h)
	require.NoError(t, err)
	assert.Contains(t, res.Text, "reloaded plugins")
	assert.Contains(t, res.Text, "note:")
}

func TestPlugin_Install(t *testing.T) {
	src := pluginTestdata(t, "valid-home/git-tools")
	destRoot := t.TempDir()
	reg := plugin.NewRegistry(nil)
	h := newPluginHost(reg)
	cmd := Plugin{HomeRoot: destRoot}
	res, err := cmd.Run(context.Background(), "install "+src, h)
	require.NoError(t, err)
	assert.Contains(t, res.Text, "installed git-tools")
}

func TestPlugin_Remove(t *testing.T) {
	src := pluginTestdata(t, "valid-home/git-tools")
	destRoot := t.TempDir()
	reg := plugin.NewRegistry(nil)
	h := newPluginHost(reg)
	cmd := Plugin{HomeRoot: destRoot}
	// Install first
	_, err := cmd.Run(context.Background(), "install "+src, h)
	require.NoError(t, err)
	// Then remove
	res, err := cmd.Run(context.Background(), "remove git-tools", h)
	require.NoError(t, err)
	assert.Contains(t, res.Text, "removed git-tools")
}

func TestPlugin_UnknownSubcommand(t *testing.T) {
	reg := plugin.NewRegistry(nil)
	h := newPluginHost(reg)
	cmd := Plugin{}
	res, err := cmd.Run(context.Background(), "blorp", h)
	require.NoError(t, err)
	assert.Contains(t, res.Text, "usage:")
}

func TestPlugin_NilRegistry(t *testing.T) {
	h := newFakeHost() // h.plugins is nil
	cmd := Plugin{}
	res, err := cmd.Run(context.Background(), "", h)
	require.NoError(t, err)
	assert.Contains(t, res.Text, "no plugin registry")
}

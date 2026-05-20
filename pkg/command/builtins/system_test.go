package builtins

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// makeSystem builds a System with explicit home and project overlay paths
// rooted in tmpHome and tmpCwd respectively.
func makeSystem(tmpHome, tmpCwd string) System {
	return System{
		HomeOverlayPath:    filepath.Join(tmpHome, ".anthrogo", "system_overlay.md"),
		ProjectOverlayPath: filepath.Join(tmpCwd, ".anthrogo", "system_overlay.md"),
	}
}

func TestSystem_ShowEmpty(t *testing.T) {
	tmpHome := t.TempDir()
	tmpCwd := t.TempDir()
	h := newFakeHost()
	res, err := makeSystem(tmpHome, tmpCwd).Run(context.Background(), "show", h)
	require.NoError(t, err)
	require.Contains(t, res.Text, "Home overlay")
	require.Contains(t, res.Text, "Project overlay")
	require.Contains(t, res.Text, "(empty)")
}

func TestSystem_ShowWithOverlay(t *testing.T) {
	tmpHome := t.TempDir()
	tmpCwd := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpHome, ".anthrogo"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpHome, ".anthrogo", "system_overlay.md"), []byte("HOME_OVERLAY"), 0o644))
	h := newFakeHost()
	res, _ := makeSystem(tmpHome, tmpCwd).Run(context.Background(), "show", h)
	require.Contains(t, res.Text, "HOME_OVERLAY")
}

func TestSystem_EditCreatesSeedFile(t *testing.T) {
	tmpHome := t.TempDir()
	tmpCwd := t.TempDir()
	h := newFakeHost()
	res, _ := makeSystem(tmpHome, tmpCwd).Run(context.Background(), "edit", h)
	require.Contains(t, res.Text, "opening")
	require.NotNil(t, res.ExecCmd)
	require.NotNil(t, res.ExecCmd.Cmd)
	_, err := os.Stat(filepath.Join(tmpHome, ".anthrogo", "system_overlay.md"))
	require.NoError(t, err)
}

func TestSystem_ResetRemovesFile(t *testing.T) {
	tmpHome := t.TempDir()
	tmpCwd := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpHome, ".anthrogo"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpHome, ".anthrogo", "system_overlay.md"), []byte("x"), 0o644))
	h := newFakeHost()
	res, _ := makeSystem(tmpHome, tmpCwd).Run(context.Background(), "reset", h)
	require.Contains(t, res.Text, "reset")
	_, err := os.Stat(filepath.Join(tmpHome, ".anthrogo", "system_overlay.md"))
	require.True(t, os.IsNotExist(err))
}

func TestSystem_UnknownSubcommand(t *testing.T) {
	h := newFakeHost()
	res, _ := System{}.Run(context.Background(), "garbage", h)
	require.Contains(t, res.Text, "usage:")
}

func TestSystem_ShowBothLayers(t *testing.T) {
	tmpHome := t.TempDir()
	tmpCwd := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpHome, ".anthrogo"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpHome, ".anthrogo", "system_overlay.md"), []byte("HOME_CONTENT"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpCwd, ".anthrogo"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpCwd, ".anthrogo", "system_overlay.md"), []byte("PROJECT_CONTENT"), 0o644))
	h := newFakeHost()
	res, err := makeSystem(tmpHome, tmpCwd).Run(context.Background(), "show", h)
	require.NoError(t, err)
	require.Contains(t, res.Text, "Home overlay")
	require.Contains(t, res.Text, "Project overlay")
	require.Contains(t, res.Text, "HOME_CONTENT")
	require.Contains(t, res.Text, "PROJECT_CONTENT")
	// Project overlay section appears after home overlay section.
	homeIdx := strings.Index(res.Text, "Home overlay")
	projIdx := strings.Index(res.Text, "Project overlay")
	require.GreaterOrEqual(t, homeIdx, 0)
	require.GreaterOrEqual(t, projIdx, 0)
	require.Less(t, homeIdx, projIdx)
}

func TestSystem_EditProject(t *testing.T) {
	tmpHome := t.TempDir()
	tmpCwd := t.TempDir()
	h := newFakeHost()
	res, err := makeSystem(tmpHome, tmpCwd).Run(context.Background(), "edit project", h)
	require.NoError(t, err)
	require.Contains(t, res.Text, "opening")
	require.NotNil(t, res.ExecCmd)
	require.NotNil(t, res.ExecCmd.Cmd)
	_, err = os.Stat(filepath.Join(tmpCwd, ".anthrogo", "system_overlay.md"))
	require.NoError(t, err)
}

func TestSystem_ResetProject(t *testing.T) {
	tmpHome := t.TempDir()
	tmpCwd := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpCwd, ".anthrogo"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpCwd, ".anthrogo", "system_overlay.md"), []byte("proj"), 0o644))
	h := newFakeHost()
	res, err := makeSystem(tmpHome, tmpCwd).Run(context.Background(), "reset project", h)
	require.NoError(t, err)
	require.Contains(t, res.Text, "project overlay reset")
	_, err = os.Stat(filepath.Join(tmpCwd, ".anthrogo", "system_overlay.md"))
	require.True(t, os.IsNotExist(err))
}

func TestSystem_EditReturnsExecCmd(t *testing.T) {
	tmpHome := t.TempDir()
	h := newFakeHost()
	res, _ := System{
		HomeOverlayPath:    filepath.Join(tmpHome, ".anthrogo", "system_overlay.md"),
		ProjectOverlayPath: filepath.Join(tmpHome, "proj", ".anthrogo", "system_overlay.md"),
	}.Run(context.Background(), "edit", h)
	require.NotNil(t, res.ExecCmd)
	require.NotNil(t, res.ExecCmd.Cmd)
	require.Contains(t, res.Text, "opening")
}


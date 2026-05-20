package builtins

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSystem_ShowEmpty(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	h := newFakeHost()
	res, err := System{}.Run(context.Background(), "show", h)
	require.NoError(t, err)
	require.Contains(t, res.Text, "Persistent user overlay")
	require.Contains(t, res.Text, "empty")
}

func TestSystem_ShowWithOverlay(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	require.NoError(t, os.MkdirAll(filepath.Join(tmpHome, ".anthrogo"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpHome, ".anthrogo", "system_overlay.md"), []byte("OVERLAY!"), 0o644))
	h := newFakeHost()
	res, _ := System{}.Run(context.Background(), "show", h)
	require.Contains(t, res.Text, "OVERLAY!")
}

func TestSystem_EditCreatesSeedFile(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	h := newFakeHost()
	res, _ := System{}.Run(context.Background(), "edit", h)
	require.Contains(t, res.Text, "Edit your overlay")
	_, err := os.Stat(filepath.Join(tmpHome, ".anthrogo", "system_overlay.md"))
	require.NoError(t, err)
}

func TestSystem_ResetRemovesFile(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	require.NoError(t, os.MkdirAll(filepath.Join(tmpHome, ".anthrogo"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpHome, ".anthrogo", "system_overlay.md"), []byte("x"), 0o644))
	h := newFakeHost()
	res, _ := System{}.Run(context.Background(), "reset", h)
	require.Contains(t, res.Text, "reset")
	_, err := os.Stat(filepath.Join(tmpHome, ".anthrogo", "system_overlay.md"))
	require.True(t, os.IsNotExist(err))
}

func TestSystem_UnknownSubcommand(t *testing.T) {
	h := newFakeHost()
	res, _ := System{}.Run(context.Background(), "garbage", h)
	require.Contains(t, res.Text, "usage:")
}

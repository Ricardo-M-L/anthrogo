package builtins

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHook_ListEmpty(t *testing.T) {
	root := t.TempDir()
	h := Hook{HomeRoot: root}
	res, err := h.Run(context.Background(), "list", nil)
	require.NoError(t, err)
	require.Contains(t, res.Text, "no hook bundles")
}

func TestHook_ListMissingRoot(t *testing.T) {
	root := filepath.Join(t.TempDir(), "nonexistent")
	h := Hook{HomeRoot: root}
	res, err := h.Run(context.Background(), "", nil)
	require.NoError(t, err)
	require.Contains(t, res.Text, "no hook bundles")
}

func TestHook_InstallLocalDir(t *testing.T) {
	src := t.TempDir()
	err := os.WriteFile(
		filepath.Join(src, "hook.yaml"),
		[]byte("name: bash-audit\ndescription: Audit bash\nevent: PreToolUse\n"),
		0o644,
	)
	require.NoError(t, err)
	// Also add a hook.sh to verify tree copy.
	require.NoError(t, os.WriteFile(filepath.Join(src, "hook.sh"), []byte("#!/bin/sh\n"), 0o755))

	root := t.TempDir()
	h := Hook{HomeRoot: root}
	res, err := h.Run(context.Background(), "install "+src, nil)
	require.NoError(t, err)
	require.Contains(t, res.Text, "installed hook bundle: bash-audit")

	// hook.yaml and hook.sh should be present under root/bash-audit/.
	_, statErr := os.Stat(filepath.Join(root, "bash-audit", "hook.yaml"))
	require.NoError(t, statErr)
	_, statErr = os.Stat(filepath.Join(root, "bash-audit", "hook.sh"))
	require.NoError(t, statErr)
}

func TestHook_InstallAlreadyExists(t *testing.T) {
	src := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(src, "hook.yaml"),
		[]byte("name: bash-audit\n"),
		0o644,
	))

	root := t.TempDir()
	h := Hook{HomeRoot: root}
	_, _ = h.Run(context.Background(), "install "+src, nil)
	res, err := h.Run(context.Background(), "install "+src, nil)
	require.NoError(t, err)
	require.Contains(t, res.Text, "already exists")
}

func TestHook_InstallNoHookYAML(t *testing.T) {
	src := t.TempDir()
	root := t.TempDir()
	h := Hook{HomeRoot: root}
	res, err := h.Run(context.Background(), "install "+src, nil)
	require.NoError(t, err)
	require.Contains(t, res.Text, "no hook.yaml")
}

func TestHook_InstallEmptyName(t *testing.T) {
	src := t.TempDir()
	require.NoError(t, os.WriteFile(
		filepath.Join(src, "hook.yaml"),
		[]byte("name: \n"),
		0o644,
	))
	root := t.TempDir()
	h := Hook{HomeRoot: root}
	res, err := h.Run(context.Background(), "install "+src, nil)
	require.NoError(t, err)
	require.Contains(t, res.Text, "empty hook name")
}

func TestHook_InstallURLDeferred(t *testing.T) {
	root := t.TempDir()
	h := Hook{HomeRoot: root}
	res, err := h.Run(context.Background(), "install https://example.com/hook.tar.gz", nil)
	require.NoError(t, err)
	require.Contains(t, res.Text, "deferred")
}

func TestHook_InstallGitDeferred(t *testing.T) {
	root := t.TempDir()
	h := Hook{HomeRoot: root}
	res, err := h.Run(context.Background(), "install git+https://github.com/example/hook", nil)
	require.NoError(t, err)
	require.Contains(t, res.Text, "deferred")
}

func TestHook_Remove(t *testing.T) {
	root := t.TempDir()
	bundleDir := filepath.Join(root, "bash-audit")
	require.NoError(t, os.MkdirAll(bundleDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bundleDir, "hook.yaml"), []byte("name: bash-audit\n"), 0o644))

	h := Hook{HomeRoot: root}
	res, err := h.Run(context.Background(), "remove bash-audit", nil)
	require.NoError(t, err)
	require.Contains(t, res.Text, "removed hook bundle: bash-audit")

	_, statErr := os.Stat(bundleDir)
	require.True(t, os.IsNotExist(statErr))
}

func TestHook_RemoveNotExist(t *testing.T) {
	root := t.TempDir()
	h := Hook{HomeRoot: root}
	res, err := h.Run(context.Background(), "remove nonexistent", nil)
	require.NoError(t, err)
	require.Contains(t, res.Text, "remove:")
}

func TestHook_ListShowsBundles(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"audit", "notify"} {
		d := filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(d, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(d, "hook.yaml"), []byte("name: "+name+"\n"), 0o644))
	}

	h := Hook{HomeRoot: root}
	res, err := h.Run(context.Background(), "list", nil)
	require.NoError(t, err)
	require.Contains(t, res.Text, "audit")
	require.Contains(t, res.Text, "notify")
}

func TestHook_UnknownSubcommand(t *testing.T) {
	root := t.TempDir()
	h := Hook{HomeRoot: root}
	res, err := h.Run(context.Background(), "garbage", nil)
	require.NoError(t, err)
	require.Contains(t, res.Text, "usage:")
}

func TestHook_InstallSrcNotDir(t *testing.T) {
	root := t.TempDir()
	f := filepath.Join(root, "file.yaml")
	require.NoError(t, os.WriteFile(f, []byte("x"), 0o644))

	h := Hook{HomeRoot: t.TempDir()}
	res, err := h.Run(context.Background(), "install "+f, nil)
	require.NoError(t, err)
	require.Contains(t, res.Text, "must be a directory")
}

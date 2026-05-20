package plugin

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistry_ListSorted(t *testing.T) {
	r := NewRegistry([]Plugin{
		{Name: "zebra"},
		{Name: "alpha"},
		{Name: "mango"},
	})
	list := r.List()
	require.Len(t, list, 3)
	assert.Equal(t, "alpha", list[0].Name)
	assert.Equal(t, "mango", list[1].Name)
	assert.Equal(t, "zebra", list[2].Name)
}

func TestRegistry_GetMiss(t *testing.T) {
	r := NewRegistry(nil)
	_, ok := r.Get("nope")
	assert.False(t, ok)
}

func TestRegistry_Reload(t *testing.T) {
	home := testdata("valid-home")
	r := NewRegistry(nil)
	assert.Empty(t, r.List())

	warnings, err := r.Reload(home, "")
	require.NoError(t, err)
	_ = warnings
	list := r.List()
	require.Len(t, list, 1)
	assert.Equal(t, "git-tools", list[0].Name)
}

func TestRegistry_InstallAndRemove(t *testing.T) {
	src := testdata("valid-home/git-tools")
	destRoot := t.TempDir()

	r := NewRegistry(nil)
	got, _, err := r.Install(src, destRoot)
	require.NoError(t, err)
	assert.Equal(t, "git-tools", got.Name)

	// Should now appear in List()
	list := r.List()
	require.Len(t, list, 1)
	assert.Equal(t, "git-tools", list[0].Name)

	// plugin.yaml must exist at destination
	_, err = os.Stat(filepath.Join(destRoot, "git-tools", "plugin.yaml"))
	require.NoError(t, err)

	// Double install should fail
	_, _, err = r.Install(src, destRoot)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")

	// Remove
	err = r.Remove("git-tools", destRoot)
	require.NoError(t, err)
	assert.Empty(t, r.List())

	// Directory should be gone
	_, err = os.Stat(filepath.Join(destRoot, "git-tools"))
	assert.True(t, os.IsNotExist(err))
}

func TestRegistry_InstallBadSrc(t *testing.T) {
	r := NewRegistry(nil)
	_, _, err := r.Install("/nonexistent-src-xyz", t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no plugin.yaml")
}

func TestRegistry_RemoveMissing(t *testing.T) {
	r := NewRegistry(nil)
	err := r.Remove("nope", t.TempDir())
	require.Error(t, err)
}

func TestCopyDir_PreservesModes(t *testing.T) {
	src := testdata("valid-home/git-tools")
	dst := t.TempDir()
	err := copyDir(src, filepath.Join(dst, "git-tools"))
	require.NoError(t, err)

	// audit.sh should be executable
	info, err := os.Stat(filepath.Join(dst, "git-tools", "hooks", "audit.sh"))
	require.NoError(t, err)
	assert.NotZero(t, info.Mode()&0o111, "audit.sh should be executable")
}

// ---------------------------------------------------------------------------
// Helpers for archive tests
// ---------------------------------------------------------------------------

// minimalPluginYAML is a valid plugin.yaml content for test archives.
const minimalPluginYAML = `name: net-plugin
version: 0.1.0
description: test plugin from archive
`

// buildTarGz creates a tar.gz in memory (returned as temp file path) that
// contains an optional top-level directory wrapping the plugin.yaml.
func buildTarGzFile(t *testing.T, nested bool, extraEntries map[string]string) string {
	t.Helper()
	tmp, err := os.CreateTemp(t.TempDir(), "test-*.tar.gz")
	require.NoError(t, err)
	defer tmp.Close()

	gw := gzip.NewWriter(tmp)
	tw := tar.NewWriter(gw)

	prefix := ""
	if nested {
		prefix = "net-plugin/"
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Typeflag: tar.TypeDir,
			Name:     prefix,
			Mode:     0o755,
		}))
	}

	writeEntry := func(name, content string) {
		require.NoError(t, tw.WriteHeader(&tar.Header{
			Typeflag: tar.TypeReg,
			Name:     name,
			Mode:     0o644,
			Size:     int64(len(content)),
		}))
		_, err := tw.Write([]byte(content))
		require.NoError(t, err)
	}

	writeEntry(prefix+"plugin.yaml", minimalPluginYAML)
	for name, content := range extraEntries {
		writeEntry(prefix+name, content)
	}

	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	return tmp.Name()
}

// buildZipFile creates a zip archive for testing.
func buildZipFile(t *testing.T, nested bool) string {
	t.Helper()
	tmp, err := os.CreateTemp(t.TempDir(), "test-*.zip")
	require.NoError(t, err)
	defer tmp.Close()

	zw := zip.NewWriter(tmp)

	prefix := ""
	if nested {
		prefix = "net-plugin/"
	}

	w, err := zw.Create(prefix + "plugin.yaml")
	require.NoError(t, err)
	_, err = w.Write([]byte(minimalPluginYAML))
	require.NoError(t, err)

	require.NoError(t, zw.Close())
	return tmp.Name()
}

// servFile starts an httptest server serving a single file at /plugin.<ext>.
func servFile(t *testing.T, filePath, ext string) string {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/plugin."+ext, func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filePath)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv.URL + "/plugin." + ext
}

// ---------------------------------------------------------------------------
// URL install tests
// ---------------------------------------------------------------------------

func TestRegistry_Install_FromTarGz(t *testing.T) {
	// Flat archive (plugin.yaml at root).
	archivePath := buildTarGzFile(t, false, nil)
	url := servFile(t, archivePath, "tar.gz")

	destRoot := t.TempDir()
	r := NewRegistry(nil)
	got, _, err := r.Install(url, destRoot)
	require.NoError(t, err)
	assert.Equal(t, "net-plugin", got.Name)

	_, err = os.Stat(filepath.Join(destRoot, "net-plugin", "plugin.yaml"))
	require.NoError(t, err)
}

func TestRegistry_Install_FromTarGzNested(t *testing.T) {
	// Archive where plugin.yaml is inside a top-level directory.
	archivePath := buildTarGzFile(t, true, nil)
	url := servFile(t, archivePath, "tar.gz")

	destRoot := t.TempDir()
	r := NewRegistry(nil)
	got, _, err := r.Install(url, destRoot)
	require.NoError(t, err)
	assert.Equal(t, "net-plugin", got.Name)
}

func TestRegistry_Install_FromZip(t *testing.T) {
	archivePath := buildZipFile(t, false)
	url := servFile(t, archivePath, "zip")

	destRoot := t.TempDir()
	r := NewRegistry(nil)
	got, _, err := r.Install(url, destRoot)
	require.NoError(t, err)
	assert.Equal(t, "net-plugin", got.Name)

	_, err = os.Stat(filepath.Join(destRoot, "net-plugin", "plugin.yaml"))
	require.NoError(t, err)
}

func TestRegistry_Install_FromZipNested(t *testing.T) {
	archivePath := buildZipFile(t, true)
	url := servFile(t, archivePath, "zip")

	destRoot := t.TempDir()
	r := NewRegistry(nil)
	got, _, err := r.Install(url, destRoot)
	require.NoError(t, err)
	assert.Equal(t, "net-plugin", got.Name)
}

func TestRegistry_Install_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)

	r := NewRegistry(nil)
	_, _, err := r.Install(srv.URL+"/nope.tar.gz", t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 404")
}

// ---------------------------------------------------------------------------
// Zip-slip protection tests
// ---------------------------------------------------------------------------

func TestRegistry_Install_RejectsZipSlipTarGz(t *testing.T) {
	// Build a tar.gz with a zip-slip entry (../escape.txt).
	tmp, err := os.CreateTemp(t.TempDir(), "slip-*.tar.gz")
	require.NoError(t, err)
	defer tmp.Close()

	gw := gzip.NewWriter(tmp)
	tw := tar.NewWriter(gw)

	slipName := "../escape.txt"
	content := "evil"
	require.NoError(t, tw.WriteHeader(&tar.Header{
		Typeflag: tar.TypeReg,
		Name:     slipName,
		Mode:     0o644,
		Size:     int64(len(content)),
	}))
	_, err = tw.Write([]byte(content))
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())

	url := servFile(t, tmp.Name(), "tar.gz")
	r := NewRegistry(nil)
	_, _, err = r.Install(url, t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "zip-slip")
}

func TestRegistry_Install_RejectsZipSlipZip(t *testing.T) {
	tmp, err := os.CreateTemp(t.TempDir(), "slip-*.zip")
	require.NoError(t, err)
	defer tmp.Close()

	zw := zip.NewWriter(tmp)
	w, err := zw.Create("../escape.txt")
	require.NoError(t, err)
	_, err = w.Write([]byte("evil"))
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	url := servFile(t, tmp.Name(), "zip")
	r := NewRegistry(nil)
	_, _, err = r.Install(url, t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "zip-slip")
}

// ---------------------------------------------------------------------------
// Git install test
// ---------------------------------------------------------------------------

func hasGit(t *testing.T) bool {
	t.Helper()
	_, err := exec.LookPath("git")
	return err == nil
}

func TestRegistry_Install_FromGit(t *testing.T) {
	if !hasGit(t) {
		t.Skip("no git on PATH")
	}

	// Create a local git repo with a minimal plugin.
	repoDir := t.TempDir()

	// Configure git identity for this repo only.
	cmds := [][]string{
		{"git", "-C", repoDir, "init"},
		{"git", "-C", repoDir, "config", "user.email", "test@example.com"},
		{"git", "-C", repoDir, "config", "user.name", "Test"},
	}
	for _, args := range cmds {
		out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
		require.NoError(t, err, "setup: %s", string(out))
	}

	// Write plugin.yaml.
	require.NoError(t, os.WriteFile(filepath.Join(repoDir, "plugin.yaml"), []byte(minimalPluginYAML), 0o644))

	// Commit.
	addOut, err := exec.Command("git", "-C", repoDir, "add", ".").CombinedOutput()
	require.NoError(t, err, string(addOut))
	commitOut, err := exec.Command("git", "-C", repoDir, "commit", "-m", "init").CombinedOutput()
	require.NoError(t, err, string(commitOut))

	// Install via git+file:// spec.
	destRoot := t.TempDir()
	r := NewRegistry(nil)
	got, _, err := r.Install("git+file://"+repoDir, destRoot)
	require.NoError(t, err)
	assert.Equal(t, "net-plugin", got.Name)

	_, err = os.Stat(filepath.Join(destRoot, "net-plugin", "plugin.yaml"))
	require.NoError(t, err)
}

func TestRegistry_Install_FromGit_NoGitOnPath(t *testing.T) {
	// Simulate no git by using a spec that requires git but PATH has no git.
	// We can't truly remove git from PATH, so skip if git is missing and
	// instead verify error message when a bad spec is used with a non-existent
	// URL to trigger a git clone error.
	if !hasGit(t) {
		t.Skip("git not on PATH — already the expected error condition")
	}

	r := NewRegistry(nil)
	// Use a local non-existent path as git URL to force clone failure.
	_, _, err := r.Install("git+https://127.0.0.1:1/nonexistent-repo.git", t.TempDir())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "git clone failed")
}

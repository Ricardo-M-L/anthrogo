package plugin

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/ricardo/anthrogo/internal/yamlsafe"
)

// Registry holds all loaded plugins, thread-safe.
type Registry struct {
	mu      sync.RWMutex
	plugins map[string]Plugin
}

// NewRegistry constructs a Registry from a pre-loaded plugin list.
func NewRegistry(list []Plugin) *Registry {
	r := &Registry{plugins: map[string]Plugin{}}
	for _, p := range list {
		r.plugins[p.Name] = p
	}
	return r
}

// Get returns a plugin by name.
func (r *Registry) Get(name string) (Plugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.plugins[name]
	return p, ok
}

// List returns all plugins sorted by name.
func (r *Registry) List() []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Plugin, 0, len(r.plugins))
	for _, p := range r.plugins {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Reload re-scans both roots and replaces the registry contents atomically.
func (r *Registry) Reload(homeRoot, cwdRoot string) ([]string, error) {
	plugins, warnings, err := LoadAll(homeRoot, cwdRoot)
	if err != nil {
		return warnings, err
	}
	r.mu.Lock()
	r.plugins = map[string]Plugin{}
	for _, p := range plugins {
		r.plugins[p.Name] = p
	}
	r.mu.Unlock()
	return warnings, nil
}

// Install installs a plugin from src into destRoot. src may be:
//   - a local directory path (default, M4.4 behaviour)
//   - an https:// or http:// URL pointing to a .tar.gz/.tgz/.zip archive
//   - a git+https:// or git+ssh:// spec (optionally with @branch suffix)
func (r *Registry) Install(src, destRoot string) (Plugin, []string, error) {
	src = strings.TrimSpace(src)
	switch {
	case strings.HasPrefix(src, "git+"):
		return r.installFromGit(src, destRoot)
	case strings.HasPrefix(src, "https://") || strings.HasPrefix(src, "http://"):
		return r.installFromURL(src, destRoot)
	default:
		return r.installFromDir(src, destRoot)
	}
}

// installFromDir copies srcDir into destRoot/<plugin-name>/ after validating
// the manifest. Returns the parsed Plugin, load-time warnings, and any error.
// Refuses if the plugin is already installed.
func (r *Registry) installFromDir(srcDir, destRoot string) (Plugin, []string, error) {
	raw, err := os.ReadFile(filepath.Join(srcDir, "plugin.yaml"))
	if err != nil {
		return Plugin{}, nil, fmt.Errorf("install: no plugin.yaml in %s: %w", srcDir, err)
	}
	var m Manifest
	var warns []string
	if err := yamlsafe.Unmarshal(raw, &m, "install plugin.yaml", &warns); err != nil {
		return Plugin{}, nil, fmt.Errorf("install: bad manifest: %w", err)
	}
	for _, w := range warns {
		fmt.Fprintf(os.Stderr, "anthrogo: %s\n", w)
	}
	if !nameRE.MatchString(m.Name) {
		return Plugin{}, nil, fmt.Errorf("install: invalid plugin name %q", m.Name)
	}
	dest := filepath.Join(destRoot, m.Name)
	if _, err := os.Stat(dest); err == nil {
		return Plugin{}, nil, fmt.Errorf("install: plugin %q already exists at %s", m.Name, dest)
	}
	if err := os.MkdirAll(destRoot, 0o755); err != nil {
		return Plugin{}, nil, err
	}
	if err := copyDir(srcDir, dest); err != nil {
		return Plugin{}, nil, err
	}
	// Re-load to get a fully-resolved Plugin.
	plugins, warnings, err := LoadAll(destRoot, "")
	if err != nil {
		return Plugin{}, warnings, fmt.Errorf("install: post-copy reload failed: %w", err)
	}
	var got Plugin
	for _, p := range plugins {
		if p.Name == m.Name {
			got = p
			break
		}
	}
	// Filter warnings to only those mentioning this plugin.
	var relevant []string
	for _, w := range warnings {
		if strings.Contains(w, m.Name) {
			relevant = append(relevant, w)
		}
	}
	r.mu.Lock()
	r.plugins[got.Name] = got
	r.mu.Unlock()
	return got, relevant, nil
}

// installFromURL downloads an archive from rawURL, extracts it to a temp dir,
// locates the plugin.yaml root, and delegates to installFromDir.
func (r *Registry) installFromURL(rawURL, destRoot string) (Plugin, []string, error) {
	resp, err := http.Get(rawURL) //nolint:gosec // user-supplied URL is intentional
	if err != nil {
		return Plugin{}, nil, fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return Plugin{}, nil, fmt.Errorf("download: HTTP %d", resp.StatusCode)
	}

	tmpFile, err := os.CreateTemp("", "anthrogo-plugin-*.archive")
	if err != nil {
		return Plugin{}, nil, err
	}
	defer os.Remove(tmpFile.Name())

	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		tmpFile.Close()
		return Plugin{}, nil, err
	}
	tmpFile.Close()

	extractDir, err := os.MkdirTemp("", "anthrogo-plugin-extract-*")
	if err != nil {
		return Plugin{}, nil, err
	}
	defer os.RemoveAll(extractDir)

	if strings.HasSuffix(rawURL, ".zip") {
		if err := extractZip(tmpFile.Name(), extractDir); err != nil {
			return Plugin{}, nil, fmt.Errorf("unzip: %w", err)
		}
	} else {
		// .tar.gz / .tgz / unknown — try tar.gz
		if err := extractTarGz(tmpFile.Name(), extractDir); err != nil {
			return Plugin{}, nil, fmt.Errorf("untar: %w", err)
		}
	}

	pluginRoot, err := findPluginRoot(extractDir)
	if err != nil {
		return Plugin{}, nil, err
	}
	return r.installFromDir(pluginRoot, destRoot)
}

// installFromGit clones a git repository (depth=1) and delegates to installFromDir.
// spec must start with "git+https://" or "git+ssh://". An optional "@branch" suffix
// selects the branch/tag to clone.
func (r *Registry) installFromGit(spec, destRoot string) (Plugin, []string, error) {
	// Strip the "git+" prefix to get the real URL.
	cloneURL := strings.TrimPrefix(spec, "git+")
	ref := ""

	// Split on the last "@", but only if it appears after the scheme (i.e., not
	// part of a user-info like git@github.com which uses the ssh:// form and
	// won't have "git+" prefix anyway).
	if at := strings.LastIndex(cloneURL, "@"); at > 0 {
		// Make sure the "@" is not inside the scheme (e.g., https://)
		schemeEnd := strings.Index(cloneURL, "://")
		if schemeEnd >= 0 && at > schemeEnd+3 {
			ref = cloneURL[at+1:]
			cloneURL = cloneURL[:at]
		}
	}

	if _, err := exec.LookPath("git"); err != nil {
		return Plugin{}, nil, fmt.Errorf("install: git not on PATH: %w", err)
	}

	tmp, err := os.MkdirTemp("", "anthrogo-plugin-git-*")
	if err != nil {
		return Plugin{}, nil, err
	}
	defer os.RemoveAll(tmp)

	args := []string{"clone", "--depth=1"}
	if ref != "" {
		args = append(args, "--branch", ref)
	}
	args = append(args, cloneURL, tmp)

	cmd := exec.Command("git", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return Plugin{}, nil, fmt.Errorf("git clone failed: %s", string(out))
	}

	pluginRoot, err := findPluginRoot(tmp)
	if err != nil {
		return Plugin{}, nil, err
	}
	return r.installFromDir(pluginRoot, destRoot)
}

// extractTarGz extracts a gzipped tar archive to destDir with zip-slip protection.
func extractTarGz(archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(destDir, filepath.Clean(hdr.Name))
		// Zip-slip protection.
		if !strings.HasPrefix(target, destDir+string(os.PathSeparator)) && target != destDir {
			return fmt.Errorf("zip-slip rejected: %s", hdr.Name)
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, safeArchiveMode(os.FileMode(hdr.Mode)))
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		}
	}
	return nil
}

// extractZip extracts a ZIP archive to destDir with zip-slip protection.
func extractZip(archivePath, destDir string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, file := range r.File {
		target := filepath.Join(destDir, filepath.Clean(file.Name))
		// Zip-slip protection.
		if !strings.HasPrefix(target, destDir+string(os.PathSeparator)) && target != destDir {
			return fmt.Errorf("zip-slip rejected: %s", file.Name)
		}

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, safeArchiveMode(file.Mode())); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, safeArchiveMode(file.Mode()))
		if err != nil {
			return err
		}
		rc, err := file.Open()
		if err != nil {
			out.Close()
			return err
		}
		_, err = io.Copy(out, rc)
		rc.Close()
		out.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// findPluginRoot returns the directory containing plugin.yaml. It checks
// extractDir first, then one level deep (tarballs commonly wrap contents in a
// single top-level directory).
func findPluginRoot(extractDir string) (string, error) {
	if _, err := os.Stat(filepath.Join(extractDir, "plugin.yaml")); err == nil {
		return extractDir, nil
	}
	entries, err := os.ReadDir(extractDir)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		candidate := filepath.Join(extractDir, e.Name())
		if _, err := os.Stat(filepath.Join(candidate, "plugin.yaml")); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("no plugin.yaml found in archive")
}

// Remove deletes the home-rooted plugin <name> and unregisters it.
func (r *Registry) Remove(name, homeRoot string) error {
	target := filepath.Join(homeRoot, name)
	info, err := os.Stat(target)
	if err != nil {
		return fmt.Errorf("remove: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("remove: %s is not a directory", target)
	}
	if err := os.RemoveAll(target); err != nil {
		return err
	}
	r.mu.Lock()
	delete(r.plugins, name)
	r.mu.Unlock()
	return nil
}

// copyDir copies src to dst recursively, preserving file mode bits and symlinks.
func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("plugin contains symlink: %s — refusing to install for security", rel)
		}
		if info.IsDir() {
			return os.MkdirAll(target, safeArchiveMode(info.Mode()))
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, safeArchiveMode(info.Mode()))
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	})
}

// safeArchiveMode strips setuid/setgid/sticky bits and world-write from a
// file mode parsed from an untrusted archive header. Mirrors the skill
// helper of the same name.
func safeArchiveMode(m os.FileMode) os.FileMode {
	return m & 0o755
}

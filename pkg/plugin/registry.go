package plugin

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
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

// Install copies srcDir into destRoot/<plugin-name>/ after validating the
// manifest. Returns the parsed Plugin, load-time warnings, and any error.
// Refuses if the plugin is already installed.
func (r *Registry) Install(srcDir, destRoot string) (Plugin, []string, error) {
	raw, err := os.ReadFile(filepath.Join(srcDir, "plugin.yaml"))
	if err != nil {
		return Plugin{}, nil, fmt.Errorf("install: no plugin.yaml in %s: %w", srcDir, err)
	}
	var m Manifest
	if err := yaml.Unmarshal(raw, &m); err != nil {
		return Plugin{}, nil, fmt.Errorf("install: bad manifest: %w", err)
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
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		}
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	})
}

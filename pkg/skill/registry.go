package skill

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

// maxSkillArchiveBytes is the maximum size of a skill archive downloaded via URL.
const maxSkillArchiveBytes = 50 * 1024 * 1024 // 50 MB

type Registry struct {
	mu     sync.RWMutex
	skills map[string]Skill
}

func NewRegistry(list []Skill) *Registry {
	r := &Registry{skills: map[string]Skill{}}
	for _, s := range list {
		r.skills[s.Name] = s
	}
	return r
}

func (r *Registry) Get(name string) (Skill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.skills[name]
	return s, ok
}

func (r *Registry) List() []Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Skill, 0, len(r.skills))
	for _, s := range r.skills {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Add inserts s into the registry if no skill with that name exists.
// Returns true if added, false if a same-named skill was already present
// (in which case s is discarded).
func (r *Registry) Add(s Skill) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.skills[s.Name]; exists {
		return false
	}
	r.skills[s.Name] = s
	return true
}

func (r *Registry) Reload(homeRoot, cwdRoot string) ([]string, error) {
	skills, warnings, err := LoadAll(homeRoot, cwdRoot)
	if err != nil {
		return warnings, err
	}
	r.mu.Lock()
	r.skills = map[string]Skill{}
	for _, s := range skills {
		r.skills[s.Name] = s
	}
	r.mu.Unlock()
	return warnings, nil
}

// Install fetches a skill from a local dir, https URL, or git+ spec
// and copies it into destRoot/<skill-name>/.
func (r *Registry) Install(src, destRoot string) (Skill, []string, error) {
	src = strings.TrimSpace(src)
	switch {
	case strings.HasPrefix(src, "git+"):
		return r.installFromGit(src, destRoot)
	case strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://"):
		return r.installFromURL(src, destRoot)
	default:
		return r.installFromDir(src, destRoot)
	}
}

// installFromDir copies srcDir into destRoot/<skill-name>/ after reading SKILL.md frontmatter.
func (r *Registry) installFromDir(srcDir, destRoot string) (Skill, []string, error) {
	skillMD := filepath.Join(srcDir, "SKILL.md")
	raw, err := os.ReadFile(skillMD)
	if err != nil {
		return Skill{}, nil, fmt.Errorf("install: no SKILL.md in %s", srcDir)
	}
	fm, _, ok := SplitFrontmatter(raw)
	if !ok {
		return Skill{}, nil, fmt.Errorf("install: missing or malformed frontmatter")
	}
	var meta struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}
	var warns []string
	if err := yamlsafe.Unmarshal(fm, &meta, "install SKILL.md", &warns); err != nil {
		return Skill{}, nil, fmt.Errorf("install: bad YAML: %w", err)
	}
	for _, w := range warns {
		fmt.Fprintf(os.Stderr, "anthrogo: %s\n", w)
	}
	if meta.Name == "" {
		return Skill{}, nil, fmt.Errorf("install: empty skill name")
	}
	dest := filepath.Join(destRoot, meta.Name)
	if _, err := os.Stat(dest); err == nil {
		return Skill{}, nil, fmt.Errorf("install: skill %q already exists at %s", meta.Name, dest)
	}
	if err := os.MkdirAll(destRoot, 0o755); err != nil {
		return Skill{}, nil, err
	}
	if err := copySkillDir(srcDir, dest); err != nil {
		return Skill{}, nil, err
	}
	skills, warnings, _ := LoadAll(destRoot, "")
	var got Skill
	for _, s := range skills {
		if s.Name == meta.Name {
			got = s
			break
		}
	}
	r.mu.Lock()
	r.skills[got.Name] = got
	r.mu.Unlock()
	return got, warnings, nil
}

// installFromURL downloads an archive, extracts it, and delegates to installFromDir.
func (r *Registry) installFromURL(rawURL, destRoot string) (Skill, []string, error) {
	resp, err := http.Get(rawURL) //nolint:gosec // user-supplied URL is intentional
	if err != nil {
		return Skill{}, nil, fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return Skill{}, nil, fmt.Errorf("download: HTTP %d", resp.StatusCode)
	}

	tmpFile, err := os.CreateTemp("", "anthrogo-skill-*.archive")
	if err != nil {
		return Skill{}, nil, err
	}
	defer os.Remove(tmpFile.Name())
	written, err := io.Copy(tmpFile, io.LimitReader(resp.Body, maxSkillArchiveBytes))
	if err != nil {
		tmpFile.Close()
		return Skill{}, nil, err
	}
	if written >= maxSkillArchiveBytes {
		tmpFile.Close()
		return Skill{}, nil, fmt.Errorf("download: archive exceeds 50MB limit")
	}
	tmpFile.Close()

	extractDir, err := os.MkdirTemp("", "anthrogo-skill-extract-*")
	if err != nil {
		return Skill{}, nil, err
	}
	defer os.RemoveAll(extractDir)

	if strings.HasSuffix(rawURL, ".zip") {
		if err := extractSkillZip(tmpFile.Name(), extractDir); err != nil {
			return Skill{}, nil, fmt.Errorf("unzip: %w", err)
		}
	} else {
		if err := extractSkillTarGz(tmpFile.Name(), extractDir); err != nil {
			return Skill{}, nil, fmt.Errorf("untar: %w", err)
		}
	}

	skillDir, err := findSkillRoot(extractDir)
	if err != nil {
		return Skill{}, nil, err
	}
	return r.installFromDir(skillDir, destRoot)
}

// installFromGit clones a git repository (depth=1) and delegates to installFromDir.
func (r *Registry) installFromGit(spec, destRoot string) (Skill, []string, error) {
	cloneURL := strings.TrimPrefix(spec, "git+")
	ref := ""
	if at := strings.LastIndex(cloneURL, "@"); at > 0 {
		schemeEnd := strings.Index(cloneURL, "://")
		if schemeEnd >= 0 && at > schemeEnd+3 {
			ref = cloneURL[at+1:]
			cloneURL = cloneURL[:at]
		}
	}
	if _, err := exec.LookPath("git"); err != nil {
		return Skill{}, nil, fmt.Errorf("install: git not on PATH: %w", err)
	}
	tmp, err := os.MkdirTemp("", "anthrogo-skill-git-*")
	if err != nil {
		return Skill{}, nil, err
	}
	defer os.RemoveAll(tmp)
	args := []string{"clone", "--depth=1"}
	if ref != "" {
		args = append(args, "--branch", ref)
	}
	args = append(args, cloneURL, tmp)
	cmd := exec.Command("git", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return Skill{}, nil, fmt.Errorf("git clone failed: %s", string(out))
	}
	skillDir, err := findSkillRoot(tmp)
	if err != nil {
		return Skill{}, nil, err
	}
	return r.installFromDir(skillDir, destRoot)
}

// extractSkillTarGz extracts a gzipped tar archive to destDir with zip-slip protection.
func extractSkillTarGz(archivePath, destDir string) error {
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

// extractSkillZip extracts a ZIP archive to destDir with zip-slip protection.
func extractSkillZip(archivePath, destDir string) error {
	rz, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer rz.Close()
	for _, file := range rz.File {
		target := filepath.Join(destDir, filepath.Clean(file.Name))
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

// findSkillRoot returns the directory containing SKILL.md (root or 1-level nested).
func findSkillRoot(extractDir string) (string, error) {
	if _, err := os.Stat(filepath.Join(extractDir, "SKILL.md")); err == nil {
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
		if _, err := os.Stat(filepath.Join(candidate, "SKILL.md")); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("no SKILL.md found in archive")
}

// copySkillDir copies src to dst recursively, preserving file mode bits.
// Symlinks are refused for security: a skill archive containing a symlink
// pointing outside the skill directory could exfiltrate host files.
func copySkillDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("skill contains symlink: %s — refusing to install for security", rel)
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

// safeArchiveMode strips setuid/setgid/sticky bits and clears world-write
// from a file mode parsed from an untrusted archive header. Preserves
// owner/group rwx and other-rx (the common Unix 0755-ish file mode space).
func safeArchiveMode(m os.FileMode) os.FileMode {
	return m & 0o755
}

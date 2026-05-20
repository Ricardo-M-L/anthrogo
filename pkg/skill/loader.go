package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

const maxBodyBytes = 1 << 20

var nameRE = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)

type frontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// LoadAll scans homeRoot and cwdRoot for skill directories. cwd-level skills
// replace home-level skills with the same name. Returns the merged skill list,
// per-skill warnings, and only returns a top-level error for unrecoverable IO.
func LoadAll(homeRoot, cwdRoot string) ([]Skill, []string, error) {
	var warnings []string
	home, w1 := loadDir(homeRoot, "home")
	warnings = append(warnings, w1...)
	cwd, w2 := loadDir(cwdRoot, "cwd")
	warnings = append(warnings, w2...)

	merged := map[string]Skill{}
	for _, s := range home {
		merged[s.Name] = s
	}
	for _, s := range cwd {
		if _, exists := merged[s.Name]; exists {
			warnings = append(warnings, fmt.Sprintf("skill %q in cwd overrides home version", s.Name))
		}
		merged[s.Name] = s
	}
	out := make([]Skill, 0, len(merged))
	for _, s := range merged {
		out = append(out, s)
	}
	return out, warnings, nil
}

func loadDir(root, source string) ([]Skill, []string) {
	if root == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, nil // missing root is OK
	}
	var skills []Skill
	var warnings []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if !nameRE.MatchString(name) {
			warnings = append(warnings, fmt.Sprintf("skill dir %q has invalid name", name))
			continue
		}
		base := filepath.Join(root, name)
		path := filepath.Join(base, "SKILL.md")
		raw, err := os.ReadFile(path)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("skill %q: no SKILL.md", name))
			continue
		}
		fm, body, ok := splitFrontmatter(raw)
		if !ok {
			warnings = append(warnings, fmt.Sprintf("skill %q: missing or malformed frontmatter", name))
			continue
		}
		var meta frontmatter
		if err := yaml.Unmarshal(fm, &meta); err != nil {
			warnings = append(warnings, fmt.Sprintf("skill %q: bad YAML: %v", name, err))
			continue
		}
		if meta.Name == "" || meta.Description == "" {
			warnings = append(warnings, fmt.Sprintf("skill %q: empty name or description", name))
			continue
		}
		if meta.Name != name {
			warnings = append(warnings, fmt.Sprintf("skill %q: frontmatter name %q doesn't match directory", name, meta.Name))
			continue
		}
		if len(body) > maxBodyBytes {
			warnings = append(warnings, fmt.Sprintf("skill %q: body truncated to %d bytes", name, maxBodyBytes))
			body = body[:maxBodyBytes]
		}
		abs, _ := filepath.Abs(base)
		skills = append(skills, Skill{
			Name:        name,
			Description: meta.Description,
			BasePath:    abs,
			Body:        string(body),
			Source:      source,
		})
	}
	return skills, warnings
}

// splitFrontmatter separates the leading "---\n...\n---\n" block.
func splitFrontmatter(raw []byte) (frontmatterBytes, body []byte, ok bool) {
	s := string(raw)
	if !strings.HasPrefix(s, "---\n") && !strings.HasPrefix(s, "---\r\n") {
		return nil, nil, false
	}
	start := strings.Index(s, "\n") + 1
	end := strings.Index(s[start:], "\n---")
	if end < 0 {
		return nil, nil, false
	}
	fmEnd := start + end
	bodyStart := fmEnd + len("\n---")
	if bodyStart < len(s) && (s[bodyStart] == '\n' || s[bodyStart] == '\r') {
		bodyStart++
	}
	if bodyStart < len(s) && s[bodyStart] == '\n' {
		bodyStart++
	}
	return []byte(s[start:fmEnd]), []byte(s[bodyStart:]), true
}

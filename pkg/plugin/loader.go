package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ricardo/anthrogo/internal/hooks"
	"github.com/ricardo/anthrogo/internal/mcp"
	"github.com/ricardo/anthrogo/internal/yamlsafe"
	"github.com/ricardo/anthrogo/pkg/skill"
)

// maxPluginYAMLBytes caps the size of a plugin manifest we'll load.
const maxPluginYAMLBytes = 1 << 20 // 1 MiB

var nameRE = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)

// LoadAll scans homeRoot and cwdRoot for plugin directories. cwd-level plugins
// override home-level plugins with the same name. Returns the merged plugin list,
// per-plugin warnings, and only returns a top-level error for unrecoverable IO.
func LoadAll(homeRoot, cwdRoot string) ([]Plugin, []string, error) {
	var warnings []string
	home, w1 := loadDir(homeRoot, "home")
	warnings = append(warnings, w1...)
	cwd, w2 := loadDir(cwdRoot, "cwd")
	warnings = append(warnings, w2...)

	merged := map[string]Plugin{}
	for _, p := range home {
		merged[p.Name] = p
	}
	for _, p := range cwd {
		if _, exists := merged[p.Name]; exists {
			warnings = append(warnings, fmt.Sprintf("plugin %q in cwd overrides home version", p.Name))
		}
		merged[p.Name] = p
	}
	out := make([]Plugin, 0, len(merged))
	for _, p := range merged {
		out = append(out, p)
	}
	return out, warnings, nil
}

func loadDir(root, source string) ([]Plugin, []string) {
	if root == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, nil // missing root is OK
	}
	var plugins []Plugin
	var warnings []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dirname := e.Name()
		base := filepath.Join(root, dirname)
		absBase, _ := filepath.Abs(base)
		manifestPath := filepath.Join(base, "plugin.yaml")
		if st, statErr := os.Stat(manifestPath); statErr == nil && st.Size() > maxPluginYAMLBytes {
			warnings = append(warnings, fmt.Sprintf("plugin %q: plugin.yaml is %d bytes (cap %d) — skipped", dirname, st.Size(), maxPluginYAMLBytes))
			continue
		}
		raw, err := os.ReadFile(manifestPath)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("plugin dir %q: no plugin.yaml", dirname))
			continue
		}
		var m Manifest
		if err := yamlsafe.Unmarshal(raw, &m, "plugin "+dirname+"/plugin.yaml", &warnings); err != nil {
			warnings = append(warnings, fmt.Sprintf("plugin %q: bad YAML: %v", dirname, err))
			continue
		}
		if !nameRE.MatchString(m.Name) {
			warnings = append(warnings, fmt.Sprintf("plugin %q: invalid manifest name %q", dirname, m.Name))
			continue
		}
		if m.Name != dirname {
			warnings = append(warnings, fmt.Sprintf("plugin %q: manifest name %q doesn't match directory", dirname, m.Name))
			continue
		}

		p := Plugin{Name: m.Name, Manifest: m, BasePath: absBase, Source: source}

		// Resolve commands
		for _, cs := range m.Commands {
			if cs.Name == "" || !strings.HasPrefix(cs.Name, "/") {
				warnings = append(warnings, fmt.Sprintf("plugin %q: skipping command with bad name %q", m.Name, cs.Name))
				continue
			}
			if cs.Body == "" {
				warnings = append(warnings, fmt.Sprintf("plugin %q: skipping command %q (empty body)", m.Name, cs.Name))
				continue
			}
			p.Commands = append(p.Commands, DynamicCommand{spec: cs})
		}

		// Resolve skills
		for _, sr := range m.Skills {
			skillDir := filepath.Join(absBase, sr.Dir)
			sk, ok, warn := loadOneSkill(skillDir, source)
			if warn != "" {
				warnings = append(warnings, fmt.Sprintf("plugin %q skill %q: %s", m.Name, sr.Dir, warn))
			}
			if ok {
				p.Skills = append(p.Skills, sk)
			}
		}

		// Absolutize hook command paths relative to plugin root
		p.Hooks = absolutizeHooks(m.Hooks, absBase)

		// Namespace MCP server keys with "<plugin>:" prefix
		if len(m.MCPServers) > 0 {
			p.MCPServers = make(map[string]mcp.MCPServerConfig, len(m.MCPServers))
			for k, v := range m.MCPServers {
				p.MCPServers[m.Name+":"+k] = v
			}
		}

		plugins = append(plugins, p)
	}
	return plugins, warnings
}

// loadOneSkill loads a single skill directory by reading its SKILL.md directly.
// Returns false + warning string if SKILL.md is missing or malformed.
func loadOneSkill(skillDir, source string) (skill.Skill, bool, string) {
	skillMD := filepath.Join(skillDir, "SKILL.md")
	if st, err := os.Stat(skillMD); err == nil && st.Size() > maxPluginYAMLBytes {
		return skill.Skill{}, false, fmt.Sprintf("SKILL.md at %s is %d bytes — refusing to load (cap %d)", skillMD, st.Size(), maxPluginYAMLBytes)
	}
	raw, err := os.ReadFile(skillMD)
	if err != nil {
		return skill.Skill{}, false, "no SKILL.md at " + skillMD
	}
	fm, body, ok := skill.SplitFrontmatter(raw)
	if !ok {
		return skill.Skill{}, false, "missing or unterminated frontmatter"
	}
	var meta struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}
	if err := yamlsafe.Unmarshal(fm, &meta, "plugin SKILL.md ("+source+")", nil); err != nil {
		return skill.Skill{}, false, "bad YAML: " + err.Error()
	}
	if meta.Name == "" || meta.Description == "" {
		return skill.Skill{}, false, "empty name or description"
	}
	base, _ := filepath.Abs(skillDir)
	return skill.Skill{Name: meta.Name, Description: meta.Description, BasePath: base, Body: string(body), Source: source}, true, ""
}

func absolutizeHooks(c hooks.Config, base string) hooks.Config {
	fix := func(list []hooks.Spec) []hooks.Spec {
		out := make([]hooks.Spec, 0, len(list))
		for _, sp := range list {
			if sp.Command != "" && !filepath.IsAbs(sp.Command) {
				sp.Command = filepath.Join(base, sp.Command)
			}
			out = append(out, sp)
		}
		return out
	}
	c.PreToolUse = fix(c.PreToolUse)
	c.PostToolUse = fix(c.PostToolUse)
	c.UserPromptSubmit = fix(c.UserPromptSubmit)
	c.Stop = fix(c.Stop)
	c.SubagentStop = fix(c.SubagentStop)
	c.Notification = fix(c.Notification)
	c.PreCompact = fix(c.PreCompact)
	c.SessionStart = fix(c.SessionStart)
	c.SessionEnd = fix(c.SessionEnd)
	return c
}

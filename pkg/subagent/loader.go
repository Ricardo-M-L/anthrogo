package subagent

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// maxSubagentYAMLBytes caps the size of a subagent spec file we'll load.
const maxSubagentYAMLBytes = 256 << 10 // 256 KiB

var nameRE = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)

type yamlSpec struct {
	Name               string      `yaml:"name"`
	Description        string      `yaml:"description"`
	SystemPromptSuffix string      `yaml:"system_prompt_suffix"`
	ToolAllowlist      []string    `yaml:"tool_allowlist"`
	Remote             *RemoteSpec `yaml:"remote"`
	Model              string      `yaml:"model,omitempty"` // optional model override
}

// LoadAll scans homeRoot and cwdRoot for *.yaml / *.yml files (non-recursive).
// cwd specs override home specs on same name (with a warning).
// Refuses to register "general-purpose" (reserved built-in name).
// Returns the merged slice, any non-fatal warnings, and any fatal error.
func LoadAll(homeRoot, cwdRoot string) ([]Spec, []string, error) {
	var warnings []string
	home, w1 := loadDir(homeRoot)
	warnings = append(warnings, w1...)
	cwd, w2 := loadDir(cwdRoot)
	warnings = append(warnings, w2...)

	merged := map[string]Spec{}
	for _, s := range home {
		merged[s.Name] = s
	}
	for _, s := range cwd {
		if _, exists := merged[s.Name]; exists {
			warnings = append(warnings, fmt.Sprintf("subagent type %q in cwd overrides home", s.Name))
		}
		merged[s.Name] = s
	}

	out := make([]Spec, 0, len(merged))
	for _, s := range merged {
		out = append(out, s)
	}
	return out, warnings, nil
}

func loadDir(root string) ([]Spec, []string) {
	if root == "" {
		return nil, nil
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, nil
	}
	var specs []Spec
	var warnings []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		stem := strings.TrimSuffix(strings.TrimSuffix(name, ".yaml"), ".yml")
		if !nameRE.MatchString(stem) {
			warnings = append(warnings, fmt.Sprintf("subagent file %q has invalid name", name))
			continue
		}
		if stem == "general-purpose" {
			warnings = append(warnings, `subagent name "general-purpose" is reserved; skipping`)
			continue
		}
		full := filepath.Join(root, name)
		if st, statErr := os.Stat(full); statErr == nil && st.Size() > maxSubagentYAMLBytes {
			warnings = append(warnings, fmt.Sprintf("subagent %q: yaml is %d bytes (cap %d) — skipped", stem, st.Size(), maxSubagentYAMLBytes))
			continue
		}
		raw, err := os.ReadFile(full)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("subagent %q: read: %v", stem, err))
			continue
		}
		var y yamlSpec
		if err := yaml.Unmarshal(raw, &y); err != nil {
			warnings = append(warnings, fmt.Sprintf("subagent %q: bad YAML: %v", stem, err))
			continue
		}
		if y.Name != "" && y.Name != stem {
			warnings = append(warnings, fmt.Sprintf("subagent %q: yaml name %q != filename", stem, y.Name))
			continue
		}
		if y.Description == "" {
			warnings = append(warnings, fmt.Sprintf("subagent %q: empty description", stem))
			continue
		}
		// Resolve env:VARNAME in auth_token.
		if y.Remote != nil && strings.HasPrefix(y.Remote.AuthToken, "env:") {
			y.Remote.AuthToken = os.Getenv(strings.TrimPrefix(y.Remote.AuthToken, "env:"))
		}
		specs = append(specs, Spec{
			Name:               stem,
			Description:        y.Description,
			SystemPromptSuffix: y.SystemPromptSuffix,
			ToolAllowlist:      y.ToolAllowlist,
			Remote:             y.Remote,
			Model:              y.Model,
		})
	}
	return specs, warnings
}

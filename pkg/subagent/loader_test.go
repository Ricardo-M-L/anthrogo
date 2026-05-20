package subagent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadAll_ValidHome(t *testing.T) {
	homeRoot := filepath.Join("testdata", "valid-home")
	specs, warnings, err := LoadAll(homeRoot, "")
	require.NoError(t, err)

	names := make(map[string]bool)
	for _, s := range specs {
		names[s.Name] = true
	}
	require.True(t, names["code-reviewer"], "code-reviewer should be loaded")
	require.False(t, names["general-purpose"], "general-purpose must be rejected as reserved")

	foundReserved := false
	for _, w := range warnings {
		if w == `subagent name "general-purpose" is reserved; skipping` {
			foundReserved = true
		}
	}
	require.True(t, foundReserved, "expected warning about reserved name")

	var codeReviewer Spec
	for _, s := range specs {
		if s.Name == "code-reviewer" {
			codeReviewer = s
		}
	}
	require.Equal(t, "code-reviewer", codeReviewer.Name)
	require.NotEmpty(t, codeReviewer.Description)
	require.Equal(t, []string{"Read", "Grep", "Glob"}, codeReviewer.ToolAllowlist)
}

func TestLoadAll_CwdOverridesHome(t *testing.T) {
	homeRoot := filepath.Join("testdata", "valid-home")
	cwdRoot := filepath.Join("testdata", "valid-cwd-overrides")
	specs, warnings, err := LoadAll(homeRoot, cwdRoot)
	require.NoError(t, err)

	var codeReviewer Spec
	for _, s := range specs {
		if s.Name == "code-reviewer" {
			codeReviewer = s
		}
	}
	require.Equal(t, "CWD override of code-reviewer with stricter rules.", codeReviewer.Description)

	foundOverride := false
	for _, w := range warnings {
		if w == `subagent type "code-reviewer" in cwd overrides home` {
			foundOverride = true
		}
	}
	require.True(t, foundOverride, "expected cwd-overrides-home warning")
}

func TestLoadAll_ReservedNameRejected(t *testing.T) {
	homeRoot := filepath.Join("testdata", "valid-home")
	specs, warnings, err := LoadAll(homeRoot, "")
	require.NoError(t, err)

	for _, s := range specs {
		require.NotEqual(t, "general-purpose", s.Name, "reserved name must not appear in output")
	}

	foundWarn := false
	for _, w := range warnings {
		if w == `subagent name "general-purpose" is reserved; skipping` {
			foundWarn = true
		}
	}
	require.True(t, foundWarn)
}

func TestLoadAll_BadYAML(t *testing.T) {
	specs, warnings, err := LoadAll(filepath.Join("testdata", "bad-yaml"), "")
	require.NoError(t, err)
	require.Empty(t, specs)
	require.NotEmpty(t, warnings, "expected at least one warning for bad yaml")
}

func TestLoadAll_NameMismatch(t *testing.T) {
	specs, warnings, err := LoadAll(filepath.Join("testdata", "bad-name-mismatch"), "")
	require.NoError(t, err)
	require.Empty(t, specs, "name-mismatch file should produce no specs")
	require.NotEmpty(t, warnings)
}

func TestLoadAll_EmptyDescription(t *testing.T) {
	tmp := t.TempDir()
	content := "name: my-agent\ndescription: \"\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "my-agent.yaml"), []byte(content), 0o644))

	specs, warnings, err := LoadAll(tmp, "")
	require.NoError(t, err)
	require.Empty(t, specs)
	require.NotEmpty(t, warnings)
}

func TestLoadAll_EmptyRoots(t *testing.T) {
	specs, warnings, err := LoadAll("", "")
	require.NoError(t, err)
	require.Empty(t, specs)
	require.Empty(t, warnings)
}

func TestLoadAll_NonexistentRoot(t *testing.T) {
	specs, warnings, err := LoadAll("/no/such/path/xyz-nonexistent", "")
	require.NoError(t, err)
	require.Empty(t, specs)
	require.Empty(t, warnings)
}

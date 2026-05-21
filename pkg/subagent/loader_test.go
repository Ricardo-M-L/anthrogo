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

func TestLoadAll_ParsesRemoteSpec(t *testing.T) {
	tmp := t.TempDir()
	content := `name: remote-worker
description: Routes to a KAIROS worker.
remote:
  endpoint: http://worker.example.com:9001
  auth_token: secret123
`
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "remote-worker.yaml"), []byte(content), 0o644))

	specs, warnings, err := LoadAll(tmp, "")
	require.NoError(t, err)
	require.Empty(t, warnings)
	require.Len(t, specs, 1)
	s := specs[0]
	require.Equal(t, "remote-worker", s.Name)
	require.NotNil(t, s.Remote, "Remote field must be non-nil")
	require.Equal(t, "http://worker.example.com:9001", s.Remote.Endpoint)
	require.Equal(t, "secret123", s.Remote.AuthToken)
}

func TestLoadAll_ResolvesEnvAuthToken(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TEST_KAIROS_TOKEN", "resolved-from-env")
	content := `name: env-worker
description: Uses env var for auth token.
remote:
  endpoint: http://worker.example.com:9001
  auth_token: env:TEST_KAIROS_TOKEN
`
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "env-worker.yaml"), []byte(content), 0o644))

	specs, warnings, err := LoadAll(tmp, "")
	require.NoError(t, err)
	require.Empty(t, warnings)
	require.Len(t, specs, 1)
	require.NotNil(t, specs[0].Remote)
	require.Equal(t, "resolved-from-env", specs[0].Remote.AuthToken)
}

func TestLoadAll_ParsesModelOverride(t *testing.T) {
	tmp := t.TempDir()
	content := `name: fast-summarizer
description: Summarizes content using a lighter model.
model: claude-haiku-4-5-20251001
`
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "fast-summarizer.yaml"), []byte(content), 0o644))

	specs, warnings, err := LoadAll(tmp, "")
	require.NoError(t, err)
	require.Empty(t, warnings)
	require.Len(t, specs, 1)
	s := specs[0]
	require.Equal(t, "fast-summarizer", s.Name)
	require.Equal(t, "claude-haiku-4-5-20251001", s.Model, "Model field must be parsed from YAML")
}

func TestLoadAll_ModelDefaultsToEmpty(t *testing.T) {
	tmp := t.TempDir()
	content := `name: no-model-agent
description: An agent without an explicit model field.
`
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "no-model-agent.yaml"), []byte(content), 0o644))

	specs, warnings, err := LoadAll(tmp, "")
	require.NoError(t, err)
	require.Empty(t, warnings)
	require.Len(t, specs, 1)
	require.Equal(t, "", specs[0].Model, "Model must be empty when not specified in YAML")
}

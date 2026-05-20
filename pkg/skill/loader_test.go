package skill

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func testdata(sub string) string {
	return filepath.Join("testdata", sub)
}

func TestLoadAll_ValidHomeOnly(t *testing.T) {
	skills, warnings, err := LoadAll(testdata("valid-home"), "")
	require.NoError(t, err)
	require.Len(t, skills, 1)
	require.Empty(t, warnings)
	require.Equal(t, "git-flow", skills[0].Name)
	require.Equal(t, "Use when starting a new feature branch off main.", skills[0].Description)
	require.Equal(t, "home", skills[0].Source)
	require.Contains(t, skills[0].Body, "Body of the skill")
}

func TestLoadAll_CwdOverridesHome(t *testing.T) {
	skills, warnings, err := LoadAll(testdata("valid-home"), testdata("valid-cwd-overrides"))
	require.NoError(t, err)
	require.Len(t, skills, 1)
	// Must have at least one warning about override
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "overrides") {
			found = true
		}
	}
	require.True(t, found, "expected override warning, got: %v", warnings)
	require.Equal(t, "cwd", skills[0].Source)
	require.Contains(t, skills[0].Body, "cwd override version")
}

func TestLoadAll_BadFrontmatterSkippedWithWarning(t *testing.T) {
	skills, warnings, err := LoadAll(testdata("bad-no-frontmatter"), "")
	require.NoError(t, err)
	require.Empty(t, skills)
	require.NotEmpty(t, warnings)
	require.Contains(t, warnings[0], "frontmatter")
}

func TestLoadAll_NameMismatchSkipped(t *testing.T) {
	skills, warnings, err := LoadAll(testdata("bad-name-mismatch"), "")
	require.NoError(t, err)
	require.Empty(t, skills)
	require.NotEmpty(t, warnings)
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "match") || strings.Contains(w, "mismatch") {
			found = true
		}
	}
	require.True(t, found, "expected name mismatch warning, got: %v", warnings)
}

func TestLoadAll_MissingSkillMdSkipped(t *testing.T) {
	skills, warnings, err := LoadAll(testdata("bad-no-skill-md"), "")
	require.NoError(t, err)
	require.Empty(t, skills)
	require.NotEmpty(t, warnings)
}

func TestLoadAll_InvalidDirNameSkipped(t *testing.T) {
	skills, warnings, err := LoadAll(testdata("bad-invalid-dirname"), "")
	require.NoError(t, err)
	require.Empty(t, skills)
	require.NotEmpty(t, warnings)
	require.Contains(t, warnings[0], "invalid name")
}

func TestLoadAll_BothRootsMissing_NoError(t *testing.T) {
	skills, warnings, err := LoadAll("/nonexistent/path/abc123", "/another/nonexistent/xyz")
	require.NoError(t, err)
	require.Empty(t, skills)
	require.Empty(t, warnings)
}

func TestSplitFrontmatter_PreservesHorizontalRulesInBody(t *testing.T) {
	raw := []byte("---\nname: x\ndescription: y\n---\n# Heading\n\n---\n\nmore body\n")
	fm, body, ok := SplitFrontmatter(raw)
	require.True(t, ok)
	require.Contains(t, string(fm), "name: x")
	require.Contains(t, string(body), "# Heading")
	require.Contains(t, string(body), "more body")
	require.Contains(t, string(body), "---") // HR preserved
}

func TestSplitFrontmatter_EmptyBody(t *testing.T) {
	raw := []byte("---\nname: x\ndescription: y\n---\n")
	_, body, ok := SplitFrontmatter(raw)
	require.True(t, ok)
	require.Equal(t, "", string(body))
}

func TestSplitFrontmatter_HandlesCRLF(t *testing.T) {
	// Build a CRLF SKILL.md
	crlf := "---\r\nname: git-flow\r\ndescription: test\r\n---\r\n\r\nBody here.\r\n"
	fm, body, ok := SplitFrontmatter([]byte(crlf))
	require.True(t, ok)
	require.Contains(t, string(fm), "name: git-flow")
	require.Contains(t, string(body), "Body here")
}

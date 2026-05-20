package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoad_NoFile_ReturnsDefaults(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ANTHROGO_HOME", dir)
	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, "default", string(cfg.Mode))
	require.Empty(t, cfg.AlwaysAllow)
}

func TestLoad_ParsesYAML(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ANTHROGO_HOME", dir)
	yaml := `mode: acceptEdits
alwaysAllow:
  - tool: Read
  - tool: Bash
    match: "git status*"
alwaysDeny:
  - tool: Bash
    match: "rm -rf*"
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "settings.yaml"), []byte(yaml), 0o644))
	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, "acceptEdits", string(cfg.Mode))
	require.Len(t, cfg.AlwaysAllow, 2)
	require.Equal(t, "Bash", cfg.AlwaysAllow[1].Tool)
	// NOTE: the Rule field is "Pattern" — but the YAML tag is "match" so
	// AlwaysDeny[0].Pattern should equal "rm -rf*".
	require.Equal(t, "rm -rf*", cfg.AlwaysDeny[0].Pattern)
}

func TestConfig_SessionSearchCacheSize_Default(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ANTHROGO_HOME", dir)
	// No session_search_cache_size in YAML → field stays zero (caller uses default 64).
	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, 0, cfg.SessionSearchCacheSize)
}

func TestConfig_SessionSearchCacheSize_Configured(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ANTHROGO_HOME", dir)
	yaml := "session_search_cache_size: 128\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "settings.yaml"), []byte(yaml), 0o644))
	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, 128, cfg.SessionSearchCacheSize)
}

func TestLoad_ParsesMCPServers(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ANTHROGO_HOME", dir)
	yaml := `mcpServers:
  filesystem:
    command: npx
    args: ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
  fetch:
    command: npx
    args: ["-y", "@modelcontextprotocol/server-fetch"]
    env:
      USER_AGENT: anthrogo/0.3
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "settings.yaml"), []byte(yaml), 0o644))
	cfg, err := Load()
	require.NoError(t, err)
	require.Len(t, cfg.MCPServers, 2)
	fs := cfg.MCPServers["filesystem"]
	require.Equal(t, "npx", fs.Command)
	require.Contains(t, fs.Args, "@modelcontextprotocol/server-filesystem")
	ft := cfg.MCPServers["fetch"]
	require.Equal(t, "anthrogo/0.3", ft.Env["USER_AGENT"])
}

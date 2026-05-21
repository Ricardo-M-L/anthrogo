package initconfig

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRun_RefusesOverwrite(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "settings.yaml")
	require.NoError(t, os.WriteFile(path, []byte("existing"), 0o600))
	err := Run(strings.NewReader(""), io.Discard, path, false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "already exists")
}

func TestRun_WritesFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "settings.yaml")
	// simulate hitting Enter at every prompt (accept defaults)
	in := strings.NewReader("\n\n\n\n\n\n")
	var out bytes.Buffer
	err := Run(in, &out, path, false)
	require.NoError(t, err)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(data), "provider: anthropic")
	require.Contains(t, string(data), "model: claude-sonnet-4-6")
	require.Contains(t, out.String(), "Wrote")
}

func TestRun_Force_Overwrites(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "settings.yaml")
	require.NoError(t, os.WriteFile(path, []byte("# old"), 0o600))
	in := strings.NewReader(strings.Repeat("\n", 10))
	err := Run(in, io.Discard, path, true)
	require.NoError(t, err)
	data, _ := os.ReadFile(path)
	require.NotContains(t, string(data), "# old")
}

func TestRun_ProviderSelectionAffectsModel(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "settings.yaml")
	// Provider deepseek, accept defaults for everything else
	in := strings.NewReader("deepseek\n\n\n\n\n\n")
	err := Run(in, io.Discard, path, false)
	require.NoError(t, err)
	data, _ := os.ReadFile(path)
	require.Contains(t, string(data), "provider: deepseek")
	require.Contains(t, string(data), "deepseek-chat")
}

func TestRun_InlineKeySaved(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "settings.yaml")
	// anthropic / default model / inline key / default mode / no telemetry / 0 auto-compact
	in := strings.NewReader("\n\n2\nsk-test-key\n\nn\n0\n")
	err := Run(in, io.Discard, path, false)
	require.NoError(t, err)
	data, _ := os.ReadFile(path)
	require.Contains(t, string(data), "sk-test-key")
}

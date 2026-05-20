package plugin

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/pkg/command"
	"github.com/ricardo/anthrogo/pkg/skill"
)

// TestPluginMergeFlow exercises the merge order used by cmd/anthrogo:
// LoadAll -> register contributions into command.Registry + skill.Registry.
func TestPluginMergeFlow(t *testing.T) {
	homeRoot, _ := filepath.Abs(filepath.Join("testdata", "valid-home"))
	plugins, _, err := LoadAll(homeRoot, "")
	require.NoError(t, err)
	require.NotEmpty(t, plugins)

	cmdReg := command.NewRegistry()
	skillReg := skill.NewRegistry(nil)
	for _, p := range plugins {
		for _, c := range p.Commands {
			cmdReg.Register(c)
		}
		for _, s := range p.Skills {
			ok := skillReg.Add(s)
			require.True(t, ok, "first add should succeed")
		}
	}
	// Verify the git-tools plugin contributed expected items.
	_, ok := cmdReg.Lookup("/new-branch")
	require.True(t, ok)
	sk, ok := skillReg.Get("git-flow")
	require.True(t, ok)
	require.NotEmpty(t, sk.Body)
}

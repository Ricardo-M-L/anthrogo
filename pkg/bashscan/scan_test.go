package bashscan

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestScan_SimpleCommand(t *testing.T) {
	r := Scan("ls -la")
	require.Equal(t, []string{"ls"}, r.Binaries)
	require.False(t, r.UsesSudo)
	require.False(t, r.UsesPipeOrChain)
}

func TestScan_Pipeline(t *testing.T) {
	r := Scan("ls | grep foo")
	require.Equal(t, []string{"ls", "grep"}, r.Binaries)
	require.True(t, r.UsesPipeOrChain)
}

func TestScan_Sudo(t *testing.T) {
	r := Scan("sudo rm -rf /")
	require.True(t, r.UsesSudo)
	require.Equal(t, []string{"sudo", "rm"}, r.Binaries)
}

func TestScan_AndChain(t *testing.T) {
	r := Scan("foo && bar")
	require.True(t, r.UsesPipeOrChain)
}

func TestScan_Subshell(t *testing.T) {
	r := Scan("echo $(date)")
	require.True(t, r.UsesSubshell)
}

func TestScan_Redirect(t *testing.T) {
	r := Scan("echo hi > /tmp/x")
	require.True(t, r.UsesRedirect)
}

func TestScan_ParseError(t *testing.T) {
	r := Scan("if then")
	require.NotEmpty(t, r.ParseError)
}

func TestHasBinary(t *testing.T) {
	r := Scan("ls && rm -rf /tmp/foo")
	require.True(t, r.HasBinary("rm"))
	require.False(t, r.HasBinary("git"))
}

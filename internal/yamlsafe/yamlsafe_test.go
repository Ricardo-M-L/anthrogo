package yamlsafe

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type sample struct {
	Name    string `yaml:"name"`
	Match   string `yaml:"match,omitempty"`
	Timeout int    `yaml:"timeout,omitempty"`
}

func TestUnmarshal_Valid_NoWarnings(t *testing.T) {
	var s sample
	var warns []string
	err := Unmarshal([]byte("name: foo\nmatch: bar\n"), &s, "test.yaml", &warns)
	require.NoError(t, err)
	require.Equal(t, "foo", s.Name)
	require.Equal(t, "bar", s.Match)
	require.Empty(t, warns)
}

func TestUnmarshal_UnknownField_WarnsAndFallsBack(t *testing.T) {
	var s sample
	var warns []string
	// `pattern:` is the trap: real field is `match:`. yaml.v3 default
	// silently drops it; yamlsafe should warn AND still populate the
	// known fields.
	err := Unmarshal([]byte("name: foo\npattern: bar\ntimeout: 30\n"), &s, "test.yaml", &warns)
	require.NoError(t, err, "unknown field should not be a fatal error")
	require.Equal(t, "foo", s.Name)
	require.Equal(t, 30, s.Timeout)
	require.Equal(t, "", s.Match)
	require.Len(t, warns, 1, "must warn about the unknown field")
	require.Contains(t, warns[0], "pattern")
	require.Contains(t, warns[0], "test.yaml")
}

func TestUnmarshal_SyntaxError_IsHardFail(t *testing.T) {
	var s sample
	var warns []string
	err := Unmarshal([]byte("name: : :\nfoo bar\n"), &s, "test.yaml", &warns)
	require.Error(t, err, "real syntax errors must propagate")
}

func TestUnmarshal_NilWarningsSlice_DoesNotCrash(t *testing.T) {
	var s sample
	err := Unmarshal([]byte("name: foo\nbadfield: x\n"), &s, "test.yaml", nil)
	require.NoError(t, err)
	require.Equal(t, "foo", s.Name)
}

package hooks

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestConfig_ParsesAndExpands(t *testing.T) {
	t.Setenv("HOME", "/Users/test")
	t.Setenv("ANTHROGO_FOO", "bar")
	raw := []byte(`
PreToolUse:
  - matcher: "Bash"
    command: ~/.anthrogo/hooks/audit.sh
    timeout: 15s
  - matcher: "Write|Edit"
    command: $ANTHROGO_FOO/scripts/x.sh
PostToolUse:
  - command: /abs/path/no-matcher.sh
UserPromptSubmit:
  - command: ~/inject.sh
`)
	var c Config
	require.NoError(t, yaml.Unmarshal(raw, &c))
	c.Expand()

	require.Equal(t, "/Users/test/.anthrogo/hooks/audit.sh", c.PreToolUse[0].Command)
	require.Equal(t, 15*time.Second, c.PreToolUse[0].Timeout)
	require.Equal(t, "bar/scripts/x.sh", c.PreToolUse[1].Command)
	require.Equal(t, "/abs/path/no-matcher.sh", c.PostToolUse[0].Command)
	require.Equal(t, "/Users/test/inject.sh", c.UserPromptSubmit[0].Command)
}

func TestConfig_DefaultsTimeoutByEvent(t *testing.T) {
	var c Config
	require.NoError(t, yaml.Unmarshal([]byte(`
PreToolUse:
  - command: /x.sh
Stop:
  - command: /y.sh
Notification:
  - command: /z.sh
`), &c))
	c.Expand()
	require.Equal(t, 30*time.Second, c.PreToolUse[0].Timeout)
	require.Equal(t, 10*time.Second, c.Stop[0].Timeout)
	require.Equal(t, 5*time.Second, c.Notification[0].Timeout)
}

func TestConfig_InvalidRegexpIsDroppedWithWarn(t *testing.T) {
	var c Config
	require.NoError(t, yaml.Unmarshal([]byte(`
PreToolUse:
  - matcher: "(unclosed"
    command: /a.sh
  - matcher: "Bash"
    command: /b.sh
`), &c))
	warnings := c.Validate()
	require.Len(t, warnings, 1)
	require.Contains(t, warnings[0], "(unclosed")
	require.Len(t, c.PreToolUse, 1)
	require.Equal(t, "/b.sh", c.PreToolUse[0].Command)
}

func TestConfig_AppendOverlay(t *testing.T) {
	base := Config{
		PreToolUse: []Spec{{Matcher: "Bash", Command: "/a.sh"}},
	}
	overlay := Config{
		PreToolUse: []Spec{{Matcher: "Write", Command: "/b.sh"}},
		Stop:       []Spec{{Command: "/c.sh"}},
	}
	merged := base.AppendOverlay(overlay)
	require.Len(t, merged.PreToolUse, 2)
	require.Equal(t, "/a.sh", merged.PreToolUse[0].Command)
	require.Equal(t, "/b.sh", merged.PreToolUse[1].Command)
	require.Equal(t, "/c.sh", merged.Stop[0].Command)
}

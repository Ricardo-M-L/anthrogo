package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/pkg/tool"
)

// makeElicitPermission builds a permission modal pre-loaded with a PromptElicitForm ask.
// The reply channel is buffered so handleElicitForm can send without blocking.
func makeElicitPermission(message string, schema map[string]any) (*permission, chan tool.PromptResponse) {
	reply := make(chan tool.PromptResponse, 1)
	p := &permission{
		theme:   DefaultTheme(),
		visible: true,
		pending: permissionAsk{
			req: tool.PromptRequest{
				Kind:    tool.PromptElicitForm,
				Message: message,
				Schema:  schema,
			},
			reply: reply,
		},
	}
	return p, reply
}

func keyMsg(t tea.KeyType, runes ...rune) tea.KeyMsg {
	return tea.KeyMsg{Type: t, Runes: runes}
}

func TestHandleElicitForm_EscCancels(t *testing.T) {
	p, reply := makeElicitPermission("Please fill in the form", nil)
	consumed := p.handleElicitForm(keyMsg(tea.KeyEsc))
	require.True(t, consumed)
	require.False(t, p.visible)
	require.Nil(t, p.formBuffer)
	resp := <-reply
	require.Equal(t, "cancel", resp.Action)
}

func TestHandleElicitForm_EmptyEnterDeclines(t *testing.T) {
	p, reply := makeElicitPermission("", nil)
	// buffer is empty
	consumed := p.handleElicitForm(keyMsg(tea.KeyEnter))
	require.True(t, consumed)
	require.False(t, p.visible)
	resp := <-reply
	require.Equal(t, "decline", resp.Action)
}

func TestHandleElicitForm_InvalidJSONDeclines(t *testing.T) {
	p, reply := makeElicitPermission("", nil)
	// Type some non-JSON text
	p.handleElicitForm(keyMsg(tea.KeyRunes, 'n', 'o', 't', 'j', 's', 'o', 'n'))
	consumed := p.handleElicitForm(keyMsg(tea.KeyEnter))
	require.True(t, consumed)
	resp := <-reply
	require.Equal(t, "decline", resp.Action)
	require.Contains(t, resp.Reason, "invalid JSON")
}

func TestHandleElicitForm_ValidJSONAccepts(t *testing.T) {
	p, reply := makeElicitPermission("Enter config", map[string]any{"type": "object"})
	// Simulate typing {"x":1}
	chars := []rune{'{', '"', 'x', '"', ':', '1', '}'}
	for _, ch := range chars {
		p.handleElicitForm(keyMsg(tea.KeyRunes, ch))
	}
	consumed := p.handleElicitForm(keyMsg(tea.KeyEnter))
	require.True(t, consumed)
	require.False(t, p.visible)
	resp := <-reply
	require.Equal(t, "accept", resp.Action)
	require.NotNil(t, resp.FormData)
	require.Equal(t, float64(1), resp.FormData["x"])
}

func TestHandleElicitForm_KeyRunesAppendToBuffer(t *testing.T) {
	p, _ := makeElicitPermission("", nil)
	p.handleElicitForm(keyMsg(tea.KeyRunes, 'a', 'b', 'c'))
	require.Equal(t, []rune{'a', 'b', 'c'}, p.formBuffer)
}

func TestHandleElicitForm_BackspaceRemovesLastRune(t *testing.T) {
	p, _ := makeElicitPermission("", nil)
	p.handleElicitForm(keyMsg(tea.KeyRunes, 'a', 'b', 'c'))
	p.handleElicitForm(keyMsg(tea.KeyBackspace))
	require.Equal(t, []rune{'a', 'b'}, p.formBuffer)
}

func TestHandleElicitForm_SpaceAppendsSpace(t *testing.T) {
	p, _ := makeElicitPermission("", nil)
	p.handleElicitForm(keyMsg(tea.KeySpace))
	require.Equal(t, []rune{' '}, p.formBuffer)
}

func TestHandleElicitForm_ViewRendersElicitForm(t *testing.T) {
	p, _ := makeElicitPermission("Describe yourself", map[string]any{"type": "object"})
	v := p.view()
	require.Contains(t, v, "MCP elicitation")
	require.Contains(t, v, "Describe yourself")
	require.Contains(t, v, "Schema:")
	require.Contains(t, v, "[Enter] submit")
	require.Contains(t, v, "[Esc] cancel/decline")
}

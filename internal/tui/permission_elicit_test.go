package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/pkg/tool"
)

// makeElicitPermission builds a permission modal pre-loaded with a PromptElicitForm ask.
// The reply channel is buffered so handleElicitForm can send without blocking.
// It calls show() so that formMode is set correctly based on the schema.
func makeElicitPermission(message string, schema map[string]any) (*permission, chan tool.PromptResponse) {
	reply := make(chan tool.PromptResponse, 1)
	p := &permission{
		theme: DefaultTheme(),
	}
	ask := permissionAsk{
		req: tool.PromptRequest{
			Kind:    tool.PromptElicitForm,
			Message: message,
			Schema:  schema,
		},
		reply: reply,
	}
	p.show(ask)
	return p, reply
}

func keyMsg(t tea.KeyType, runes ...rune) tea.KeyMsg {
	return tea.KeyMsg{Type: t, Runes: runes}
}

// ── Single-mode (M6.3 fallback) tests ──────────────────────────────────────

func TestHandleElicitSingle_EscCancels(t *testing.T) {
	p, reply := makeElicitPermission("Please fill in the form", nil)
	require.Equal(t, "single", p.formMode)
	consumed := p.handleElicitForm(keyMsg(tea.KeyEsc))
	require.True(t, consumed)
	require.False(t, p.visible)
	require.Nil(t, p.formBuffer)
	resp := <-reply
	require.Equal(t, "cancel", resp.Action)
}

func TestHandleElicitSingle_EmptyEnterDeclines(t *testing.T) {
	p, reply := makeElicitPermission("", nil)
	require.Equal(t, "single", p.formMode)
	consumed := p.handleElicitForm(keyMsg(tea.KeyEnter))
	require.True(t, consumed)
	require.False(t, p.visible)
	resp := <-reply
	require.Equal(t, "decline", resp.Action)
}

func TestHandleElicitSingle_InvalidJSONDeclines(t *testing.T) {
	p, reply := makeElicitPermission("", nil)
	require.Equal(t, "single", p.formMode)
	p.handleElicitForm(keyMsg(tea.KeyRunes, 'n', 'o', 't', 'j', 's', 'o', 'n'))
	consumed := p.handleElicitForm(keyMsg(tea.KeyEnter))
	require.True(t, consumed)
	resp := <-reply
	require.Equal(t, "decline", resp.Action)
	require.Contains(t, resp.Reason, "invalid JSON")
}

func TestHandleElicitSingle_ValidJSONAccepts(t *testing.T) {
	// Schema with type:object but no properties → falls back to single mode
	p, reply := makeElicitPermission("Enter config", map[string]any{"type": "object"})
	require.Equal(t, "single", p.formMode)
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

func TestHandleElicitSingle_KeyRunesAppendToBuffer(t *testing.T) {
	p, _ := makeElicitPermission("", nil)
	require.Equal(t, "single", p.formMode)
	p.handleElicitForm(keyMsg(tea.KeyRunes, 'a', 'b', 'c'))
	require.Equal(t, []rune{'a', 'b', 'c'}, p.formBuffer)
}

func TestHandleElicitSingle_BackspaceRemovesLastRune(t *testing.T) {
	p, _ := makeElicitPermission("", nil)
	p.handleElicitForm(keyMsg(tea.KeyRunes, 'a', 'b', 'c'))
	p.handleElicitForm(keyMsg(tea.KeyBackspace))
	require.Equal(t, []rune{'a', 'b'}, p.formBuffer)
}

func TestHandleElicitSingle_SpaceAppendsSpace(t *testing.T) {
	p, _ := makeElicitPermission("", nil)
	p.handleElicitForm(keyMsg(tea.KeySpace))
	require.Equal(t, []rune{' '}, p.formBuffer)
}

func TestHandleElicitSingle_ViewRendersElicitForm(t *testing.T) {
	p, _ := makeElicitPermission("Describe yourself", map[string]any{"type": "object"})
	require.Equal(t, "single", p.formMode)
	v := p.view()
	require.Contains(t, v, "MCP elicitation")
	require.Contains(t, v, "Describe yourself")
	require.Contains(t, v, "Schema:")
	require.Contains(t, v, "[Enter] submit")
	require.Contains(t, v, "[Esc] cancel/decline")
}

// ── Multi-field mode tests ─────────────────────────────────────────────────

func simpleObjectSchema(required []string, props map[string]map[string]any) map[string]any {
	properties := map[string]any{}
	for k, v := range props {
		properties[k] = map[string]any(v)
	}
	req := make([]any, len(required))
	for i, r := range required {
		req[i] = r
	}
	return map[string]any{
		"type":       "object",
		"properties": properties,
		"required":   req,
	}
}

func TestPermission_ElicitMulti_BasicSubmit(t *testing.T) {
	schema := simpleObjectSchema(
		[]string{"name"},
		map[string]map[string]any{
			"name": {"type": "string"},
			"age":  {"type": "integer"},
		},
	)
	p, reply := makeElicitPermission("Fill in details", schema)
	require.Equal(t, "multi", p.formMode)
	require.Len(t, p.formFields, 2)

	// fields are sorted: age (0), name (1)
	// Tab to name (index 1)
	p.handleElicitMulti(keyMsg(tea.KeyTab))
	require.Equal(t, 1, p.formFocus) // name field

	// type "Alice" into name
	p.handleElicitMulti(keyMsg(tea.KeyRunes, 'A', 'l', 'i', 'c', 'e'))

	// Tab back to age (index 0)
	p.handleElicitMulti(keyMsg(tea.KeyTab))
	require.Equal(t, 0, p.formFocus)

	// type "30" into age
	p.handleElicitMulti(keyMsg(tea.KeyRunes, '3', '0'))

	// submit
	consumed := p.handleElicitMulti(keyMsg(tea.KeyEnter))
	require.True(t, consumed)
	require.False(t, p.visible)

	resp := <-reply
	require.Equal(t, "accept", resp.Action)
	require.Equal(t, "Alice", resp.FormData["name"])
	require.Equal(t, 30, resp.FormData["age"])
}

func TestPermission_ElicitMulti_RequiredEmpty_Declines(t *testing.T) {
	schema := simpleObjectSchema(
		[]string{"name"},
		map[string]map[string]any{
			"name": {"type": "string"},
		},
	)
	p, reply := makeElicitPermission("", schema)
	require.Equal(t, "multi", p.formMode)

	// Don't type anything, just submit
	p.handleElicitMulti(keyMsg(tea.KeyEnter))

	resp := <-reply
	require.Equal(t, "decline", resp.Action)
	require.Contains(t, resp.Reason, "required field empty")
	require.Contains(t, resp.Reason, "name")
}

func TestPermission_ElicitMulti_InvalidInteger_Declines(t *testing.T) {
	schema := simpleObjectSchema(
		[]string{"count"},
		map[string]map[string]any{
			"count": {"type": "integer"},
		},
	)
	p, reply := makeElicitPermission("", schema)
	require.Equal(t, "multi", p.formMode)

	p.handleElicitMulti(keyMsg(tea.KeyRunes, 'n', 'o', 't', 'a', 'n', 'u', 'm'))
	p.handleElicitMulti(keyMsg(tea.KeyEnter))

	resp := <-reply
	require.Equal(t, "decline", resp.Action)
	require.Contains(t, resp.Reason, "invalid integer")
	require.Contains(t, resp.Reason, "count")
}

func TestPermission_ElicitMulti_OptionalEmpty_OmittedFromData(t *testing.T) {
	schema := simpleObjectSchema(
		[]string{"name"},
		map[string]map[string]any{
			"name":  {"type": "string"},
			"extra": {"type": "string"},
		},
	)
	p, reply := makeElicitPermission("", schema)
	require.Equal(t, "multi", p.formMode)

	// fields sorted: extra (0), name (1)
	// Tab to name
	p.handleElicitMulti(keyMsg(tea.KeyTab))
	// type into name
	p.handleElicitMulti(keyMsg(tea.KeyRunes, 'B', 'o', 'b'))
	// submit — extra is empty and optional
	p.handleElicitMulti(keyMsg(tea.KeyEnter))

	resp := <-reply
	require.Equal(t, "accept", resp.Action)
	require.Equal(t, "Bob", resp.FormData["name"])
	_, hasExtra := resp.FormData["extra"]
	require.False(t, hasExtra, "optional empty field should be omitted")
}

func TestPermission_ElicitMulti_FallbackToSingleOnComplexSchema(t *testing.T) {
	// Schema with nested object → fallback to single mode
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"nested": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"x": map[string]any{"type": "string"},
				},
			},
		},
	}
	p, _ := makeElicitPermission("", schema)
	require.Equal(t, "single", p.formMode)
	require.Nil(t, p.formFields)
}

func TestPermission_ElicitMulti_BooleanField(t *testing.T) {
	schema := simpleObjectSchema(
		[]string{"enabled"},
		map[string]map[string]any{
			"enabled": {"type": "boolean"},
		},
	)
	tests := []struct {
		input    []rune
		expected bool
	}{
		{[]rune{'t', 'r', 'u', 'e'}, true},
		{[]rune{'y', 'e', 's'}, true},
		{[]rune{'y'}, true},
		{[]rune{'1'}, true},
		{[]rune{'f', 'a', 'l', 's', 'e'}, false},
		{[]rune{'n', 'o'}, false},
		{[]rune{'n'}, false},
		{[]rune{'0'}, false},
	}
	for _, tt := range tests {
		p, reply := makeElicitPermission("", schema)
		require.Equal(t, "multi", p.formMode)
		p.handleElicitMulti(keyMsg(tea.KeyRunes, tt.input...))
		p.handleElicitMulti(keyMsg(tea.KeyEnter))
		resp := <-reply
		require.Equal(t, "accept", resp.Action, "input=%q", string(tt.input))
		require.Equal(t, tt.expected, resp.FormData["enabled"], "input=%q", string(tt.input))
	}
}

func TestPermission_ElicitMulti_InvalidBoolean_Declines(t *testing.T) {
	schema := simpleObjectSchema(
		[]string{"flag"},
		map[string]map[string]any{
			"flag": {"type": "boolean"},
		},
	)
	p, reply := makeElicitPermission("", schema)
	p.handleElicitMulti(keyMsg(tea.KeyRunes, 'm', 'a', 'y', 'b', 'e'))
	p.handleElicitMulti(keyMsg(tea.KeyEnter))
	resp := <-reply
	require.Equal(t, "decline", resp.Action)
	require.Contains(t, resp.Reason, "invalid boolean")
}

func TestPermission_ElicitMulti_TabNavigation(t *testing.T) {
	schema := simpleObjectSchema(
		nil,
		map[string]map[string]any{
			"a": {"type": "string"},
			"b": {"type": "string"},
			"c": {"type": "string"},
		},
	)
	p, _ := makeElicitPermission("", schema)
	require.Equal(t, 0, p.formFocus)
	p.handleElicitMulti(keyMsg(tea.KeyTab))
	require.Equal(t, 1, p.formFocus)
	p.handleElicitMulti(keyMsg(tea.KeyTab))
	require.Equal(t, 2, p.formFocus)
	p.handleElicitMulti(keyMsg(tea.KeyTab))
	require.Equal(t, 0, p.formFocus) // wraps around
	p.handleElicitMulti(keyMsg(tea.KeyShiftTab))
	require.Equal(t, 2, p.formFocus) // wraps backward
}

func TestPermission_ElicitMulti_EscCancels(t *testing.T) {
	schema := simpleObjectSchema(
		nil,
		map[string]map[string]any{
			"x": {"type": "string"},
		},
	)
	p, reply := makeElicitPermission("", schema)
	require.Equal(t, "multi", p.formMode)
	consumed := p.handleElicitMulti(keyMsg(tea.KeyEsc))
	require.True(t, consumed)
	require.False(t, p.visible)
	require.Empty(t, p.formMode)
	resp := <-reply
	require.Equal(t, "cancel", resp.Action)
}

func TestPermission_ElicitMulti_ViewRendersFields(t *testing.T) {
	schema := simpleObjectSchema(
		[]string{"username"},
		map[string]map[string]any{
			"username": {"type": "string", "description": "Your login name"},
			"age":      {"type": "integer"},
		},
	)
	p, _ := makeElicitPermission("Please register", schema)
	require.Equal(t, "multi", p.formMode)
	v := p.view()
	require.Contains(t, v, "MCP elicitation")
	require.Contains(t, v, "Please register")
	require.Contains(t, v, "age")
	require.Contains(t, v, "username")
	require.Contains(t, v, "(required)")
	require.Contains(t, v, "Your login name")
	require.Contains(t, v, "[Tab/Shift-Tab] navigate")
	require.Contains(t, v, "[Enter] submit")
	require.Contains(t, v, "[Esc] cancel")
}

func TestPermission_ElicitMulti_NumberField(t *testing.T) {
	schema := simpleObjectSchema(
		[]string{"price"},
		map[string]map[string]any{
			"price": {"type": "number"},
		},
	)
	p, reply := makeElicitPermission("", schema)
	require.Equal(t, "multi", p.formMode)
	p.handleElicitMulti(keyMsg(tea.KeyRunes, '3', '.', '1', '4'))
	p.handleElicitMulti(keyMsg(tea.KeyEnter))
	resp := <-reply
	require.Equal(t, "accept", resp.Action)
	require.InDelta(t, 3.14, resp.FormData["price"], 0.001)
}

// ── M9.7 item 1: Arrow keys + cursor ──────────────────────────────────────────

func TestPermission_ElicitMulti_ArrowKeyMovesCursor(t *testing.T) {
	schema := simpleObjectSchema(nil, map[string]map[string]any{
		"word": {"type": "string"},
	})
	p, _ := makeElicitPermission("", schema)
	// type "abc"
	p.handleElicitMulti(keyMsg(tea.KeyRunes, 'a', 'b', 'c'))
	require.Equal(t, 3, p.formFields[0].cursor)
	// move left twice
	p.handleElicitMulti(keyMsg(tea.KeyLeft))
	p.handleElicitMulti(keyMsg(tea.KeyLeft))
	require.Equal(t, 1, p.formFields[0].cursor)
	// move right once
	p.handleElicitMulti(keyMsg(tea.KeyRight))
	require.Equal(t, 2, p.formFields[0].cursor)
	// Home → 0
	p.handleElicitMulti(keyMsg(tea.KeyHome))
	require.Equal(t, 0, p.formFields[0].cursor)
	// End → 3
	p.handleElicitMulti(keyMsg(tea.KeyEnd))
	require.Equal(t, 3, p.formFields[0].cursor)
}

func TestPermission_ElicitMulti_InsertAtCursor(t *testing.T) {
	schema := simpleObjectSchema(nil, map[string]map[string]any{
		"w": {"type": "string"},
	})
	p, _ := makeElicitPermission("", schema)
	// type "ac", move left, insert "b"
	p.handleElicitMulti(keyMsg(tea.KeyRunes, 'a', 'c'))
	p.handleElicitMulti(keyMsg(tea.KeyLeft))
	p.handleElicitMulti(keyMsg(tea.KeyRunes, 'b'))
	require.Equal(t, []rune{'a', 'b', 'c'}, p.formFields[0].buffer)
	require.Equal(t, 2, p.formFields[0].cursor)
}

func TestPermission_ElicitMulti_BackspaceMid(t *testing.T) {
	schema := simpleObjectSchema(nil, map[string]map[string]any{
		"w": {"type": "string"},
	})
	p, _ := makeElicitPermission("", schema)
	// type "abc", move left, backspace removes 'b' (char before cursor)
	p.handleElicitMulti(keyMsg(tea.KeyRunes, 'a', 'b', 'c'))
	p.handleElicitMulti(keyMsg(tea.KeyLeft)) // cursor at 2, between b and c
	p.handleElicitMulti(keyMsg(tea.KeyBackspace))
	require.Equal(t, []rune{'a', 'c'}, p.formFields[0].buffer)
	require.Equal(t, 1, p.formFields[0].cursor)
}

func TestPermission_ElicitMulti_DeleteMid(t *testing.T) {
	schema := simpleObjectSchema(nil, map[string]map[string]any{
		"w": {"type": "string"},
	})
	p, _ := makeElicitPermission("", schema)
	// type "abc", Home, delete removes 'a' (char at cursor)
	p.handleElicitMulti(keyMsg(tea.KeyRunes, 'a', 'b', 'c'))
	p.handleElicitMulti(keyMsg(tea.KeyHome))
	p.handleElicitMulti(keyMsg(tea.KeyDelete))
	require.Equal(t, []rune{'b', 'c'}, p.formFields[0].buffer)
	require.Equal(t, 0, p.formFields[0].cursor)
}

// ── M9.7 item 2: Multiline string (Ctrl+J) ────────────────────────────────────

func TestPermission_ElicitMulti_MultilineString_AltEnterInsertsNewline(t *testing.T) {
	schema := simpleObjectSchema(nil, map[string]map[string]any{
		"msg": {"type": "string"},
	})
	p, _ := makeElicitPermission("", schema)
	p.handleElicitMulti(keyMsg(tea.KeyRunes, 'h', 'i'))
	// Ctrl+J inserts newline
	p.handleElicitMulti(keyMsg(tea.KeyCtrlJ))
	p.handleElicitMulti(keyMsg(tea.KeyRunes, 'b', 'y', 'e'))
	require.Equal(t, []rune{'h', 'i', '\n', 'b', 'y', 'e'}, p.formFields[0].buffer)
	require.Equal(t, 6, p.formFields[0].cursor)
}

// ── M9.7 item 3: Enum cycling + submit ────────────────────────────────────────

func TestPermission_ElicitMulti_EnumCycling(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"color": map[string]any{
				"type": "string",
				"enum": []any{"red", "green", "blue"},
			},
		},
		"required": []any{},
	}
	p, _ := makeElicitPermission("", schema)
	require.Equal(t, "multi", p.formMode)
	require.Equal(t, "enum", p.formFields[0].typ)
	require.Equal(t, 0, p.formFields[0].enumIdx)
	// Right cycles forward
	p.handleElicitMulti(keyMsg(tea.KeyRight))
	require.Equal(t, 1, p.formFields[0].enumIdx)
	p.handleElicitMulti(keyMsg(tea.KeyRight))
	require.Equal(t, 2, p.formFields[0].enumIdx)
	// Wraps around
	p.handleElicitMulti(keyMsg(tea.KeyRight))
	require.Equal(t, 0, p.formFields[0].enumIdx)
	// Left cycles backward
	p.handleElicitMulti(keyMsg(tea.KeyLeft))
	require.Equal(t, 2, p.formFields[0].enumIdx)
	// Up/Down also cycle
	p.handleElicitMulti(keyMsg(tea.KeyDown))
	require.Equal(t, 0, p.formFields[0].enumIdx)
	p.handleElicitMulti(keyMsg(tea.KeyUp))
	require.Equal(t, 2, p.formFields[0].enumIdx)
}

func TestPermission_ElicitMulti_EnumSubmit(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"size": map[string]any{
				"type": "string",
				"enum": []any{"small", "medium", "large"},
			},
		},
		"required": []any{},
	}
	p, reply := makeElicitPermission("", schema)
	// cycle to "medium"
	p.handleElicitMulti(keyMsg(tea.KeyRight))
	require.Equal(t, 1, p.formFields[0].enumIdx)
	p.handleElicitMulti(keyMsg(tea.KeyEnter))
	resp := <-reply
	require.Equal(t, "accept", resp.Action)
	require.Equal(t, "medium", resp.FormData["size"])
}

// ── M9.7 item 4: Schema default values ────────────────────────────────────────

func TestPermission_ElicitMulti_DefaultValuePrefilled(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name":  map[string]any{"type": "string", "default": "Alice"},
			"age":   map[string]any{"type": "integer", "default": float64(30)},
			"score": map[string]any{"type": "number", "default": float64(9.5)},
			"flag":  map[string]any{"type": "boolean", "default": true},
		},
		"required": []any{},
	}
	p, reply := makeElicitPermission("", schema)
	require.Equal(t, "multi", p.formMode)
	// fields sorted: age, flag, name, score
	// verify pre-fills
	byName := map[string]formField{}
	for _, f := range p.formFields {
		byName[f.name] = f
	}
	require.Equal(t, []rune("Alice"), byName["name"].buffer)
	require.Equal(t, 5, byName["name"].cursor)
	require.Equal(t, []rune("30"), byName["age"].buffer)
	require.Equal(t, []rune("9.5"), byName["score"].buffer)
	require.Equal(t, []rune("true"), byName["flag"].buffer)

	// submit without changes — should accept defaults
	p.handleElicitMulti(keyMsg(tea.KeyEnter))
	resp := <-reply
	require.Equal(t, "accept", resp.Action)
	require.Equal(t, "Alice", resp.FormData["name"])
	require.Equal(t, 30, resp.FormData["age"])
}

func TestPermission_ElicitMulti_DefaultEnumPrefilled(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"color": map[string]any{
				"type":    "string",
				"enum":    []any{"red", "green", "blue"},
				"default": "green",
			},
		},
		"required": []any{},
	}
	p, reply := makeElicitPermission("", schema)
	require.Equal(t, "enum", p.formFields[0].typ)
	require.Equal(t, 1, p.formFields[0].enumIdx) // "green" is index 1
	p.handleElicitMulti(keyMsg(tea.KeyEnter))
	resp := <-reply
	require.Equal(t, "accept", resp.Action)
	require.Equal(t, "green", resp.FormData["color"])
}

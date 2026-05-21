package tool

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBrowser_Schema_ContainsModeEnum verifies the schema declares the 4
// expected mode values without starting Chrome.
func TestBrowser_Schema_ContainsModeEnum(t *testing.T) {
	b := &Browser{}
	schema := b.Schema()

	props, ok := schema["properties"].(map[string]any)
	require.True(t, ok, "schema must have properties")

	modeField, ok := props["mode"].(map[string]any)
	require.True(t, ok, "schema must have mode field")

	enum, ok := modeField["enum"].([]string)
	require.True(t, ok, "mode must have string enum")

	assert.Equal(t, []string{"get", "click", "text", "screenshot"}, enum)

	required, ok := schema["required"].([]string)
	require.True(t, ok, "schema must have required")
	assert.Contains(t, required, "mode")
}

// TestBrowser_Call_RejectsMissingMode ensures an empty input returns an error
// without attempting to start Chrome.
func TestBrowser_Call_RejectsMissingMode(t *testing.T) {
	b := &Browser{}
	result, err := b.Call(context.Background(), map[string]any{}, nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Text, "mode is required")
}

// TestBrowser_Call_RejectsBadMode ensures an unrecognised mode returns an
// error without starting Chrome (validation happens before Chrome init).
func TestBrowser_Call_RejectsBadMode(t *testing.T) {
	b := &Browser{}
	result, err := b.Call(context.Background(), map[string]any{"mode": "launch_nukes"}, nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Text, "unknown mode")
}

// TestBrowser_Call_GetWithoutURL_IsError checks that mode=get with no url
// field returns an error without starting Chrome.
func TestBrowser_Call_GetWithoutURL_IsError(t *testing.T) {
	b := &Browser{}
	result, err := b.Call(context.Background(), map[string]any{"mode": "get"}, nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Text, "url is required")
}

// TestBrowser_Call_ClickWithoutSelector_IsError checks that mode=click with
// no selector returns an error without starting Chrome.
func TestBrowser_Call_ClickWithoutSelector_IsError(t *testing.T) {
	b := &Browser{}
	result, err := b.Call(context.Background(), map[string]any{"mode": "click"}, nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Text, "selector is required")
}

// TestBrowser_E2E_GetAboutBlank is an end-to-end smoke test that requires a
// real Chrome installation. Only runs when ANTHROGO_E2E_BROWSER=1.
func TestBrowser_E2E_GetAboutBlank(t *testing.T) {
	if os.Getenv("ANTHROGO_E2E_BROWSER") != "1" {
		t.Skip("requires Chrome; set ANTHROGO_E2E_BROWSER=1 to run")
	}
	b := &Browser{}
	defer b.Close()

	result, err := b.Call(context.Background(), map[string]any{
		"mode": "get",
		"url":  "about:blank",
	}, nil)
	require.NoError(t, err)
	assert.False(t, result.IsError, "unexpected error: %s", result.Text)
	assert.Contains(t, result.Text, "navigated to about:blank")
}

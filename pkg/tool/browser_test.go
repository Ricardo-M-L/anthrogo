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

// TestBrowser_Screenshot_RejectsRelativePath verifies that a relative out_path
// is rejected before Chrome is started (no Chrome needed).
func TestBrowser_Screenshot_RejectsRelativePath(t *testing.T) {
	b := &Browser{}
	result, err := b.Call(context.Background(), map[string]any{
		"mode":     "screenshot",
		"out_path": "relative/path/shot.png",
	}, nil)
	require.NoError(t, err)
	assert.True(t, result.IsError)
	assert.Contains(t, result.Text, "out_path must be an absolute path")
}

// TestBrowser_Close_RemovesUserDataDir verifies that Close removes the temp
// userDataDir without requiring a real Chrome instance.
func TestBrowser_Close_RemovesUserDataDir(t *testing.T) {
	dir := t.TempDir()
	b := &Browser{userDataDir: dir}
	// Confirm the dir exists before closing.
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("temp dir should exist before Close: %v", err)
	}
	if err := b.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("expected userDataDir to be removed after Close")
	}
	// Idempotency: second Close should not error.
	if err := b.Close(); err != nil {
		t.Fatalf("second Close returned error: %v", err)
	}
}

// TestBrowser_Close_Idempotent verifies that calling Close twice on a Browser
// that has a userDataDir does not panic and that the second call is a no-op.
func TestBrowser_Close_Idempotent(t *testing.T) {
	dir := t.TempDir()
	b := &Browser{userDataDir: dir}
	if err := b.Close(); err != nil {
		t.Fatalf("first Close returned error: %v", err)
	}
	// Second close: userDataDir is now "" so no RemoveAll; should not panic.
	if err := b.Close(); err != nil {
		t.Fatalf("second Close returned error: %v", err)
	}
}

// TestBrowser_Schema_HasScreenshotMode verifies the schema enum includes "screenshot".
func TestBrowser_Schema_HasScreenshotMode(t *testing.T) {
	b := &Browser{}
	schema := b.Schema()

	props, ok := schema["properties"].(map[string]any)
	require.True(t, ok, "schema must have properties")

	modeField, ok := props["mode"].(map[string]any)
	require.True(t, ok, "schema must have mode field")

	enum, ok := modeField["enum"].([]string)
	require.True(t, ok, "mode must have string enum")

	found := false
	for _, v := range enum {
		if v == "screenshot" {
			found = true
			break
		}
	}
	assert.True(t, found, "schema enum must include 'screenshot'")
}

// TestBrowser_Close_NoUserDataDir_NoOp verifies that Close on a fresh Browser{}
// with no userDataDir returns nil without calling os.RemoveAll("").
func TestBrowser_Close_NoUserDataDir_NoOp(t *testing.T) {
	b := &Browser{} // empty userDataDir
	if err := b.Close(); err != nil {
		t.Fatalf("Close on fresh Browser returned error: %v", err)
	}
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

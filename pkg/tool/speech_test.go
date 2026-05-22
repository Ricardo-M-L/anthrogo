package tool

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSpeechToText_MissingPath verifies that omitting path returns an error.
func TestSpeechToText_MissingPath(t *testing.T) {
	res, err := (&SpeechToText{}).Call(context.Background(), map[string]any{}, nil)
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Text, "path is required")
}

// TestSpeechToText_MissingFile verifies that a non-existent audio file returns an error.
func TestSpeechToText_MissingFile(t *testing.T) {
	res, err := (&SpeechToText{}).Call(context.Background(), map[string]any{
		"path": "/tmp/anthrogo-test-nonexistent-audio-file.wav",
	}, nil)
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Text, "audio file not found")
}

// TestSpeechToText_NoWhisperBinary verifies that a missing binary returns an
// error with the install hint. Note: /tmp/x.wav won't exist so the file check
// fires first — the result is still IsError.
func TestSpeechToText_NoWhisperBinary(t *testing.T) {
	res, err := (&SpeechToText{}).Call(context.Background(), map[string]any{
		"path":   "/tmp/x.wav",
		"binary": "/nonexistent/whisper",
	}, nil)
	require.NoError(t, err)
	require.True(t, res.IsError)
}

// TestSpeechToText_NoWhisperBinaryWithExistingFile verifies that when a file
// exists but the whisper binary is missing, the install hint is returned.
func TestSpeechToText_NoWhisperBinaryWithExistingFile(t *testing.T) {
	// Create a temp file so the stat check passes
	f, err := os.CreateTemp("", "anthrogo-stt-test-*.wav")
	require.NoError(t, err)
	defer os.Remove(f.Name())
	f.Close()

	res, callErr := (&SpeechToText{}).Call(context.Background(), map[string]any{
		"path":   f.Name(),
		"binary": "/nonexistent/whisper-binary-xyz",
	}, nil)
	require.NoError(t, callErr)
	require.True(t, res.IsError)
	require.Contains(t, res.Text, "not on PATH")
	require.Contains(t, res.Text, "install")
}

// TestTextToSpeech_MissingText verifies that omitting text returns an error.
func TestTextToSpeech_MissingText(t *testing.T) {
	res, err := (&TextToSpeech{}).Call(context.Background(), map[string]any{}, nil)
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Text, "text is required")
}

// TestTextToSpeech_DarwinSay synthesizes a short phrase to a temp file on macOS.
// Skipped when not on darwin or when 'say' is not on PATH.
func TestTextToSpeech_DarwinSay(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin only")
	}
	if _, err := exec.LookPath("say"); err != nil {
		t.Skip("no 'say' binary")
	}
	out := filepath.Join(t.TempDir(), "out.aiff")
	res, err := (&TextToSpeech{}).Call(context.Background(), map[string]any{
		"text":   "hello",
		"output": out,
	}, nil)
	require.NoError(t, err)
	require.False(t, res.IsError)
	info, err := os.Stat(out)
	require.NoError(t, err)
	require.Greater(t, info.Size(), int64(0))
}

// TestSpeechToText_Schema verifies the schema has required fields.
func TestSpeechToText_Schema(t *testing.T) {
	s := (&SpeechToText{}).Schema()
	require.Equal(t, "object", s["type"])
	props, ok := s["properties"].(map[string]any)
	require.True(t, ok)
	_, hasPath := props["path"]
	_, hasModel := props["model"]
	_, hasBinary := props["binary"]
	require.True(t, hasPath)
	require.True(t, hasModel)
	require.True(t, hasBinary)
	req, ok := s["required"].([]string)
	require.True(t, ok)
	require.Equal(t, []string{"path"}, req)
}

// TestTextToSpeech_Schema verifies the schema has required fields.
func TestTextToSpeech_Schema(t *testing.T) {
	s := (&TextToSpeech{}).Schema()
	require.Equal(t, "object", s["type"])
	props, ok := s["properties"].(map[string]any)
	require.True(t, ok)
	_, hasText := props["text"]
	_, hasOutput := props["output"]
	_, hasVoice := props["voice"]
	require.True(t, hasText)
	require.True(t, hasOutput)
	require.True(t, hasVoice)
	req, ok := s["required"].([]string)
	require.True(t, ok)
	require.Equal(t, []string{"text"}, req)
}

// TestSpeechToText_Metadata verifies Name, IsReadOnly, IsConcurrencySafe.
func TestSpeechToText_Metadata(t *testing.T) {
	tool := &SpeechToText{}
	require.Equal(t, "SpeechToText", tool.Name())
	require.True(t, tool.IsReadOnly())
	require.True(t, tool.IsConcurrencySafe())
}

// TestTextToSpeech_Metadata verifies Name, IsReadOnly, IsConcurrencySafe.
func TestTextToSpeech_Metadata(t *testing.T) {
	tool := &TextToSpeech{}
	require.Equal(t, "TextToSpeech", tool.Name())
	require.False(t, tool.IsReadOnly())
	require.False(t, tool.IsConcurrencySafe())
}

// TestTextToSpeech_UserFacingName verifies the truncation behavior.
func TestTextToSpeech_UserFacingName(t *testing.T) {
	tool := &TextToSpeech{}
	require.Equal(t, "TextToSpeech", tool.UserFacingName(map[string]any{}))
	require.Equal(t, "TextToSpeech hello world", tool.UserFacingName(map[string]any{"text": "hello world"}))

	long := "this is a very long text that exceeds forty characters in length"
	name := tool.UserFacingName(map[string]any{"text": long})
	require.True(t, len(name) < len("TextToSpeech ")+len(long))
	require.Contains(t, name, "…")
}

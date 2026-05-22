package tool

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// SpeechToText transcribes an audio file using the whisper CLI.
type SpeechToText struct{ DefaultPermission }

func (*SpeechToText) Name() string                      { return "SpeechToText" }
func (*SpeechToText) Description(context.Context) string { return speechToTextDescription }
func (*SpeechToText) UserFacingName(input map[string]any) string {
	if p, _ := input["path"].(string); p != "" {
		return "SpeechToText " + p
	}
	return "SpeechToText"
}
func (*SpeechToText) IsReadOnly() bool        { return true }
func (*SpeechToText) IsConcurrencySafe() bool { return true }

func (*SpeechToText) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Audio file path (wav/mp3/m4a/flac)",
			},
			"model": map[string]any{
				"type":        "string",
				"description": "Whisper model name (tiny|base|small|medium|large; default base)",
			},
			"binary": map[string]any{
				"type":        "string",
				"description": "Optional path to whisper CLI binary; default 'whisper' on PATH",
			},
		},
		"required": []string{"path"},
	}
}

func (*SpeechToText) Call(ctx context.Context, input map[string]any, _ *Context) (Result, error) {
	path, _ := input["path"].(string)
	if path == "" {
		return errResult("path is required"), nil
	}
	if _, err := os.Stat(path); err != nil {
		return errResult("audio file not found: " + path), nil
	}

	binary, _ := input["binary"].(string)
	if binary == "" {
		binary = "whisper"
	}
	if _, err := exec.LookPath(binary); err != nil {
		return errResult(fmt.Sprintf("speech: %s not on PATH (install: pip install openai-whisper or pipx install openai-whisper)", binary)), nil
	}

	model, _ := input["model"].(string)
	if model == "" {
		model = "base"
	}

	tmpDir, err := os.MkdirTemp("", "anthrogo-stt-*")
	if err != nil {
		return errResult(err.Error()), nil
	}
	defer os.RemoveAll(tmpDir)

	cmd := exec.CommandContext(ctx, binary, path, "--model", model, "--output_format", "txt", "--output_dir", tmpDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return Result{Type: ResultText, Text: string(out), ForLLM: string(out), IsError: true}, nil
	}

	// The output file is <tmpDir>/<basename without ext>.txt
	base := filepath.Base(path)
	if ext := filepath.Ext(base); ext != "" {
		base = strings.TrimSuffix(base, ext)
	}
	txtPath := filepath.Join(tmpDir, base+".txt")
	txtData, err := os.ReadFile(txtPath)
	if err != nil {
		return Result{Type: ResultText, Text: "speech: transcript file not found at " + txtPath, ForLLM: "speech: transcript file not found", IsError: true}, nil
	}
	text := strings.TrimSpace(string(txtData))
	return Result{Type: ResultText, Text: text, ForLLM: text}, nil
}

const speechToTextDescription = "Transcribe an audio file to text using OpenAI Whisper (whisper CLI on PATH)."

// TextToSpeech synthesizes speech from text using a platform system synthesizer.
type TextToSpeech struct{ DefaultPermission }

func (*TextToSpeech) Name() string                      { return "TextToSpeech" }
func (*TextToSpeech) Description(context.Context) string { return textToSpeechDescription }
func (*TextToSpeech) UserFacingName(input map[string]any) string {
	if t, _ := input["text"].(string); t != "" {
		if len(t) > 40 {
			return "TextToSpeech " + t[:40] + "…"
		}
		return "TextToSpeech " + t
	}
	return "TextToSpeech"
}
func (*TextToSpeech) IsReadOnly() bool        { return false }
func (*TextToSpeech) IsConcurrencySafe() bool { return false }

func (*TextToSpeech) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text": map[string]any{
				"type": "string",
			},
			"output": map[string]any{
				"type":        "string",
				"description": "Output audio file path. Optional; if empty, speak now via system synthesizer.",
			},
			"voice": map[string]any{
				"type":        "string",
				"description": "Voice name (system-specific)",
			},
		},
		"required": []string{"text"},
	}
}

func (*TextToSpeech) Call(ctx context.Context, input map[string]any, _ *Context) (Result, error) {
	text, _ := input["text"].(string)
	if text == "" {
		return errResult("text is required"), nil
	}
	output, _ := input["output"].(string)
	voice, _ := input["voice"].(string)

	var args []string
	var bin string
	switch runtime.GOOS {
	case "darwin":
		bin = "say"
		if voice != "" {
			args = append(args, "-v", voice)
		}
		if output != "" {
			// macOS say writes AIFF; -o for output file
			args = append(args, "-o", output)
		}
		args = append(args, text)
	case "linux":
		if _, err := exec.LookPath("espeak"); err == nil {
			bin = "espeak"
		} else if _, err := exec.LookPath("espeak-ng"); err == nil {
			bin = "espeak-ng"
		} else {
			return errResult("speech: install espeak or espeak-ng"), nil
		}
		if voice != "" {
			args = append(args, "-v", voice)
		}
		if output != "" {
			args = append(args, "-w", output)
		}
		args = append(args, text)
	default:
		return errResult("speech: TextToSpeech not yet supported on " + runtime.GOOS), nil
	}

	if _, err := exec.LookPath(bin); err != nil {
		return errResult("speech: " + bin + " not on PATH"), nil
	}

	cmd := exec.CommandContext(ctx, bin, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return Result{Type: ResultText, Text: string(out), ForLLM: string(out), IsError: true}, nil
	}
	msg := "spoken"
	if output != "" {
		msg = "saved to " + output
	}
	return Result{Type: ResultText, Text: msg, ForLLM: msg}, nil
}

const textToSpeechDescription = "Synthesize speech from text. Uses 'say' (macOS), 'espeak' (Linux), or 'powershell' (Windows). Saves to 'output' file or plays live."

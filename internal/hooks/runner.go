package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

const (
	maxStdoutBytes = 1 << 20 // 1 MiB
	maxStderrBytes = 1 << 20
)

// Result is what the runner reports back. Output is nil if stdout was empty
// or not parseable JSON.
type Result struct {
	ExitCode int
	Stdout   []byte
	Stderr   []byte
	Output   *Output
	TimedOut bool
}

// RunHook spawns `spec.Command`, feeds `payload` (marshaled JSON) to stdin,
// waits up to spec.Timeout. Always returns a Result on completion or timeout;
// returns a non-nil error only on setup failure (e.g. cmd.Start() failure).
func RunHook(ctx context.Context, spec Spec, payload any) (*Result, error) {
	timeout := spec.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	encoded, _ := json.Marshal(payload)

	cmd := exec.CommandContext(runCtx, "sh", "-c", spec.Command)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Env = sanitizedEnv()
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		if pgid, err := syscall.Getpgid(cmd.Process.Pid); err == nil {
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
		} else {
			_ = cmd.Process.Kill()
		}
		return nil
	}

	// Use bytes.NewReader for stdin — avoids any goroutine / pipe race.
	cmd.Stdin = bytes.NewReader(encoded)

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &limitedWriter{w: &stdoutBuf, n: maxStdoutBytes}
	cmd.Stderr = &limitedWriter{w: &stderrBuf, n: maxStderrBytes}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start hook %s: %w", spec.Command, err)
	}

	waitErr := cmd.Wait()

	r := &Result{
		Stdout: stdoutBuf.Bytes(),
		Stderr: stderrBuf.Bytes(),
	}

	switch {
	case waitErr == nil:
		r.ExitCode = 0
	case errors.Is(runCtx.Err(), context.DeadlineExceeded):
		r.TimedOut = true
		r.ExitCode = -1
	default:
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			r.ExitCode = exitErr.ExitCode()
		} else {
			return nil, waitErr
		}
	}

	if len(r.Stdout) > 0 {
		var out Output
		if err := json.Unmarshal(r.Stdout, &out); err == nil {
			r.Output = &out
		}
	}
	return r, nil
}

// limitedWriter caps how much we collect; further writes are dropped silently.
type limitedWriter struct {
	w *bytes.Buffer
	n int
}

func (l *limitedWriter) Write(p []byte) (int, error) {
	if l.n <= 0 {
		return len(p), nil
	}
	if len(p) > l.n {
		// Truncate to fit; pretend full absorption so exec's copier doesn't error.
		clipped := p[:l.n]
		n, err := l.w.Write(clipped)
		l.n -= n
		return len(p), err
	}
	n, err := l.w.Write(p)
	l.n -= n
	return n, err
}

// sanitizedEnv returns the parent environment with credential-bearing
// variables stripped. Hooks execute user-supplied scripts; passing through
// every API key would let a malicious or compromised hook exfiltrate them.
// Callers wanting an unfiltered hook env can opt back in via the
// ANTHROGO_HOOKS_FULL_ENV=1 escape hatch.
func sanitizedEnv() []string {
	if os.Getenv("ANTHROGO_HOOKS_FULL_ENV") == "1" {
		return os.Environ()
	}
	denyPrefixes := []string{
		"ANTHROPIC_",
		"OPENAI_",
		"GROQ_",
		"MISTRAL_",
		"COHERE_",
		"DEEPSEEK_",
		"KIMI_",
		"MOONSHOT_",
		"MINIMAX_",
		"GLM_",
		"ZHIPU_",
		"AWS_",
		"AZURE_",
		"GOOGLE_",
		"GCP_",
		"VERTEX_",
		"HUGGINGFACE_",
		"SLACK_",
		"GITHUB_TOKEN",
		"GH_TOKEN",
		"NPM_TOKEN",
		"PYPI_",
		"ANTHROGO_EMBED_",
		"ANTHROGO_IMAGE_",
	}
	denyExact := map[string]bool{
		"GITHUB_TOKEN":  true,
		"GH_TOKEN":      true,
		"NPM_TOKEN":     true,
		"ANTHROGO_API_KEY": true,
	}
	out := os.Environ()[:0:0]
	for _, e := range os.Environ() {
		eq := strings.IndexByte(e, '=')
		if eq < 0 {
			out = append(out, e)
			continue
		}
		name := e[:eq]
		if denyExact[name] {
			continue
		}
		drop := false
		for _, p := range denyPrefixes {
			if strings.HasPrefix(name, p) {
				drop = true
				break
			}
		}
		if drop {
			continue
		}
		out = append(out, e)
	}
	return out
}

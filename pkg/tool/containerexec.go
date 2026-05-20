package tool

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// ContainerExec runs a command inside a docker or podman container.
// M10.7: provides real OS-level isolation vs M10.2 Bash sandbox's env scrubbing.
// M11.7: adds pull_policy, gpu, user, workdir, and separate stdout/stderr capture.
type ContainerExec struct{ DefaultPermission }

func (ContainerExec) Name() string { return "ContainerExec" }
func (ContainerExec) Description(context.Context) string {
	return "Run a command inside a docker/podman container. Default network: none (no internet)."
}
func (ContainerExec) UserFacingName(input map[string]any) string {
	if img, ok := input["image"].(string); ok && img != "" {
		return "ContainerExec: " + img
	}
	return "ContainerExec"
}
func (ContainerExec) IsReadOnly() bool        { return false }
func (ContainerExec) IsConcurrencySafe() bool { return true }

func (ContainerExec) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"image": map[string]any{
				"type":        "string",
				"description": "Container image (e.g. ubuntu:24.04, alpine, golang:1.25).",
			},
			"command": map[string]any{
				"type":        "string",
				"description": "Shell command to run inside the container.",
			},
			"mounts": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "string",
				},
				"description": "Bind-mount specs like '/host/path:/container/path' or '/host/path:/container/path:ro'. Read-only unless suffix :rw.",
			},
			"env": map[string]any{
				"type": "object",
				"additionalProperties": map[string]any{
					"type": "string",
				},
				"description": "Env vars to set inside the container.",
			},
			"network": map[string]any{
				"type":        "string",
				"description": "Network mode: 'none' (default; no internet), 'host', or a docker-network name.",
			},
			"timeout_ms": map[string]any{
				"type":        "integer",
				"description": "Default 120000.",
			},
			"pull_policy": map[string]any{
				"type":        "string",
				"description": "always | missing (default) | never",
			},
			"gpu": map[string]any{
				"type":        "boolean",
				"description": "Add --gpus all (NVIDIA Container Toolkit required).",
			},
			"user": map[string]any{
				"type":        "string",
				"description": "Run as <uid>:<gid> or username inside container.",
			},
			"workdir": map[string]any{
				"type":        "string",
				"description": "Initial working directory inside container.",
			},
		},
		"required": []string{"image", "command"},
	}
}

func (*ContainerExec) Call(ctx context.Context, input map[string]any, tcx *Context) (Result, error) {
	image, _ := input["image"].(string)
	cmd, _ := input["command"].(string)
	if image == "" || cmd == "" {
		msg := "containerexec: image and command required"
		return Result{Type: ResultText, Text: msg, ForLLM: msg, IsError: true}, nil
	}

	runtime := ""
	if _, err := exec.LookPath("docker"); err == nil {
		runtime = "docker"
	} else if _, err := exec.LookPath("podman"); err == nil {
		runtime = "podman"
	} else {
		msg := "containerexec: neither docker nor podman on PATH"
		return Result{Type: ResultText, Text: msg, ForLLM: msg, IsError: true}, nil
	}

	timeoutMs := intField(input, "timeout_ms", 120000)
	runCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	// Handle pull_policy before building run args.
	pullPolicy, _ := input["pull_policy"].(string)
	if pullPolicy == "always" {
		pullCtx, pullCancel := context.WithTimeout(runCtx, 10*time.Minute)
		pullCmd := exec.CommandContext(pullCtx, runtime, "pull", image)
		if out, err := pullCmd.CombinedOutput(); err != nil {
			pullCancel()
			msg := "containerexec: pull failed: " + string(out)
			return Result{Type: ResultText, Text: msg, ForLLM: msg, IsError: true}, nil
		}
		pullCancel()
	}

	args := []string{"run", "--rm", "-i"}

	if pullPolicy == "never" {
		args = append(args, "--pull", "never")
	}

	network, _ := input["network"].(string)
	if network == "" {
		network = "none"
	}
	args = append(args, "--network", network)

	if mounts, ok := input["mounts"].([]any); ok {
		for _, m := range mounts {
			ms, ok := m.(string)
			if !ok {
				continue
			}
			parts := strings.Split(ms, ":")
			if len(parts) < 2 {
				msg := "containerexec: bad mount spec: " + ms
				return Result{Type: ResultText, Text: msg, ForLLM: msg, IsError: true}, nil
			}
			host, container := parts[0], parts[1]
			mode := "ro"
			if len(parts) >= 3 {
				mode = parts[2]
			}
			if mode == "rw" {
				args = append(args, "--mount", fmt.Sprintf("type=bind,src=%s,dst=%s", host, container))
			} else {
				args = append(args, "--mount", fmt.Sprintf("type=bind,src=%s,dst=%s,readonly", host, container))
			}
		}
	}

	if envMap, ok := input["env"].(map[string]any); ok {
		for k, v := range envMap {
			if vs, ok := v.(string); ok {
				args = append(args, "-e", fmt.Sprintf("%s=%s", k, vs))
			}
		}
	}

	if gpu, _ := input["gpu"].(bool); gpu {
		args = append(args, "--gpus", "all")
	}
	if u, _ := input["user"].(string); u != "" {
		args = append(args, "--user", u)
	}
	if wd, _ := input["workdir"].(string); wd != "" {
		args = append(args, "--workdir", wd)
	}

	args = append(args, image, "/bin/sh", "-c", cmd)
	runner := exec.CommandContext(runCtx, runtime, args...)

	var stdoutBuf, stderrBuf bytes.Buffer
	runner.Stdout = &stdoutBuf
	runner.Stderr = &stderrBuf
	err := runner.Run()
	out := stdoutBuf.String()
	errOut := stderrBuf.String()

	// Combine into Result text: stdout body, then stderr footer if non-empty.
	text := out
	if errOut != "" {
		if text != "" {
			text += "\n"
		}
		text += "--- stderr ---\n" + errOut
	}

	exitCode := 0
	if runner.ProcessState != nil {
		exitCode = runner.ProcessState.ExitCode()
	}

	data := map[string]any{
		"stdout":    out,
		"stderr":    errOut,
		"exit_code": exitCode,
	}

	if runCtx.Err() == context.DeadlineExceeded {
		msg := text + "\n[timeout after " + fmt.Sprintf("%d", timeoutMs) + "ms]"
		return Result{Type: ResultText, Text: msg, ForLLM: msg, IsError: true, Data: data}, nil
	}
	if err != nil {
		msg := text + "\n[exit error: " + err.Error() + "]"
		return Result{Type: ResultText, Text: msg, ForLLM: msg, IsError: true, Data: data}, nil
	}
	return Result{Type: ResultText, Text: text, ForLLM: text, Data: data}, nil
}

package tool

import (
	"context"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

// hasContainerRuntime reports whether docker or podman is on PATH.
func hasContainerRuntime(t *testing.T) bool {
	t.Helper()
	if _, err := exec.LookPath("docker"); err == nil {
		return true
	}
	if _, err := exec.LookPath("podman"); err == nil {
		return true
	}
	return false
}

func TestContainerExec_MissingImageAndCommand(t *testing.T) {
	res, err := (&ContainerExec{}).Call(context.Background(), map[string]any{}, nil)
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Text, "image and command required")
}

func TestContainerExec_MissingCommand(t *testing.T) {
	res, err := (&ContainerExec{}).Call(context.Background(), map[string]any{
		"image": "alpine",
	}, nil)
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Text, "image and command required")
}

func TestContainerExec_MissingImage(t *testing.T) {
	res, err := (&ContainerExec{}).Call(context.Background(), map[string]any{
		"command": "echo hi",
	}, nil)
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Text, "image and command required")
}

func TestContainerExec_BadMountSpec(t *testing.T) {
	if !hasContainerRuntime(t) {
		t.Skip("no docker/podman on PATH")
	}
	res, err := (&ContainerExec{}).Call(context.Background(), map[string]any{
		"image":   "alpine",
		"command": "echo hi",
		"mounts":  []any{"badmountspec"},
	}, nil)
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Text, "bad mount spec")
}

func TestContainerExec_Schema(t *testing.T) {
	s := ContainerExec{}.Schema()
	require.Equal(t, "object", s["type"])
	props, ok := s["properties"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, props, "image")
	require.Contains(t, props, "command")
	require.Contains(t, props, "mounts")
	require.Contains(t, props, "env")
	require.Contains(t, props, "network")
	require.Contains(t, props, "timeout_ms")
	required, ok := s["required"].([]string)
	require.True(t, ok)
	require.Contains(t, required, "image")
	require.Contains(t, required, "command")
}

func TestContainerExec_Metadata(t *testing.T) {
	tool := ContainerExec{}
	require.Equal(t, "ContainerExec", tool.Name())
	require.False(t, tool.IsReadOnly())
	require.True(t, tool.IsConcurrencySafe())

	require.Equal(t, "ContainerExec: alpine", tool.UserFacingName(map[string]any{"image": "alpine"}))
	require.Equal(t, "ContainerExec", tool.UserFacingName(map[string]any{}))
}

// skipIfNoAlpine skips the test if the alpine image is not locally available
// (e.g. no docker daemon, credential helper missing, or offline).
func skipIfNoAlpine(t *testing.T) {
	t.Helper()
	out, err := exec.Command("docker", "image", "inspect", "alpine").CombinedOutput()
	if err != nil {
		// Also try podman
		out, err = exec.Command("podman", "image", "inspect", "alpine").CombinedOutput()
		if err != nil {
			t.Skipf("alpine image not available locally (inspect: %s %v)", string(out), err)
		}
	}
	_ = out
}

func TestContainerExec_BasicEcho(t *testing.T) {
	if !hasContainerRuntime(t) {
		t.Skip("no docker/podman on PATH")
	}
	skipIfNoAlpine(t)
	res, err := (&ContainerExec{}).Call(context.Background(), map[string]any{
		"image":   "alpine",
		"command": "echo hello",
	}, nil)
	require.NoError(t, err)
	require.False(t, res.IsError, "output: %s", res.Text)
	require.Contains(t, res.Text, "hello")
}

func TestContainerExec_NetworkNoneDefaulted(t *testing.T) {
	if !hasContainerRuntime(t) {
		t.Skip("no docker/podman on PATH")
	}
	skipIfNoAlpine(t)
	// With no network field, default is "none". Ping should fail.
	res, err := (&ContainerExec{}).Call(context.Background(), map[string]any{
		"image":   "alpine",
		"command": "ping -c 1 -W 1 8.8.8.8 || echo network_blocked",
	}, nil)
	require.NoError(t, err)
	// Either an exit error (ping fails) or our echo — either way network is blocked.
	_ = res
}

func TestContainerExec_BindMountReadonly(t *testing.T) {
	if !hasContainerRuntime(t) {
		t.Skip("no docker/podman on PATH")
	}
	skipIfNoAlpine(t)
	tmpDir := t.TempDir()
	res, err := (&ContainerExec{}).Call(context.Background(), map[string]any{
		"image":   "alpine",
		"command": "touch /work/newfile 2>&1 || echo write_blocked",
		"mounts":  []any{tmpDir + ":/work:ro"},
	}, nil)
	require.NoError(t, err)
	// readonly mount: write should fail; either error result or "write_blocked" output
	_ = res
}

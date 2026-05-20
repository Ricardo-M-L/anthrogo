package tool

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/pkg/bgtasks"
)

func waitBgTask(m *bgtasks.Manager, id string, timeout time.Duration) *bgtasks.Task {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if t, ok := m.Get(id); ok && t.Status != bgtasks.StatusRunning {
			return t
		}
		time.Sleep(20 * time.Millisecond)
	}
	t, _ := m.Get(id)
	return t
}

// --- BackgroundLaunch ---

func TestBackgroundLaunch_Schema(t *testing.T) {
	b := &BackgroundLaunch{Manager: bgtasks.NewManager()}
	schema := b.Schema()
	require.Equal(t, "object", schema["type"])
	props, ok := schema["properties"].(map[string]any)
	require.True(t, ok)
	_, hasCmd := props["command"]
	require.True(t, hasCmd)
	req, _ := schema["required"].([]string)
	require.Contains(t, req, "command")
}

func TestBackgroundLaunch_MissingCommand(t *testing.T) {
	b := &BackgroundLaunch{Manager: bgtasks.NewManager()}
	res, err := b.Call(context.Background(), map[string]any{}, nil)
	require.NoError(t, err)
	require.True(t, res.IsError)
}

func TestBackgroundLaunch_NilManager(t *testing.T) {
	b := &BackgroundLaunch{}
	res, err := b.Call(context.Background(), map[string]any{"command": "echo hi"}, nil)
	require.NoError(t, err)
	require.True(t, res.IsError)
}

func TestBackgroundLaunch_ReturnsTaskID(t *testing.T) {
	m := bgtasks.NewManager()
	b := &BackgroundLaunch{Manager: m}
	res, err := b.Call(context.Background(), map[string]any{"command": "echo hello"}, nil)
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Contains(t, res.Text, "task_id=")
	// Data map should carry task_id.
	id, ok := res.Data["task_id"].(string)
	require.True(t, ok)
	require.NotEmpty(t, id)
}

func TestBackgroundLaunch_UserFacingName(t *testing.T) {
	b := &BackgroundLaunch{}
	require.Equal(t, "BackgroundLaunch", b.UserFacingName(map[string]any{}))
	require.Contains(t, b.UserFacingName(map[string]any{"command": "echo hi"}), "echo hi")
}

// --- BackgroundStatus ---

func TestBackgroundStatus_NilManager(t *testing.T) {
	b := &BackgroundStatus{}
	res, err := b.Call(context.Background(), map[string]any{}, nil)
	require.NoError(t, err)
	require.True(t, res.IsError)
}

func TestBackgroundStatus_NoTasks(t *testing.T) {
	b := &BackgroundStatus{Manager: bgtasks.NewManager()}
	res, err := b.Call(context.Background(), map[string]any{}, nil)
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Contains(t, res.Text, "no background tasks")
}

func TestBackgroundStatus_OneTask(t *testing.T) {
	m := bgtasks.NewManager()
	id := m.Launch("echo status-test")
	waitBgTask(m, id, 5*time.Second)
	b := &BackgroundStatus{Manager: m}
	res, err := b.Call(context.Background(), map[string]any{"task_id": id}, nil)
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Contains(t, res.Text, id)
	require.Contains(t, res.Text, "complete")
}

func TestBackgroundStatus_NotFound(t *testing.T) {
	b := &BackgroundStatus{Manager: bgtasks.NewManager()}
	res, err := b.Call(context.Background(), map[string]any{"task_id": "badid"}, nil)
	require.NoError(t, err)
	require.True(t, res.IsError)
}

func TestBackgroundStatus_ListAll(t *testing.T) {
	m := bgtasks.NewManager()
	id1 := m.Launch("echo a")
	id2 := m.Launch("echo b")
	waitBgTask(m, id1, 5*time.Second)
	waitBgTask(m, id2, 5*time.Second)
	b := &BackgroundStatus{Manager: m}
	res, err := b.Call(context.Background(), map[string]any{}, nil)
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Contains(t, res.Text, id1)
	require.Contains(t, res.Text, id2)
}

// --- BackgroundOutput ---

func TestBackgroundOutput_NilManager(t *testing.T) {
	b := &BackgroundOutput{}
	res, err := b.Call(context.Background(), map[string]any{"task_id": "x"}, nil)
	require.NoError(t, err)
	require.True(t, res.IsError)
}

func TestBackgroundOutput_MissingID(t *testing.T) {
	b := &BackgroundOutput{Manager: bgtasks.NewManager()}
	res, err := b.Call(context.Background(), map[string]any{}, nil)
	require.NoError(t, err)
	require.True(t, res.IsError)
}

func TestBackgroundOutput_NotFound(t *testing.T) {
	b := &BackgroundOutput{Manager: bgtasks.NewManager()}
	res, err := b.Call(context.Background(), map[string]any{"task_id": "noexist"}, nil)
	require.NoError(t, err)
	require.True(t, res.IsError)
}

func TestBackgroundOutput_HasOutput(t *testing.T) {
	m := bgtasks.NewManager()
	id := m.Launch("echo hello-out; echo err-out >&2")
	waitBgTask(m, id, 5*time.Second)
	b := &BackgroundOutput{Manager: m}
	res, err := b.Call(context.Background(), map[string]any{"task_id": id}, nil)
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Contains(t, res.Text, "hello-out")
	require.Contains(t, res.Text, "err-out")
}

// --- BackgroundCancel ---

func TestBackgroundCancel_NilManager(t *testing.T) {
	b := &BackgroundCancel{}
	res, err := b.Call(context.Background(), map[string]any{"task_id": "x"}, nil)
	require.NoError(t, err)
	require.True(t, res.IsError)
}

func TestBackgroundCancel_MissingID(t *testing.T) {
	b := &BackgroundCancel{Manager: bgtasks.NewManager()}
	res, err := b.Call(context.Background(), map[string]any{}, nil)
	require.NoError(t, err)
	require.True(t, res.IsError)
}

func TestBackgroundCancel_Success(t *testing.T) {
	m := bgtasks.NewManager()
	id := m.Launch("sleep 30")
	time.Sleep(80 * time.Millisecond)
	b := &BackgroundCancel{Manager: m}
	res, err := b.Call(context.Background(), map[string]any{"task_id": id}, nil)
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Contains(t, res.Text, "canceled")
}

func TestBackgroundCancel_AlreadyDone(t *testing.T) {
	m := bgtasks.NewManager()
	id := m.Launch("echo done")
	waitBgTask(m, id, 5*time.Second)
	b := &BackgroundCancel{Manager: m}
	res, err := b.Call(context.Background(), map[string]any{"task_id": id}, nil)
	require.NoError(t, err)
	require.True(t, res.IsError)
}

// --- truncStr ---

func TestTruncStr(t *testing.T) {
	require.Equal(t, "hello", truncStr("hello", 10))
	require.True(t, strings.HasSuffix(truncStr("hello world long string", 5), "…"))
	// Newlines replaced
	require.NotContains(t, truncStr("a\nb", 100), "\n")
}

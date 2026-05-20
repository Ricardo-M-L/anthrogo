package bgtasks

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func waitFor(m *Manager, id string, timeout time.Duration) *Task {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if t, ok := m.Get(id); ok && t.Status != StatusRunning {
			return t
		}
		time.Sleep(20 * time.Millisecond)
	}
	t, _ := m.Get(id)
	return t
}

func TestManager_LaunchAndComplete(t *testing.T) {
	m := NewManager()
	id := m.Launch("echo hi")
	task := waitFor(m, id, 5*time.Second)
	require.NotNil(t, task)
	require.Equal(t, StatusComplete, task.Status)
	require.Equal(t, 0, task.ExitCode)
	require.Contains(t, task.Stdout.String(), "hi")
}

func TestManager_LaunchFailed(t *testing.T) {
	m := NewManager()
	id := m.Launch("exit 42")
	task := waitFor(m, id, 5*time.Second)
	require.NotNil(t, task)
	require.Equal(t, StatusFailed, task.Status)
	require.Equal(t, 42, task.ExitCode)
}

func TestManager_Cancel(t *testing.T) {
	m := NewManager()
	id := m.Launch("sleep 30")
	time.Sleep(100 * time.Millisecond) // let it start
	require.NoError(t, m.Cancel(id))
	task := waitFor(m, id, 2*time.Second)
	require.NotNil(t, task)
	require.Equal(t, StatusCanceled, task.Status)
}

func TestManager_CancelAlreadyDone(t *testing.T) {
	m := NewManager()
	id := m.Launch("echo done")
	waitFor(m, id, 5*time.Second)
	err := m.Cancel(id)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not running")
}

func TestManager_GetNonexistent(t *testing.T) {
	m := NewManager()
	_, ok := m.Get("notavalidid")
	require.False(t, ok)
}

func TestManager_ListEmpty(t *testing.T) {
	m := NewManager()
	ids := m.List()
	require.Empty(t, ids)
}

func TestManager_ListMultiple(t *testing.T) {
	m := NewManager()
	id1 := m.Launch("echo a")
	id2 := m.Launch("echo b")
	ids := m.List()
	require.Len(t, ids, 2)
	require.Contains(t, ids, id1)
	require.Contains(t, ids, id2)
}

func TestManager_StdoutStderr(t *testing.T) {
	m := NewManager()
	id := m.Launch("echo stdout; echo stderr >&2")
	task := waitFor(m, id, 5*time.Second)
	require.Equal(t, StatusComplete, task.Status)
	require.Contains(t, task.Stdout.String(), "stdout")
	require.Contains(t, task.Stderr.String(), "stderr")
}

func TestManager_SnapshotIsolation(t *testing.T) {
	// Two Get() calls should return independent buffer copies.
	m := NewManager()
	id := m.Launch("echo snapshot")
	task1 := waitFor(m, id, 5*time.Second)
	task2, ok := m.Get(id)
	require.True(t, ok)
	require.Equal(t, task1.Stdout.String(), task2.Stdout.String())
	// Mutating one should not affect the other.
	task1.Stdout.Reset()
	require.NotEqual(t, task1.Stdout.String(), task2.Stdout.String())
}

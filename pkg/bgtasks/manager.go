package bgtasks

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"
)

// Status represents the lifecycle state of a background task.
type Status string

const (
	StatusRunning  Status = "running"
	StatusComplete Status = "complete"
	StatusFailed   Status = "failed"
	StatusCanceled Status = "canceled"
)

// lockedBuffer is a bytes.Buffer protected by a mutex so that exec.Cmd's
// internal copy goroutines and Manager.Get() can access it concurrently.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (lb *lockedBuffer) Write(p []byte) (int, error) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	return lb.buf.Write(p)
}

// snapshot returns a copy of the current contents.
func (lb *lockedBuffer) snapshot() []byte {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	return append([]byte(nil), lb.buf.Bytes()...)
}

// taskSnapshot is the immutable view returned by Manager.Get().
type taskSnapshot struct {
	ID         string
	Command    string
	Status     Status
	Stdout     bytes.Buffer
	Stderr     bytes.Buffer
	ExitCode   int
	StartedAt  time.Time
	FinishedAt time.Time
}

// Task holds the runtime state of one background task.
type Task struct {
	ID         string
	Command    string
	Status     Status
	Stdout     bytes.Buffer // populated from snapshot; not used for live writes
	Stderr     bytes.Buffer
	ExitCode   int
	StartedAt  time.Time
	FinishedAt time.Time

	stdout lockedBuffer
	stderr lockedBuffer
	cmd    *exec.Cmd
	cancel context.CancelFunc
}

// maxRetainedTasks caps the in-memory task history per Manager. When a Launch
// would push the count over the cap, the oldest finished task is evicted
// (running tasks are never evicted).
const maxRetainedTasks = 256

// Manager is an in-process registry of background tasks.
type Manager struct {
	mu    sync.Mutex
	tasks map[string]*Task
}

// NewManager returns a ready-to-use Manager.
func NewManager() *Manager {
	return &Manager{tasks: map[string]*Task{}}
}

// Launch starts a new background task. The returned id is unique per Manager.
// The command runs via `sh -c <command>`.
func (m *Manager) Launch(command string) string {
	id := generateID()
	ctx, cancel := context.WithCancel(context.Background())
	task := &Task{
		ID:        id,
		Command:   command,
		Status:    StatusRunning,
		StartedAt: time.Now(),
		cancel:    cancel,
	}
	cmd := exec.CommandContext(ctx, "sh", "-c", command)
	// Point exec to the locked buffers so the OS copy goroutines hold the per-buffer lock.
	cmd.Stdout = &task.stdout
	cmd.Stderr = &task.stderr
	task.cmd = cmd

	m.mu.Lock()
	m.evictFinishedLocked()
	m.tasks[id] = task
	m.mu.Unlock()

	go func() {
		err := cmd.Run()
		m.mu.Lock()
		defer m.mu.Unlock()
		task.FinishedAt = time.Now()
		if ctx.Err() == context.Canceled {
			task.Status = StatusCanceled
		} else if err != nil {
			task.Status = StatusFailed
			if exitErr, ok := err.(*exec.ExitError); ok {
				task.ExitCode = exitErr.ExitCode()
			} else {
				task.ExitCode = -1
			}
		} else {
			task.Status = StatusComplete
			task.ExitCode = 0
		}
	}()

	return id
}

// Get returns a snapshot of the task with deep-copied output buffers.
// Safe to call while the task is still running.
func (m *Manager) Get(id string) (*Task, bool) {
	m.mu.Lock()
	t, ok := m.tasks[id]
	if !ok {
		m.mu.Unlock()
		return nil, false
	}
	// Copy fields protected by the manager lock.
	cp := &Task{
		ID:         t.ID,
		Command:    t.Command,
		Status:     t.Status,
		ExitCode:   t.ExitCode,
		StartedAt:  t.StartedAt,
		FinishedAt: t.FinishedAt,
	}
	m.mu.Unlock()

	// Snapshot output buffers under their own locks (independent of m.mu).
	cp.Stdout = *bytes.NewBuffer(t.stdout.snapshot())
	cp.Stderr = *bytes.NewBuffer(t.stderr.snapshot())
	return cp, true
}

// Cancel signals the running task to stop. No-op if already finished.
func (m *Manager) Cancel(id string) error {
	m.mu.Lock()
	t, ok := m.tasks[id]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("no such task: %s", id)
	}
	if t.Status != StatusRunning {
		return fmt.Errorf("task %s is not running (status=%s)", id, t.Status)
	}
	if t.cancel != nil {
		t.cancel()
	}
	return nil
}

// List returns all task IDs in arbitrary order.
func (m *Manager) List() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, 0, len(m.tasks))
	for id := range m.tasks {
		out = append(out, id)
	}
	return out
}

// generateID produces a 12-char hex string; sufficient for a single anthrogo process.
func generateID() string {
	var b [6]byte
	_, _ = io.ReadFull(rand.Reader, b[:])
	return hex.EncodeToString(b[:])
}

// evictFinishedLocked removes the oldest-finished tasks beyond
// maxRetainedTasks. Running tasks are never evicted. Caller must hold m.mu.
func (m *Manager) evictFinishedLocked() {
	if len(m.tasks) < maxRetainedTasks {
		return
	}
	type byTime struct {
		id  string
		end time.Time
	}
	finished := make([]byTime, 0, len(m.tasks))
	for id, t := range m.tasks {
		if t.Status != StatusRunning {
			finished = append(finished, byTime{id, t.FinishedAt})
		}
	}
	// Sort ascending by FinishedAt — oldest first.
	for i := 1; i < len(finished); i++ {
		for j := i; j > 0 && finished[j].end.Before(finished[j-1].end); j-- {
			finished[j], finished[j-1] = finished[j-1], finished[j]
		}
	}
	// Evict enough to bring count strictly below the cap.
	excess := len(m.tasks) - (maxRetainedTasks - 1)
	if excess > len(finished) {
		excess = len(finished)
	}
	for i := 0; i < excess; i++ {
		delete(m.tasks, finished[i].id)
	}
}

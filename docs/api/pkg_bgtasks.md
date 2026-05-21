# `github.com/ricardo/anthrogo/pkg/bgtasks`

```go
package bgtasks // import "github.com/ricardo/anthrogo/pkg/bgtasks"


TYPES

type Manager struct {
	// Has unexported fields.
}
    Manager is an in-process registry of background tasks.

func NewManager() *Manager
    NewManager returns a ready-to-use Manager.

func (m *Manager) Cancel(id string) error
    Cancel signals the running task to stop. No-op if already finished.

func (m *Manager) Get(id string) (*Task, bool)
    Get returns a snapshot of the task with deep-copied output buffers. Safe to
    call while the task is still running.

func (m *Manager) Launch(command string) string
    Launch starts a new background task. The returned id is unique per Manager.
    The command runs via `sh -c <command>`.

func (m *Manager) List() []string
    List returns all task IDs in arbitrary order.

type Status string
    Status represents the lifecycle state of a background task.

const (
	StatusRunning  Status = "running"
	StatusComplete Status = "complete"
	StatusFailed   Status = "failed"
	StatusCanceled Status = "canceled"
)
type Task struct {
	ID         string
	Command    string
	Status     Status
	Stdout     bytes.Buffer // populated from snapshot; not used for live writes
	Stderr     bytes.Buffer
	ExitCode   int
	StartedAt  time.Time
	FinishedAt time.Time

	// Has unexported fields.
}
    Task holds the runtime state of one background task.

```

# `github.com/ricardo/anthrogo/pkg/telemetry`

```go
package telemetry // import "github.com/ricardo/anthrogo/pkg/telemetry"


TYPES

type Reporter struct {
	// Has unexported fields.
}
    Reporter buffers events and flushes periodically (or on Close). All data is
    opt-in; disabled by default.

func NewReporter(enabled bool, endpoint string, cacheDir string) *Reporter
    NewReporter constructs a Reporter. When enabled is false all methods
    are no-ops and no goroutine is started. cacheDir is used to persist the
    per-install machine ID.

func (r *Reporter) Close()
    Close flushes remaining events synchronously and stops the background
    goroutine.

func (r *Reporter) Endpoint() string
    Endpoint returns the configured flush endpoint.

func (r *Reporter) Event(kind string, data map[string]any)
    Event records a single named event. It is a no-op when the reporter is
    disabled. Sensitive keys are stripped automatically before buffering.

func (r *Reporter) IsEnabled() bool
    IsEnabled reports whether telemetry is active.

```

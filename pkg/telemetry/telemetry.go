package telemetry

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// sensitiveKeys is the set of field names that must never be sent in an event.
var sensitiveKeys = map[string]bool{
	"path":     true,
	"command":  true,
	"prompt":   true,
	"text":     true,
	"cwd":      true,
	"input":    true,
	"args":     true,
	"stdout":   true,
	"stderr":   true,
	"url":      true,
	"endpoint": true,
	"api_key":  true,
	"token":    true,
	"secret":   true,
}

// sanitizeEvent returns a copy of data with all sensitiveKeys removed.
func sanitizeEvent(data map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range data {
		if !sensitiveKeys[k] {
			out[k] = v
		}
	}
	return out
}

// Reporter buffers events and flushes periodically (or on Close).
// All data is opt-in; disabled by default.
type Reporter struct {
	enabled   bool
	endpoint  string
	machineID string

	mu     sync.Mutex
	events []map[string]any

	stop chan struct{}
	done chan struct{}
}

// NewReporter constructs a Reporter. When enabled is false all methods are
// no-ops and no goroutine is started. cacheDir is used to persist the
// per-install machine ID.
func NewReporter(enabled bool, endpoint string, cacheDir string) *Reporter {
	r := &Reporter{
		enabled:  enabled,
		endpoint: endpoint,
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	if enabled {
		r.machineID = loadOrCreateMachineID(cacheDir)
		go r.flushLoop()
	} else {
		close(r.done)
	}
	return r
}

// IsEnabled reports whether telemetry is active.
func (r *Reporter) IsEnabled() bool {
	return r != nil && r.enabled
}

// Endpoint returns the configured flush endpoint.
func (r *Reporter) Endpoint() string {
	if r == nil {
		return ""
	}
	return r.endpoint
}

func loadOrCreateMachineID(cacheDir string) string {
	path := filepath.Join(cacheDir, "machine_id")
	if raw, err := os.ReadFile(path); err == nil {
		return string(raw)
	}
	var b [8]byte
	_, _ = io.ReadFull(rand.Reader, b[:])
	id := hex.EncodeToString(b[:])
	_ = os.MkdirAll(cacheDir, 0o755)
	_ = os.WriteFile(path, []byte(id), 0o600)
	return id
}

// Event records a single named event. It is a no-op when the reporter is
// disabled. Sensitive keys are stripped automatically before buffering.
func (r *Reporter) Event(kind string, data map[string]any) {
	if r == nil || !r.enabled {
		return
	}
	ev := map[string]any{
		"kind":     kind,
		"ts":       time.Now().Unix(),
		"anthrogo": os.Getenv("ANTHROGO_VERSION_OVERRIDE"),
		"go_os":    runtime.GOOS,
		"go_arch":  runtime.GOARCH,
		"machine":  r.machineID,
	}
	for k, v := range sanitizeEvent(data) {
		ev[k] = v
	}
	r.mu.Lock()
	r.events = append(r.events, ev)
	r.mu.Unlock()
}

func (r *Reporter) flushLoop() {
	defer close(r.done)
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			r.flush()
		case <-r.stop:
			r.flush()
			return
		}
	}
}

func (r *Reporter) flush() {
	if r == nil || !r.enabled {
		return
	}
	r.mu.Lock()
	if len(r.events) == 0 {
		r.mu.Unlock()
		return
	}
	batch := r.events
	r.events = nil
	r.mu.Unlock()

	if r.endpoint == "" {
		return
	}

	body, err := json.Marshal(map[string]any{"events": batch})
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "POST", r.endpoint, bytes.NewReader(body))
	if err != nil {
		// Re-queue on failure.
		r.mu.Lock()
		r.events = append(batch, r.events...)
		r.mu.Unlock()
		return
	}
	req.Header.Set("Content-Type", "application/json")
	// Dedicated client with both context AND transport-level timeout so a
	// stalled DNS / TLS handshake to an unreachable endpoint can't park
	// Close() forever during process exit.
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		// Silent failure — re-queue to retry next flush.
		r.mu.Lock()
		r.events = append(batch, r.events...)
		r.mu.Unlock()
		return
	}
	resp.Body.Close()
}

// Close flushes remaining events synchronously and stops the background goroutine.
func (r *Reporter) Close() {
	if r == nil || !r.enabled {
		return
	}
	close(r.stop)
	<-r.done
}

package fake

import (
	"context"
	"errors"
	"sync"

	"github.com/ricardo/anthrogo/pkg/provider"
)

// Provider replays scripted Event sequences, one per Stream() call.
// LastModel records the Model field of the most recent Stream() call; safe to read after drain.
type Provider struct {
	mu        sync.Mutex
	scripts   [][]provider.Event
	cursor    int
	LastModel string

	// StreamErrors, when non-nil, injects an EventError into the channel at
	// the start of the corresponding Stream() call (before any scripted events).
	// Index i is consumed for the i-th call. A nil entry means no injected error
	// for that call. After StreamErrors is exhausted, no errors are injected.
	StreamErrors []error
	errCursor    int
}

func New(scripts ...[]provider.Event) *Provider {
	return &Provider{scripts: scripts}
}

func (p *Provider) Stream(ctx context.Context, req provider.Request) (<-chan provider.Event, error) {
	p.mu.Lock()
	if p.cursor >= len(p.scripts) {
		p.mu.Unlock()
		return nil, errors.New("fake: no more scripts")
	}
	script := p.scripts[p.cursor]
	p.cursor++
	p.LastModel = req.Model

	// Check for a scripted stream error to inject.
	var injectErr error
	if p.errCursor < len(p.StreamErrors) {
		injectErr = p.StreamErrors[p.errCursor]
		p.errCursor++
	}
	p.mu.Unlock()

	out := make(chan provider.Event, len(script)+1)
	go func() {
		defer close(out)
		// Inject error first if configured.
		if injectErr != nil {
			select {
			case <-ctx.Done():
				return
			case out <- provider.Event{Kind: provider.EventError, Err: injectErr}:
			}
			return
		}
		for _, ev := range script {
			select {
			case <-ctx.Done():
				return
			case out <- ev:
			}
		}
	}()
	return out, nil
}

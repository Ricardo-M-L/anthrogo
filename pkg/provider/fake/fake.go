package fake

import (
	"context"
	"errors"
	"sync"

	"github.com/ricardo/anthrogo/pkg/provider"
)

// Provider replays scripted Event sequences, one per Stream() call.
type Provider struct {
	mu      sync.Mutex
	scripts [][]provider.Event
	cursor  int
}

func New(scripts ...[]provider.Event) *Provider {
	return &Provider{scripts: scripts}
}

func (p *Provider) Stream(ctx context.Context, _ provider.Request) (<-chan provider.Event, error) {
	p.mu.Lock()
	if p.cursor >= len(p.scripts) {
		p.mu.Unlock()
		return nil, errors.New("fake: no more scripts")
	}
	script := p.scripts[p.cursor]
	p.cursor++
	p.mu.Unlock()

	out := make(chan provider.Event, len(script))
	go func() {
		defer close(out)
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

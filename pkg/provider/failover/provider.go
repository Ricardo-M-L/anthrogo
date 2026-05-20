package failover

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ricardo/anthrogo/pkg/provider"
)

// Provider tries each backend in order; on EventError before any text/tool_use
// arrives, it switches to the next. If text/tool/usage already streamed when
// the error fires, the error is forwarded (partial stream can't be retried).
type Provider struct {
	Backends []provider.Provider
	Names    []string // for logging
	Logger   *slog.Logger
}

// New creates a failover Provider with the given backends and names.
func New(backends []provider.Provider, names []string) *Provider {
	return &Provider{Backends: backends, Names: names}
}

// Stream tries each backend in order. On EventError before any committed event
// (text_delta, tool_use_start, etc.), it switches to the next backend. If a
// committed event has already been emitted or no more backends remain, the
// error is forwarded as-is.
func (p *Provider) Stream(ctx context.Context, req provider.Request) (<-chan provider.Event, error) {
	if len(p.Backends) == 0 {
		return nil, fmt.Errorf("failover: no backends configured")
	}

	out := make(chan provider.Event, 64)
	go func() {
		defer close(out)
		for i, backend := range p.Backends {
			name := ""
			if i < len(p.Names) {
				name = p.Names[i]
			}
			ch, err := backend.Stream(ctx, req)
			if err != nil {
				if p.Logger != nil {
					p.Logger.Warn("failover backend start error", "backend", name, "err", err)
				}
				if i < len(p.Backends)-1 {
					continue
				}
				out <- provider.Event{Kind: provider.EventError, Err: fmt.Errorf("failover: all backends failed; last: %w", err)}
				return
			}

			// Track whether we've emitted any "committed" event (text/tool_use/usage).
			// If yes and error fires later, can't retry — pass the error through.
			committed := false
			switchToNext := false
			for ev := range ch {
				switch ev.Kind {
				case provider.EventTextDelta, provider.EventToolUseStart, provider.EventToolInputDelta,
					provider.EventBlockStop, provider.EventMessageStop, provider.EventUsage,
					provider.EventStart:
					committed = true
				case provider.EventError:
					if !committed && i < len(p.Backends)-1 {
						if p.Logger != nil {
							p.Logger.Warn("failover", "backend", name, "switching", true, "err", ev.Err)
						}
						switchToNext = true
						// drain remaining channel events without forwarding
						go func() {
							for range ch {
							}
						}()
						goto nextBackend
					}
					// committed or last backend → forward error and stop
					out <- ev
					return
				}
				out <- ev
			}
			if !switchToNext {
				return // stream ended cleanly
			}
		nextBackend:
		}
	}()
	return out, nil
}

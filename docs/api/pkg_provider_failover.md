# `github.com/ricardo/anthrogo/pkg/provider/failover`

```go
package failover // import "github.com/ricardo/anthrogo/pkg/provider/failover"


TYPES

type Provider struct {
	Backends []provider.Provider
	Names    []string // for logging
	Logger   *slog.Logger
}
    Provider tries each backend in order; on EventError before any text/tool_use
    arrives, it switches to the next. If text/tool/usage already streamed when
    the error fires, the error is forwarded (partial stream can't be retried).

func New(backends []provider.Provider, names []string) *Provider
    New creates a failover Provider with the given backends and names.

func (p *Provider) Stream(ctx context.Context, req provider.Request) (<-chan provider.Event, error)
    Stream tries each backend in order. On EventError before any committed
    event (text_delta, tool_use_start, etc.), it switches to the next backend.
    If a committed event has already been emitted or no more backends remain,
    the error is forwarded as-is.

```

# `github.com/ricardo/anthrogo/pkg/provider/fake`

```go
package fake // import "github.com/ricardo/anthrogo/pkg/provider/fake"


TYPES

type Provider struct {
	// Has unexported fields.
}
    Provider replays scripted Event sequences, one per Stream() call.

func New(scripts ...[]provider.Event) *Provider

func (p *Provider) Stream(ctx context.Context, _ provider.Request) (<-chan provider.Event, error)

```

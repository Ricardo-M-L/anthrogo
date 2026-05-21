# `github.com/ricardo/anthrogo/pkg/provider/anthropic`

```go
package anthropic // import "github.com/ricardo/anthrogo/pkg/provider/anthropic"


TYPES

type Provider struct {
	// Has unexported fields.
}
    Provider wraps anthropic-sdk-go and implements provider.Provider.

func New(apiKey, model string) *Provider
    New constructs a Provider using ANTHROPIC_API_KEY when apiKey == "".

func NewWithOptions(model string, opts ...option.RequestOption) *Provider
    NewWithOptions constructs a Provider with arbitrary SDK request options.
    Used by bedrock, Vertex and other backends that provide their own auth.

func (p *Provider) Model() string
    Model returns the default model name.

func (p *Provider) Stream(ctx context.Context, req provider.Request) (<-chan provider.Event, error)
    Stream implements provider.Provider.

```

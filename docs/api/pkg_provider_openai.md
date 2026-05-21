# `github.com/ricardo/anthrogo/pkg/provider/openai`

```go
package openai // import "github.com/ricardo/anthrogo/pkg/provider/openai"


TYPES

type Provider struct {
	BaseURL string
	APIKey  string
	// Client is the HTTP client used for requests. When nil, a default client
	// with a 5-minute timeout is used.
	Client *http.Client
}
    Provider implements provider.Provider for any OpenAI Chat Completions
    compatible endpoint (DeepSeek, Kimi, MiniMax, GLM, vllm, ollama, etc.).

func New(baseURL, apiKey string) *Provider
    New constructs a Provider.

func (p *Provider) Stream(ctx context.Context, req provider.Request) (<-chan provider.Event, error)
    Stream implements provider.Provider.

```

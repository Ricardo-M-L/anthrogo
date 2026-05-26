package anthropic

import (
	"os"

	sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// Provider wraps anthropic-sdk-go and implements provider.Provider.
type Provider struct {
	client sdk.Client // v1 SDK returns Client by value, not *Client
	model  string
}

// New constructs a Provider using ANTHROPIC_API_KEY when apiKey == "".
func New(apiKey, model string) *Provider {
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	return NewWithOptions(model, option.WithAPIKey(apiKey))
}

// NewWithOptions constructs a Provider with arbitrary SDK request options.
// Used by bedrock, Vertex and other backends that provide their own auth.
func NewWithOptions(model string, opts ...option.RequestOption) *Provider {
	return &Provider{client: sdk.NewClient(opts...), model: model}
}

// Model returns the default model name.
func (p *Provider) Model() string { return p.model }

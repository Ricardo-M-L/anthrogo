package anthropic

import (
	"os"

	sdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// Provider wraps anthropic-sdk-go and implements provider.Provider.
type Provider struct {
	client *sdk.Client
	model  string
}

// New constructs a Provider using ANTHROPIC_API_KEY when apiKey == "".
func New(apiKey, model string) *Provider {
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	clientVal := sdk.NewClient(option.WithAPIKey(apiKey))
	return &Provider{client: clientVal, model: model}
}

// Model returns the default model name.
func (p *Provider) Model() string { return p.model }

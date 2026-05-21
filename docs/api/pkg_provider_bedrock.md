# `github.com/ricardo/anthrogo/pkg/provider/bedrock`

```go
package bedrock // import "github.com/ricardo/anthrogo/pkg/provider/bedrock"

Package bedrock provides an anthrogo Provider backed by Anthropic models
hosted on AWS Bedrock. Authentication is handled by the AWS default credential
chain (environment variables, ~/.aws/credentials, EC2/ECS IAM role, etc.).
No Anthropic API key is required.

Model IDs follow the Bedrock convention:

    anthropic.claude-sonnet-4-6-v1:0
    anthropic.claude-haiku-4-5-20251001-v1:0

TYPES

type Provider struct {
	// Has unexported fields.
}
    Provider talks to Anthropic models hosted on AWS Bedrock. It delegates the
    streaming implementation to pkg/provider/anthropic.Provider so that all
    event translation logic lives in one place.

func New(ctx context.Context, region, model string) (*Provider, error)
    New constructs a Bedrock-backed Provider.

    region is the AWS region (e.g. "us-west-2"). When empty, the region is
    resolved by the AWS default config chain (AWS_REGION env var, ~/.aws/config,
    etc.).

    model is the Bedrock model ID (e.g. "anthropic.claude-sonnet-4-6-v1:0").

    The constructor only loads AWS config; it does NOT make network calls.
    Credential failures are deferred to the first Stream call.

func (p *Provider) Stream(ctx context.Context, req provider.Request) (<-chan provider.Event, error)
    Stream implements provider.Provider. Requires valid AWS credentials in the
    environment; returns an error event on the channel when they are absent or
    invalid.

```

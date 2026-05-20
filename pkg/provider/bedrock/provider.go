// Package bedrock provides an anthrogo Provider backed by Anthropic models
// hosted on AWS Bedrock.  Authentication is handled by the AWS default
// credential chain (environment variables, ~/.aws/credentials, EC2/ECS IAM
// role, etc.).  No Anthropic API key is required.
//
// Model IDs follow the Bedrock convention:
//
//	anthropic.claude-sonnet-4-6-v1:0
//	anthropic.claude-haiku-4-5-20251001-v1:0
package bedrock

import (
	"context"

	sdkbedrock "github.com/anthropics/anthropic-sdk-go/bedrock"

	"github.com/aws/aws-sdk-go-v2/config"

	"github.com/ricardo/anthrogo/pkg/provider"
	anthroProv "github.com/ricardo/anthrogo/pkg/provider/anthropic"
)

// Provider talks to Anthropic models hosted on AWS Bedrock.  It delegates the
// streaming implementation to pkg/provider/anthropic.Provider so that all
// event translation logic lives in one place.
type Provider struct {
	inner *anthroProv.Provider
}

// New constructs a Bedrock-backed Provider.
//
// region is the AWS region (e.g. "us-west-2").  When empty, the region is
// resolved by the AWS default config chain (AWS_REGION env var,
// ~/.aws/config, etc.).
//
// model is the Bedrock model ID (e.g. "anthropic.claude-sonnet-4-6-v1:0").
//
// The constructor only loads AWS config; it does NOT make network calls.
// Credential failures are deferred to the first Stream call.
func New(ctx context.Context, region, model string) (*Provider, error) {
	var loadOpts []func(*config.LoadOptions) error
	if region != "" {
		loadOpts = append(loadOpts, config.WithRegion(region))
	}
	// WithLoadDefaultConfig loads the default AWS credential chain and
	// registers the Bedrock signing middleware.  It panics on config parse
	// errors (malformed ~/.aws/config), but does NOT fail on missing creds;
	// those are checked lazily at request time.
	opt := sdkbedrock.WithLoadDefaultConfig(ctx, loadOpts...)
	inner := anthroProv.NewWithOptions(model, opt)
	return &Provider{inner: inner}, nil
}

// Stream implements provider.Provider.  Requires valid AWS credentials in the
// environment; returns an error event on the channel when they are absent or
// invalid.
func (p *Provider) Stream(ctx context.Context, req provider.Request) (<-chan provider.Event, error) {
	return p.inner.Stream(ctx, req)
}

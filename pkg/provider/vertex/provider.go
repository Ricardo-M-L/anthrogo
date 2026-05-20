// Package vertex provides an anthrogo Provider backed by Anthropic models
// hosted on Google Cloud Vertex AI. Authentication is handled by the Google
// Application Default Credentials chain (GOOGLE_APPLICATION_CREDENTIALS env,
// `gcloud auth application-default login`, or workload identity).
// No Anthropic API key is required.
//
// Model IDs follow Vertex AI / Model Garden convention:
//
//	claude-sonnet-4-6@20260101
//	claude-haiku-4-5@20251001
package vertex

import (
	"context"
	"fmt"

	sdkvertex "github.com/anthropics/anthropic-sdk-go/vertex"

	"github.com/ricardo/anthrogo/pkg/provider"
	anthroProv "github.com/ricardo/anthrogo/pkg/provider/anthropic"
)

// Provider talks to Anthropic models hosted on Google Cloud Vertex AI. It
// delegates the streaming implementation to pkg/provider/anthropic.Provider so
// that all event translation logic lives in one place.
type Provider struct {
	inner *anthroProv.Provider
}

// New constructs a Vertex-backed Provider.
//
// region is the GCP region (e.g. "us-east5"). It is mandatory; Vertex requires
// an explicit region to select the serving endpoint.
//
// projectID is the GCP project (e.g. "my-gcp-project"). It is mandatory.
//
// model is the Vertex AI model ID (e.g. "claude-sonnet-4-6@20260101").
//
// Note: the Anthropic SDK's WithGoogleAuth eagerly calls
// google.FindDefaultCredentials at construction time and panics on failure.
// New recovers that panic and converts it to an error so callers get a clean
// error message instead of a crash when GCP credentials are absent.
func New(ctx context.Context, region, projectID, model string) (p *Provider, err error) {
	if region == "" {
		return nil, fmt.Errorf("vertex provider requires region (e.g. us-east5)")
	}
	if projectID == "" {
		return nil, fmt.Errorf("vertex provider requires project_id")
	}

	// WithGoogleAuth panics when Google Application Default Credentials are
	// unavailable. Recover and return a descriptive error instead.
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("vertex provider: %v (set GOOGLE_APPLICATION_CREDENTIALS or run `gcloud auth application-default login`)", r)
		}
	}()

	// WithGoogleAuth signature: (ctx, region, projectID string, scopes ...string) option.RequestOption
	opt := sdkvertex.WithGoogleAuth(ctx, region, projectID)
	inner := anthroProv.NewWithOptions(model, opt)
	return &Provider{inner: inner}, nil
}

// Stream implements provider.Provider. Requires valid Google Application
// Default Credentials in the environment; returns an error event on the
// channel when they are absent or invalid.
func (p *Provider) Stream(ctx context.Context, req provider.Request) (<-chan provider.Event, error) {
	return p.inner.Stream(ctx, req)
}

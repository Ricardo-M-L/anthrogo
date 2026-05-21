# `github.com/ricardo/anthrogo/pkg/provider/vertex`

```go
package vertex // import "github.com/ricardo/anthrogo/pkg/provider/vertex"

Package vertex provides an anthrogo Provider backed by Anthropic models hosted
on Google Cloud Vertex AI. Authentication is handled by the Google Application
Default Credentials chain (GOOGLE_APPLICATION_CREDENTIALS env, `gcloud auth
application-default login`, or workload identity). No Anthropic API key is
required.

Model IDs follow Vertex AI / Model Garden convention:

    claude-sonnet-4-6@20260101
    claude-haiku-4-5@20251001

TYPES

type Provider struct {
	// Has unexported fields.
}
    Provider talks to Anthropic models hosted on Google Cloud Vertex AI.
    It delegates the streaming implementation to pkg/provider/anthropic.Provider
    so that all event translation logic lives in one place.

func New(ctx context.Context, region, projectID, model string) (p *Provider, err error)
    New constructs a Vertex-backed Provider.

    region is the GCP region (e.g. "us-east5"). It is mandatory; Vertex requires
    an explicit region to select the serving endpoint.

    projectID is the GCP project (e.g. "my-gcp-project"). It is mandatory.

    model is the Vertex AI model ID (e.g. "claude-sonnet-4-6@20260101").

    Note: the Anthropic SDK's WithGoogleAuth eagerly calls
    google.FindDefaultCredentials at construction time and panics on failure.
    New recovers that panic and converts it to an error so callers get a clean
    error message instead of a crash when GCP credentials are absent.

func (p *Provider) Stream(ctx context.Context, req provider.Request) (<-chan provider.Event, error)
    Stream implements provider.Provider. Requires valid Google Application
    Default Credentials in the environment; returns an error event on the
    channel when they are absent or invalid.

```

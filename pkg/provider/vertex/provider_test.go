package vertex_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/pkg/provider/vertex"
)

func TestVertexProvider_RequiresRegion(t *testing.T) {
	_, err := vertex.New(context.Background(), "", "my-proj", "claude-sonnet-4-6@20260101")
	require.Error(t, err)
	require.Contains(t, err.Error(), "region")
}

func TestVertexProvider_RequiresProjectID(t *testing.T) {
	_, err := vertex.New(context.Background(), "us-east5", "", "claude-sonnet-4-6@20260101")
	require.Error(t, err)
	require.Contains(t, err.Error(), "project_id")
}

// TestVertexProvider_ConstructsWithRegionAndProject verifies that the
// constructor either succeeds (when GCP credentials are available) or returns
// a descriptive error (when they are absent). It must never panic.
//
// Note: unlike Bedrock, the Anthropic SDK's WithGoogleAuth eagerly resolves
// Application Default Credentials at construction time, so this test passes
// when run with GOOGLE_APPLICATION_CREDENTIALS set or after
// `gcloud auth application-default login`, and returns an error otherwise.
func TestVertexProvider_ConstructsWithRegionAndProject(t *testing.T) {
	p, err := vertex.New(context.Background(), "us-east5", "my-proj", "claude-sonnet-4-6@20260101")
	if err != nil {
		// No GCP credentials in this environment — acceptable. Verify the
		// error message is informative rather than a raw panic string.
		require.Contains(t, err.Error(), "vertex provider")
		t.Skipf("no GCP credentials available (%v); skipping construction test", err)
		return
	}
	require.NotNil(t, p)
}

// Stream-level tests require real GCP credentials and are intentionally
// omitted. To test against a live Vertex endpoint, set
// GOOGLE_APPLICATION_CREDENTIALS (or run `gcloud auth application-default login`)
// and call p.Stream directly.

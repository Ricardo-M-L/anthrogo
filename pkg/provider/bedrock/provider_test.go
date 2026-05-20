package bedrock_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/pkg/provider/bedrock"
)

// TestBedrockProvider_ConstructsWithoutAWS verifies that New does not fail
// when AWS credentials are absent.  Config loading is not credential retrieval;
// credentials are checked lazily at first Stream call.
func TestBedrockProvider_ConstructsWithoutAWS(t *testing.T) {
	p, err := bedrock.New(context.Background(), "", "anthropic.claude-sonnet-4-6-v1:0")
	require.NoError(t, err)
	require.NotNil(t, p)
}

// TestBedrockProvider_RegionSet verifies that an explicit region does not
// cause the constructor to fail.
func TestBedrockProvider_RegionSet(t *testing.T) {
	p, err := bedrock.New(context.Background(), "us-west-2", "anthropic.claude-sonnet-4-6-v1:0")
	require.NoError(t, err)
	require.NotNil(t, p)
}

// Stream-level tests require real AWS credentials and are intentionally omitted.
// To test against a live Bedrock endpoint, set AWS_ACCESS_KEY_ID,
// AWS_SECRET_ACCESS_KEY, AWS_REGION and call p.Stream directly.

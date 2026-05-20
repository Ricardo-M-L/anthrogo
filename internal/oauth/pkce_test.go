package oauth

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateChallenge_LengthAndUniqueness(t *testing.T) {
	v1, c1, err := GenerateChallenge()
	require.NoError(t, err)
	require.NotEmpty(t, v1)
	require.NotEmpty(t, c1)

	v2, c2, err := GenerateChallenge()
	require.NoError(t, err)

	// Verifiers and challenges should be different each call.
	require.NotEqual(t, v1, v2)
	require.NotEqual(t, c1, c2)

	// A 32-byte random value base64url-encoded (no padding) is 43 chars.
	require.Equal(t, 43, len(v1))
	// A sha256 digest (32 bytes) base64url-encoded (no padding) is 43 chars.
	require.Equal(t, 43, len(c1))
}

func TestGenerateChallenge_S256(t *testing.T) {
	verifier, challenge, err := GenerateChallenge()
	require.NoError(t, err)

	// Verify: challenge == base64url(sha256(verifier))
	sum := sha256.Sum256([]byte(verifier))
	expected := base64.RawURLEncoding.EncodeToString(sum[:])
	require.Equal(t, expected, challenge)
}

func TestGenerateChallenge_RawURLEncoding(t *testing.T) {
	// Raw URL encoding must not include padding ('=') or '+' or '/'
	verifier, challenge, err := GenerateChallenge()
	require.NoError(t, err)
	require.NotContains(t, verifier, "=")
	require.NotContains(t, verifier, "+")
	require.NotContains(t, verifier, "/")
	require.NotContains(t, challenge, "=")
	require.NotContains(t, challenge, "+")
	require.NotContains(t, challenge, "/")
}

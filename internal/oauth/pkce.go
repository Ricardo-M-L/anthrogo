package oauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"io"
)

// GenerateChallenge returns (verifier, challenge) where challenge =
// base64url(sha256(verifier)) — implements PKCE S256 method per RFC 7636.
func GenerateChallenge() (verifier, challenge string, err error) {
	b := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", "", err
	}
	verifier = base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	return verifier, challenge, nil
}

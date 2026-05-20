package kairos

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// GenerateKeyPair returns (private, public) keys.
func GenerateKeyPair() (priv ed25519.PrivateKey, pub ed25519.PublicKey, err error) {
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	return privKey, pubKey, nil
}

// LoadPrivateKey reads a base64-encoded ed25519 private key from a file.
func LoadPrivateKey(path string) (ed25519.PrivateKey, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil {
		return nil, fmt.Errorf("invalid base64: %w", err)
	}
	if len(decoded) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("expected %d bytes, got %d", ed25519.PrivateKeySize, len(decoded))
	}
	return ed25519.PrivateKey(decoded), nil
}

// LoadPublicKey reads a base64 ed25519 public key from a file or inline literal.
//
// Detection order:
//  1. Try to decode input directly as base64. If the result is exactly
//     ed25519.PublicKeySize (32) bytes, use it.
//  2. Otherwise treat input as a file path and read the file, then decode its
//     contents as base64.
//
// This handles the common case where an inline base64 literal happens to start
// with '/' (valid base64 character) without incorrectly treating it as a path.
func LoadPublicKey(input string) (ed25519.PublicKey, error) {
	// First, attempt inline base64 decode.
	if decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(input)); err == nil {
		if len(decoded) == ed25519.PublicKeySize {
			return ed25519.PublicKey(decoded), nil
		}
	}
	// Fall back to file read.
	data, err := os.ReadFile(input)
	if err != nil {
		return nil, err
	}
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(data)))
	if err != nil {
		return nil, fmt.Errorf("invalid base64: %w", err)
	}
	if len(decoded) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("expected %d bytes, got %d", ed25519.PublicKeySize, len(decoded))
	}
	return ed25519.PublicKey(decoded), nil
}

// SignedFrame wraps an SSE payload with a base64 ed25519 signature over its
// canonical JSON marshalling.
type SignedFrame struct {
	Payload   json.RawMessage `json:"payload"`
	Signature string          `json:"sig"` // base64 of Sign(payload)
}

// SignFrame signs the given payload bytes with the private key and returns a SignedFrame.
func SignFrame(priv ed25519.PrivateKey, payload []byte) SignedFrame {
	sig := ed25519.Sign(priv, payload)
	return SignedFrame{
		Payload:   json.RawMessage(payload),
		Signature: base64.StdEncoding.EncodeToString(sig),
	}
}

// VerifyFrame verifies the signature in the frame against its payload using the
// given public key. Returns nil on success, an error on failure.
func VerifyFrame(pub ed25519.PublicKey, frame SignedFrame) error {
	sig, err := base64.StdEncoding.DecodeString(frame.Signature)
	if err != nil {
		return fmt.Errorf("invalid signature base64: %w", err)
	}
	if !ed25519.Verify(pub, frame.Payload, sig) {
		return errors.New("signature verification failed")
	}
	return nil
}

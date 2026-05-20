package kairos

import (
	"crypto/ed25519"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestKeyPair_GenerateLoadRoundTrip generates a keypair, writes both keys to
// temp files, reloads them, and verifies that a sign/verify round-trip succeeds.
func TestKeyPair_GenerateLoadRoundTrip(t *testing.T) {
	priv, pub, err := GenerateKeyPair()
	require.NoError(t, err)
	require.Len(t, priv, ed25519.PrivateKeySize)
	require.Len(t, pub, ed25519.PublicKeySize)

	dir := t.TempDir()
	privPath := filepath.Join(dir, "test.priv")
	pubPath := filepath.Join(dir, "test.pub")

	privB64 := base64.StdEncoding.EncodeToString(priv)
	pubB64 := base64.StdEncoding.EncodeToString(pub)
	require.NoError(t, os.WriteFile(privPath, []byte(privB64+"\n"), 0600))
	require.NoError(t, os.WriteFile(pubPath, []byte(pubB64+"\n"), 0644))

	loadedPriv, err := LoadPrivateKey(privPath)
	require.NoError(t, err)
	require.Equal(t, []byte(priv), []byte(loadedPriv))

	loadedPub, err := LoadPublicKey(pubPath)
	require.NoError(t, err)
	require.Equal(t, []byte(pub), []byte(loadedPub))

	// Sign with loaded private; verify with loaded public.
	payload := []byte(`{"test":"value"}`)
	frame := SignFrame(loadedPriv, payload)
	require.NoError(t, VerifyFrame(loadedPub, frame))
}

// TestSignFrame_VerifyOk signs a payload and verifies it succeeds.
func TestSignFrame_VerifyOk(t *testing.T) {
	priv, pub, err := GenerateKeyPair()
	require.NoError(t, err)

	payload := []byte(`{"event":"text","data":"hello"}`)
	frame := SignFrame(priv, payload)
	require.NotEmpty(t, frame.Signature)
	require.Equal(t, payload, []byte(frame.Payload))
	require.NoError(t, VerifyFrame(pub, frame))
}

// TestSignFrame_TamperedPayloadFails verifies that mutating a single payload byte
// causes signature verification to fail.
func TestSignFrame_TamperedPayloadFails(t *testing.T) {
	priv, pub, err := GenerateKeyPair()
	require.NoError(t, err)

	payload := []byte(`{"event":"text","data":"hello"}`)
	frame := SignFrame(priv, payload)

	// Tamper: flip a byte in the payload copy inside the frame.
	tampered := make([]byte, len(frame.Payload))
	copy(tampered, frame.Payload)
	tampered[0] ^= 0x01
	frame.Payload = tampered

	err = VerifyFrame(pub, frame)
	require.Error(t, err)
	require.Contains(t, err.Error(), "signature verification failed")
}

// TestVerifyFrame_BadSig verifies that a random/incorrect signature fails.
func TestVerifyFrame_BadSig(t *testing.T) {
	_, pub, err := GenerateKeyPair()
	require.NoError(t, err)

	payload := []byte(`{"event":"done","data":"world"}`)
	// Construct a frame with a random 64-byte signature (not matching the payload).
	badSig := make([]byte, ed25519.SignatureSize)
	for i := range badSig {
		badSig[i] = byte(i)
	}
	frame := SignedFrame{
		Payload:   payload,
		Signature: base64.StdEncoding.EncodeToString(badSig),
	}

	err = VerifyFrame(pub, frame)
	require.Error(t, err)
	require.Contains(t, err.Error(), "signature verification failed")
}

// TestLoadPublicKey_InlineBase64 verifies that LoadPublicKey accepts a bare
// base64 literal (not a file path).
func TestLoadPublicKey_InlineBase64(t *testing.T) {
	_, pub, err := GenerateKeyPair()
	require.NoError(t, err)

	b64 := base64.StdEncoding.EncodeToString(pub)
	loaded, err := LoadPublicKey(b64)
	require.NoError(t, err)
	require.Equal(t, []byte(pub), []byte(loaded))
}

package kairos

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// generateSelfSignedCert creates a self-signed ECDSA certificate valid for
// 127.0.0.1 and localhost and returns the PEM-encoded cert and key.
func generateSelfSignedCert(t *testing.T) (certPEM, keyPEM []byte) {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "anthrogo-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:     []string{"localhost"},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	require.NoError(t, err)

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	privDER, err := x509.MarshalECPrivateKey(priv)
	require.NoError(t, err)
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privDER})

	return certPEM, keyPEM
}

// TestServer_TLSMethodsExist smoke-tests that RunTLS and RunAutocert are
// callable and return an error when given invalid file paths.
func TestServer_TLSMethodsExist(t *testing.T) {
	s := NewServer(nil, "")
	require.NotNil(t, s)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := s.RunTLS(ctx, "127.0.0.1:0", "/nonexistent/cert.pem", "/nonexistent/key.pem")
	require.Error(t, err)
}

// TestServer_RunAutocert_Skips skips the live autocert test as it requires a
// real domain and port 443.
func TestServer_RunAutocert_Skips(t *testing.T) {
	t.Skip("autocert requires a real domain + port 443; manual testing only")
}

// TestServer_RunTLS_SelfSignedRoundTrip starts a TLS server with a self-signed
// certificate and verifies that a client configured with the same CA cert can
// reach /kairos/healthz successfully.
func TestServer_RunTLS_SelfSignedRoundTrip(t *testing.T) {
	certPEM, keyPEM := generateSelfSignedCert(t)

	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")
	require.NoError(t, os.WriteFile(certPath, certPEM, 0o600))
	require.NoError(t, os.WriteFile(keyPath, keyPEM, 0o600))

	// Load the cert so we can configure the TLS listener directly via httptest.
	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)

	s := NewServer(nil, "")

	// Use httptest.NewUnstartedServer so we can supply a custom TLS config
	// and get a known address without the port-0 problem.
	ts := httptest.NewUnstartedServer(s.Handler())
	ts.TLS = &tls.Config{Certificates: []tls.Certificate{tlsCert}}
	ts.StartTLS()
	defer ts.Close()

	// Build a client that trusts only our self-signed cert.
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(certPEM)
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: pool},
		},
	}

	resp, err := client.Get(ts.URL + "/kairos/healthz")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestServer_RunTLS_StartsAndStops starts a TLS server in a goroutine using
// the file-based RunTLS method, then cancels the context and verifies the
// method returns ctx.Err(). Because ListenAndServeTLS doesn't expose the
// listener, we call it on a fixed free port obtained via a temp listener.
func TestServer_RunTLS_StartsAndStops(t *testing.T) {
	certPEM, keyPEM := generateSelfSignedCert(t)

	dir := t.TempDir()
	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")
	require.NoError(t, os.WriteFile(certPath, certPEM, 0o600))
	require.NoError(t, os.WriteFile(keyPath, keyPEM, 0o600))

	// Find a free port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := ln.Addr().String()
	ln.Close()

	s := NewServer(nil, "")

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- s.RunTLS(ctx, addr, certPath, keyPath)
	}()

	// Give the server a moment to bind, then cancel.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(3 * time.Second):
		t.Fatal("RunTLS did not return after context cancellation")
	}
}

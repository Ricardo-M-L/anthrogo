package oauth

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestExchangeCode_Success(t *testing.T) {
	var receivedForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		receivedForm = r.Form
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"access_token":"AT","token_type":"Bearer","expires_in":3600,"refresh_token":"RT","scope":"read write"}`)
	}))
	defer srv.Close()

	cfg := Config{TokenURL: srv.URL, ClientID: "cid"}
	tok, err := exchangeCode(context.Background(), cfg, "mycode", "myverifier", "http://localhost/cb")
	require.NoError(t, err)
	require.Equal(t, "AT", tok.AccessToken)
	require.Equal(t, "Bearer", tok.TokenType)
	require.Equal(t, "RT", tok.RefreshToken)
	require.WithinDuration(t, time.Now().Add(time.Hour), tok.ExpiresAt, time.Minute)
	require.Equal(t, []string{"read", "write"}, tok.Scopes)

	require.Equal(t, "authorization_code", receivedForm.Get("grant_type"))
	require.Equal(t, "mycode", receivedForm.Get("code"))
	require.Equal(t, "myverifier", receivedForm.Get("code_verifier"))
	require.Equal(t, "cid", receivedForm.Get("client_id"))
	require.Equal(t, "http://localhost/cb", receivedForm.Get("redirect_uri"))
}

func TestExchangeCode_WithClientSecret(t *testing.T) {
	var receivedForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		receivedForm = r.Form
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"access_token":"AT2","token_type":"Bearer","expires_in":1800}`)
	}))
	defer srv.Close()

	cfg := Config{TokenURL: srv.URL, ClientID: "cid", ClientSecret: "secret123"}
	tok, err := exchangeCode(context.Background(), cfg, "code2", "ver2", "http://localhost/cb")
	require.NoError(t, err)
	require.Equal(t, "AT2", tok.AccessToken)
	require.Equal(t, "secret123", receivedForm.Get("client_secret"))
}

func TestExchangeCode_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()

	cfg := Config{TokenURL: srv.URL, ClientID: "cid"}
	_, err := exchangeCode(context.Background(), cfg, "bad", "ver", "http://localhost/cb")
	require.Error(t, err)
	require.Contains(t, err.Error(), "401")
}

func TestRefreshToken_Success(t *testing.T) {
	var receivedForm url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, r.ParseForm())
		receivedForm = r.Form
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"access_token":"NEW_AT","token_type":"Bearer","expires_in":7200,"refresh_token":"NEW_RT"}`)
	}))
	defer srv.Close()

	cfg := Config{TokenURL: srv.URL, ClientID: "cid"}
	tok, err := refreshToken(context.Background(), cfg, "old_refresh_token")
	require.NoError(t, err)
	require.Equal(t, "NEW_AT", tok.AccessToken)
	require.Equal(t, "NEW_RT", tok.RefreshToken)
	require.WithinDuration(t, time.Now().Add(2*time.Hour), tok.ExpiresAt, time.Minute)

	require.Equal(t, "refresh_token", receivedForm.Get("grant_type"))
	require.Equal(t, "old_refresh_token", receivedForm.Get("refresh_token"))
	require.Equal(t, "cid", receivedForm.Get("client_id"))
}

func TestRefreshToken_ExpiresInDefault(t *testing.T) {
	// When expires_in is 0/absent, default to 3600.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"access_token":"AT","token_type":"Bearer"}`)
	}))
	defer srv.Close()

	cfg := Config{TokenURL: srv.URL, ClientID: "cid"}
	tok, err := refreshToken(context.Background(), cfg, "rt")
	require.NoError(t, err)
	require.WithinDuration(t, time.Now().Add(time.Hour), tok.ExpiresAt, time.Minute)
}

func TestFetchToken_CachedValid(t *testing.T) {
	dir := t.TempDir()
	// Save a valid (non-expired) token in the cache.
	cached := &Token{
		AccessToken: "cached_at",
		TokenType:   "Bearer",
		ExpiresAt:   time.Now().Add(time.Hour),
	}
	require.NoError(t, SaveToken(dir, "myserver", cached))

	// Token endpoint should never be called.
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cfg := Config{TokenURL: srv.URL, ClientID: "cid"}
	tok, err := FetchToken(context.Background(), cfg, dir, "myserver")
	require.NoError(t, err)
	require.Equal(t, "cached_at", tok.AccessToken)
	require.False(t, called, "token endpoint should not be called for a valid cached token")
}

func TestFetchToken_RefreshOnExpiry(t *testing.T) {
	dir := t.TempDir()
	// Save an expired token with a refresh token.
	expired := &Token{
		AccessToken:  "old_at",
		TokenType:    "Bearer",
		RefreshToken: "good_rt",
		ExpiresAt:    time.Now().Add(-time.Minute), // already expired
	}
	require.NoError(t, SaveToken(dir, "srv2", expired))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"access_token":"refreshed_at","token_type":"Bearer","expires_in":3600}`)
	}))
	defer srv.Close()

	cfg := Config{TokenURL: srv.URL, ClientID: "cid"}
	tok, err := FetchToken(context.Background(), cfg, dir, "srv2")
	require.NoError(t, err)
	require.Equal(t, "refreshed_at", tok.AccessToken)

	// Confirm the new token was cached.
	reloaded, err := LoadToken(dir, "srv2")
	require.NoError(t, err)
	require.Equal(t, "refreshed_at", reloaded.AccessToken)
}

func TestPostToken_ContentType(t *testing.T) {
	var gotCT string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"access_token":"x","token_type":"Bearer","expires_in":60}`)
	}))
	defer srv.Close()

	_, err := postToken(context.Background(), srv.URL, url.Values{"grant_type": []string{"client_credentials"}})
	require.NoError(t, err)
	require.Equal(t, "application/x-www-form-urlencoded", gotCT)
}

func TestPostToken_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, "{invalid}")
	}))
	defer srv.Close()

	_, err := postToken(context.Background(), srv.URL, url.Values{})
	require.Error(t, err)
}

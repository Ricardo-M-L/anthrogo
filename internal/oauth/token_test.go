package oauth

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestToken_IsExpired(t *testing.T) {
	// A token that expires in 1 hour is not expired.
	tok := &Token{ExpiresAt: time.Now().Add(time.Hour)}
	require.False(t, tok.IsExpired(), "token expiring in 1h should not be expired")

	// A token that expired 1 second ago is expired.
	tok2 := &Token{ExpiresAt: time.Now().Add(-time.Second)}
	require.True(t, tok2.IsExpired(), "token expiring -1s ago should be expired")
}

func TestToken_IsExpired_SkewMargin(t *testing.T) {
	// A token expiring in exactly 29s is within the 30s skew margin → expired.
	tok := &Token{ExpiresAt: time.Now().Add(29 * time.Second)}
	require.True(t, tok.IsExpired(), "token expiring in 29s should be considered expired (30s skew)")

	// A token expiring in 31s is outside the 30s skew margin → not expired.
	tok2 := &Token{ExpiresAt: time.Now().Add(31 * time.Second)}
	require.False(t, tok2.IsExpired(), "token expiring in 31s should not be expired (30s skew)")
}

func TestSaveLoadToken_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	want := &Token{
		AccessToken:  "access123",
		TokenType:    "Bearer",
		RefreshToken: "refresh456",
		ExpiresAt:    time.Now().Add(time.Hour).Truncate(time.Second),
		Scopes:       []string{"read", "write"},
	}
	require.NoError(t, SaveToken(dir, "myserver", want))

	got, err := LoadToken(dir, "myserver")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, want.AccessToken, got.AccessToken)
	require.Equal(t, want.TokenType, got.TokenType)
	require.Equal(t, want.RefreshToken, got.RefreshToken)
	require.WithinDuration(t, want.ExpiresAt, got.ExpiresAt, time.Second)
	require.Equal(t, want.Scopes, got.Scopes)
}

func TestLoadToken_NotExist(t *testing.T) {
	dir := t.TempDir()
	tok, err := LoadToken(dir, "nonexistent")
	require.NoError(t, err)
	require.Nil(t, tok)
}

func TestSaveToken_FilePermissions(t *testing.T) {
	dir := t.TempDir()
	tok := &Token{AccessToken: "at", TokenType: "Bearer", ExpiresAt: time.Now().Add(time.Hour)}
	require.NoError(t, SaveToken(dir, "srv", tok))

	info, err := os.Stat(filepath.Join(dir, "srv.json"))
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestLoadToken_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	require.NoError(t, os.WriteFile(path, []byte("{not valid json"), 0o600))
	_, err := LoadToken(dir, "bad")
	require.Error(t, err)
}

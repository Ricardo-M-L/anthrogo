package builtins

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/internal/oauth"
)

func TestLogin_NotConfigured(t *testing.T) {
	res, _ := (Login{}).Run(context.Background(), "", newFakeHost())
	require.Contains(t, res.Text, "settings.yaml")
}

func TestLogin_Status_NoToken(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	res, _ := (Login{}).Run(context.Background(), "status", newFakeHost())
	require.Contains(t, res.Text, "not logged in")
}

func TestLogin_Status_HasValidToken(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	require.NoError(t, oauth.SaveToken(filepath.Join(tmpHome, ".anthrogo", "auth"), "anthropic", &oauth.Token{
		AccessToken: "tok",
		TokenType:   "Bearer",
		ExpiresAt:   time.Now().Add(time.Hour),
	}))
	res, _ := (Login{}).Run(context.Background(), "status", newFakeHost())
	require.Contains(t, res.Text, "logged in")
	require.NotContains(t, res.Text, "EXPIRED")
}

func TestLogin_Logout_RemovesFile(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	require.NoError(t, oauth.SaveToken(filepath.Join(tmpHome, ".anthrogo", "auth"), "anthropic", &oauth.Token{
		AccessToken: "x",
		ExpiresAt:   time.Now().Add(time.Hour),
	}))
	res, _ := (Login{}).Run(context.Background(), "logout", newFakeHost())
	require.Contains(t, res.Text, "logged out")
	_, err := os.Stat(filepath.Join(tmpHome, ".anthrogo", "auth", "anthropic.json"))
	require.True(t, os.IsNotExist(err))
}

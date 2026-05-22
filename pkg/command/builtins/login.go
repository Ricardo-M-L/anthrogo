package builtins

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ricardo/anthrogo/internal/oauth"
	"github.com/ricardo/anthrogo/pkg/command"
)

// Login implements /login — runs the M6.5 OAuth 2.1 PKCE flow and saves the
// resulting token to ~/.anthrogo/auth/anthropic.json.
type Login struct {
	Config oauth.Config
}

func (Login) Name() string      { return "/login" }
func (Login) Aliases() []string { return nil }
func (Login) Description() string {
	return "Run OAuth 2.1 PKCE flow against the configured IdP; saves a token used by the Anthropic provider."
}
func (Login) Type() command.Type { return command.TypeLocal }

func (l Login) Run(ctx context.Context, args string, host command.Host) (command.Result, error) {
	args = strings.TrimSpace(args)
	if args == "status" {
		return loginStatus(host)
	}
	if args == "logout" {
		return loginLogout(host)
	}
	if l.Config.AuthorizationURL == "" || l.Config.TokenURL == "" || l.Config.ClientID == "" {
		return command.Result{Text: "login: auth.authorization_url, auth.token_url, and auth.client_id must be set in settings.yaml"}, nil
	}
	cacheRoot := filepath.Join(os.Getenv("HOME"), ".anthrogo", "auth")
	tok, err := oauth.FetchToken(ctx, l.Config, cacheRoot, "anthropic")
	if err != nil {
		return command.Result{Text: "login failed: " + err.Error()}, nil
	}
	return command.Result{Text: fmt.Sprintf("logged in; token expires at %s\nrestart anthrogo (or the next provider init) picks up the new token.", tok.ExpiresAt.Format("2006-01-02 15:04 MST"))}, nil
}

func loginStatus(_ command.Host) (command.Result, error) {
	cacheRoot := filepath.Join(os.Getenv("HOME"), ".anthrogo", "auth")
	tok, err := oauth.LoadToken(cacheRoot, "anthropic")
	if err != nil {
		return command.Result{Text: "login status: " + err.Error()}, nil
	}
	if tok == nil {
		return command.Result{Text: "not logged in (run /login)"}, nil
	}
	return command.Result{Text: fmt.Sprintf("logged in; token expires at %s%s",
		tok.ExpiresAt.Format("2006-01-02 15:04 MST"),
		ifThen(tok.IsExpired(), " (EXPIRED — run /login)", ""))}, nil
}

func ifThen(cond bool, yes, no string) string {
	if cond {
		return yes
	}
	return no
}

func loginLogout(_ command.Host) (command.Result, error) {
	cacheRoot := filepath.Join(os.Getenv("HOME"), ".anthrogo", "auth")
	path := filepath.Join(cacheRoot, "anthropic.json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return command.Result{Text: "logout failed: " + err.Error()}, nil
	}
	return command.Result{Text: "logged out (token file removed)"}, nil
}

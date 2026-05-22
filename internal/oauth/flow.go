package oauth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// Config holds the OAuth 2.1 parameters for a single authorization server.
type Config struct {
	AuthorizationURL string
	TokenURL         string
	ClientID         string
	ClientSecret     string // optional (PKCE-only public clients)
	Scopes           []string
	RedirectPort     int // 0 → 8765
}

// FetchToken returns a valid token for serverName, refreshing or launching the
// browser authorization-code flow as needed. Tokens are cached under cacheRoot.
func FetchToken(ctx context.Context, cfg Config, cacheRoot, serverName string) (*Token, error) {
	if cfg.RedirectPort == 0 {
		cfg.RedirectPort = 8765
	}

	cached, _ := LoadToken(cacheRoot, serverName)
	if cached != nil && !cached.IsExpired() {
		return cached, nil
	}
	if cached != nil && cached.RefreshToken != "" {
		refreshed, err := refreshToken(ctx, cfg, cached.RefreshToken)
		if err == nil {
			if saveErr := SaveToken(cacheRoot, serverName, refreshed); saveErr != nil {
				fmt.Fprintf(os.Stderr, "oauth: cache refreshed token: %v\n", saveErr)
			}
			return refreshed, nil
		}
		// fall through to full re-auth on refresh failure
	}

	token, err := authorizationCodeFlow(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if saveErr := SaveToken(cacheRoot, serverName, token); saveErr != nil {
		fmt.Fprintf(os.Stderr, "oauth: cache token: %v\n", saveErr)
	}
	return token, nil
}

func authorizationCodeFlow(ctx context.Context, cfg Config) (*Token, error) {
	verifier, challenge, err := GenerateChallenge()
	if err != nil {
		return nil, err
	}
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/callback", cfg.RedirectPort)
	state := randomState()

	// Start local listener before opening browser so we don't miss the redirect.
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", cfg.RedirectPort))
	if err != nil {
		return nil, fmt.Errorf("oauth: bind redirect port %d: %w", cfg.RedirectPort, err)
	}

	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)
	server := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			q := r.URL.Query()
			if q.Get("state") != state {
				errCh <- fmt.Errorf("oauth callback: state mismatch")
				fmt.Fprintln(w, "state mismatch")
				return
			}
			if errParam := q.Get("error"); errParam != "" {
				errCh <- fmt.Errorf("oauth callback error: %s — %s", errParam, q.Get("error_description"))
				fmt.Fprintf(w, "OAuth error: %s", errParam)
				return
			}
			code := q.Get("code")
			if code == "" {
				errCh <- fmt.Errorf("oauth callback: missing code")
				fmt.Fprintln(w, "missing code")
				return
			}
			fmt.Fprintln(w, "Authorization complete. You can close this tab.")
			codeCh <- code
		}),
	}
	go func() { _ = server.Serve(listener) }()
	defer server.Close()

	// Build the authorization URL.
	authURL, err := url.Parse(cfg.AuthorizationURL)
	if err != nil {
		return nil, fmt.Errorf("oauth: invalid authorization_url %q: %w", cfg.AuthorizationURL, err)
	}
	q := authURL.Query()
	q.Set("response_type", "code")
	q.Set("client_id", cfg.ClientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("state", state)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	if len(cfg.Scopes) > 0 {
		q.Set("scope", strings.Join(cfg.Scopes, " "))
	}
	authURL.RawQuery = q.Encode()

	fmt.Fprintf(stderrTarget, "OAuth: open %s in your browser to authorize.\n", authURL)
	_ = openBrowserFn(authURL.String())

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-errCh:
		return nil, err
	case <-time.After(5 * time.Minute):
		return nil, fmt.Errorf("oauth: timed out waiting for callback after 5m")
	case code := <-codeCh:
		return exchangeCode(ctx, cfg, code, verifier, redirectURI)
	}
}

func exchangeCode(ctx context.Context, cfg Config, code, verifier, redirectURI string) (*Token, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("client_id", cfg.ClientID)
	form.Set("code_verifier", verifier)
	if cfg.ClientSecret != "" {
		form.Set("client_secret", cfg.ClientSecret)
	}
	return postToken(ctx, cfg.TokenURL, form)
}

func refreshToken(ctx context.Context, cfg Config, refresh string) (*Token, error) {
	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", refresh)
	form.Set("client_id", cfg.ClientID)
	if cfg.ClientSecret != "" {
		form.Set("client_secret", cfg.ClientSecret)
	}
	return postToken(ctx, cfg.TokenURL, form)
}

func postToken(ctx context.Context, tokenURL string, form url.Values) (*Token, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("oauth: token endpoint returned HTTP %d", resp.StatusCode)
	}
	var raw struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
		RefreshToken string `json:"refresh_token"`
		Scope        string `json:"scope"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	expSec := raw.ExpiresIn
	if expSec == 0 {
		expSec = 3600
	}
	var scopes []string
	if raw.Scope != "" {
		scopes = strings.Fields(raw.Scope)
	}
	return &Token{
		AccessToken:  raw.AccessToken,
		TokenType:    raw.TokenType,
		RefreshToken: raw.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(expSec) * time.Second),
		Scopes:       scopes,
	}, nil
}

// openBrowserFn is a variable so tests can replace it.
var openBrowserFn = openBrowser

func openBrowser(rawURL string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", rawURL)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", rawURL)
	default:
		cmd = exec.Command("xdg-open", rawURL)
	}
	return cmd.Start()
}

func randomState() string {
	b := make([]byte, 16)
	_, _ = io.ReadFull(rand.Reader, b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// stderrTarget is overridable for tests; defaults to os.Stderr.
var stderrTarget io.Writer = os.Stderr

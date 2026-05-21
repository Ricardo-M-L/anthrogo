# `github.com/ricardo/anthrogo/internal/oauth`

```go
package oauth // import "github.com/ricardo/anthrogo/internal/oauth"


FUNCTIONS

func GenerateChallenge() (verifier, challenge string, err error)
    GenerateChallenge returns (verifier, challenge) where challenge =
    base64url(sha256(verifier)) — implements PKCE S256 method per RFC 7636.

func SaveToken(cacheRoot, serverName string, t *Token) error
    SaveToken writes t to cacheRoot/<serverName>.json (mode 0600; dir 0700).


TYPES

type Config struct {
	AuthorizationURL string
	TokenURL         string
	ClientID         string
	ClientSecret     string // optional (PKCE-only public clients)
	Scopes           []string
	RedirectPort     int // 0 → 8765
}
    Config holds the OAuth 2.1 parameters for a single authorization server.

type Token struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"` // "Bearer"
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
	Scopes       []string  `json:"scopes,omitempty"`
}
    Token holds the result of a completed OAuth 2.1 authorization flow.

func FetchToken(ctx context.Context, cfg Config, cacheRoot, serverName string) (*Token, error)
    FetchToken returns a valid token for serverName, refreshing or launching
    the browser authorization-code flow as needed. Tokens are cached under
    cacheRoot.

func LoadToken(cacheRoot, serverName string) (*Token, error)
    LoadToken reads a cached token from cacheRoot/<serverName>.json. Returns
    (nil, nil) if the file does not exist.

func (t *Token) IsExpired() bool
    IsExpired reports whether the token has expired (with a 30s clock-skew
    margin).

```

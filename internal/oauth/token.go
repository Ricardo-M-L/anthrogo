package oauth

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// Token holds the result of a completed OAuth 2.1 authorization flow.
type Token struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"` // "Bearer"
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
	Scopes       []string  `json:"scopes,omitempty"`
}

// IsExpired reports whether the token has expired (with a 30s clock-skew margin).
func (t *Token) IsExpired() bool {
	return time.Now().After(t.ExpiresAt.Add(-30 * time.Second))
}

// LoadToken reads a cached token from cacheRoot/<serverName>.json.
// Returns (nil, nil) if the file does not exist.
func LoadToken(cacheRoot, serverName string) (*Token, error) {
	path := filepath.Join(cacheRoot, serverName+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var t Token
	if err := json.Unmarshal(raw, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// SaveToken writes t to cacheRoot/<serverName>.json (mode 0600; dir 0700).
func SaveToken(cacheRoot, serverName string, t *Token) error {
	if err := os.MkdirAll(cacheRoot, 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(cacheRoot, serverName+".json")
	return os.WriteFile(path, raw, 0o600)
}

package session

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"

	"github.com/ricardo/anthrogo/internal/config"
)

func ProjectDir(cwd string) (string, error) {
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return "", err
	}
	home, err := config.Home()
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256([]byte(abs))
	dir := filepath.Join(home, "projects", hex.EncodeToString(hash[:6]))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func SessionFile(cwd, sessionID string) (string, error) {
	dir, err := ProjectDir(cwd)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, sessionID+".jsonl"), nil
}

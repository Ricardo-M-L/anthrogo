package config

import (
	"os"
	"path/filepath"
)

// Home returns the anthrogo config directory: $ANTHROGO_HOME, else
// ~/.anthrogo, creating it on demand.
func Home() (string, error) {
	if h := os.Getenv("ANTHROGO_HOME"); h != "" {
		return ensureDir(h)
	}
	u, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return ensureDir(filepath.Join(u, ".anthrogo"))
}

func ensureDir(p string) (string, error) {
	if err := os.MkdirAll(p, 0o755); err != nil {
		return "", err
	}
	return p, nil
}

// SettingsPath returns the settings.yaml path inside Home.
func SettingsPath() (string, error) {
	h, err := Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, "settings.yaml"), nil
}

// SkillsDir returns the absolute path to <home>/.anthrogo/skills/. Pass the
// raw user home (os.UserHomeDir() or os.Getenv("HOME")); do NOT pass the
// already-resolved anthrogo home directory.
func SkillsDir(home string) string {
	return filepath.Join(home, ".anthrogo", "skills")
}

// SystemOverlayPath returns the path to the persistent user system prompt
// overlay file: <home>/.anthrogo/system_overlay.md. Pass the raw user home
// (os.UserHomeDir() or os.Getenv("HOME")).
func SystemOverlayPath(home string) string {
	return filepath.Join(home, ".anthrogo", "system_overlay.md")
}

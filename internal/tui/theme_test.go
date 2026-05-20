package tui

import (
	"testing"
)

func TestThemeByName_Dark(t *testing.T) {
	th, ok := ThemeByName("dark")
	if !ok {
		t.Fatal("expected dark theme to be found")
	}
	if th.Name != "dark" {
		t.Errorf("expected name 'dark', got %q", th.Name)
	}
}

func TestThemeByName_DarkEmpty(t *testing.T) {
	th, ok := ThemeByName("")
	if !ok {
		t.Fatal("expected empty string to resolve to dark theme")
	}
	if th.Name != "dark" {
		t.Errorf("expected name 'dark', got %q", th.Name)
	}
}

func TestThemeByName_Light(t *testing.T) {
	th, ok := ThemeByName("light")
	if !ok {
		t.Fatal("expected light theme to be found")
	}
	if th.Name != "light" {
		t.Errorf("expected name 'light', got %q", th.Name)
	}
}

func TestThemeByName_CaseInsensitive(t *testing.T) {
	th, ok := ThemeByName("LIGHT")
	if !ok {
		t.Fatal("expected case-insensitive lookup to work")
	}
	if th.Name != "light" {
		t.Errorf("expected name 'light', got %q", th.Name)
	}
}

func TestThemeByName_Unknown(t *testing.T) {
	_, ok := ThemeByName("solarized")
	if ok {
		t.Fatal("expected unknown theme to return false")
	}
}

func TestDefaultThemeIsDark(t *testing.T) {
	d := DefaultTheme()
	if d.Name != "dark" {
		t.Errorf("DefaultTheme should be dark, got %q", d.Name)
	}
}

func TestThemeNames(t *testing.T) {
	names := ThemeNames()
	if len(names) < 2 {
		t.Fatalf("expected at least 2 theme names, got %d", len(names))
	}
	found := map[string]bool{}
	for _, n := range names {
		found[n] = true
	}
	if !found["dark"] {
		t.Error("expected 'dark' in ThemeNames()")
	}
	if !found["light"] {
		t.Error("expected 'light' in ThemeNames()")
	}
}

func TestThemeFromConfig_Named(t *testing.T) {
	th := ThemeFromConfig("light", nil)
	if th.Name != "light" {
		t.Errorf("expected light theme, got %q", th.Name)
	}
}

func TestThemeFromConfig_Default(t *testing.T) {
	th := ThemeFromConfig("", nil)
	if th.Name != "dark" {
		t.Errorf("expected dark fallback, got %q", th.Name)
	}
}

func TestThemeFromConfig_Custom(t *testing.T) {
	overrides := map[string]string{
		"user_prompt": "#ff0000",
		"assistant":   "#00ff00",
	}
	th := ThemeFromConfig("custom", overrides)
	if th.Name != "custom" {
		t.Errorf("expected name 'custom', got %q", th.Name)
	}
}

func TestThemeFromConfig_CustomNoOverrides(t *testing.T) {
	// When name == "custom" but no overrides, should fall through to ThemeByName("custom")
	// which returns zero-value Theme (not found). So Name will be "".
	th := ThemeFromConfig("custom", nil)
	// ThemeByName("custom") returns false, so we get zero Theme.
	if th.Name != "" {
		t.Errorf("expected empty name for custom with no overrides, got %q", th.Name)
	}
}

package builtins

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTheme_List(t *testing.T) {
	h := newFakeHost()
	res, err := (Theme{}).Run(context.Background(), "list", h)
	require.NoError(t, err)
	require.Contains(t, res.Text, "dark")
	require.Contains(t, res.Text, "light")
	require.Contains(t, res.Text, "/theme set")
}

func TestTheme_ListDefault(t *testing.T) {
	h := newFakeHost()
	res, err := (Theme{}).Run(context.Background(), "", h)
	require.NoError(t, err)
	require.Contains(t, res.Text, "dark")
	require.Contains(t, res.Text, "light")
}

func TestTheme_Show_Unknown(t *testing.T) {
	h := newFakeHost()
	res, err := (Theme{}).Run(context.Background(), "show", h)
	require.NoError(t, err)
	require.Contains(t, res.Text, "current theme:")
	require.Contains(t, res.Text, "(unknown)")
}

func TestTheme_SetUnknown(t *testing.T) {
	h := newFakeHost()
	res, err := (Theme{}).Run(context.Background(), "set solarized", h)
	require.NoError(t, err)
	require.Contains(t, res.Text, "unknown theme")
	require.Contains(t, res.Text, "/theme list")
}

func TestTheme_InvalidSubcommand(t *testing.T) {
	h := newFakeHost()
	res, err := (Theme{}).Run(context.Background(), "frobnicate", h)
	require.NoError(t, err)
	require.Contains(t, res.Text, "usage")
}

// themeHost embeds fakeHost and implements SetTheme / ThemeName.
type themeHost struct {
	*fakeHost
	themeName  string
	setThemeFn func(string) error
}

func (th *themeHost) ThemeName() string { return th.themeName }
func (th *themeHost) SetTheme(name string) error {
	if th.setThemeFn != nil {
		return th.setThemeFn(name)
	}
	th.themeName = name
	return nil
}

func TestTheme_Set_DelegatesToHost(t *testing.T) {
	called := ""
	h := &themeHost{
		fakeHost:  newFakeHost(),
		themeName: "dark",
		setThemeFn: func(name string) error {
			called = name
			return nil
		},
	}
	res, err := (Theme{}).Run(context.Background(), "set light", h)
	require.NoError(t, err)
	require.Equal(t, "light", called)
	require.Contains(t, res.Text, "theme set to light")
}

func TestTheme_Show_WithHost(t *testing.T) {
	h := &themeHost{
		fakeHost:  newFakeHost(),
		themeName: "light",
	}
	res, err := (Theme{}).Run(context.Background(), "show", h)
	require.NoError(t, err)
	require.Contains(t, res.Text, "light")
}

func TestTheme_Set_NoSetterInterface(t *testing.T) {
	h := newFakeHost() // does not implement SetTheme
	res, err := (Theme{}).Run(context.Background(), "set dark", h)
	require.NoError(t, err)
	require.Contains(t, res.Text, "unsupported")
}

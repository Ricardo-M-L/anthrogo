package builtins

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ricardo/anthrogo/internal/version"
	"github.com/ricardo/anthrogo/pkg/selfupdate"
	"github.com/stretchr/testify/require"
)

// setVersionAPIBase overrides selfupdate.APIBase for the duration of a test.
func setVersionAPIBase(t *testing.T, base string) {
	t.Helper()
	orig := selfupdate.APIBase
	selfupdate.APIBase = base
	t.Cleanup(func() { selfupdate.APIBase = orig })
}

func TestVersion_NoCheck(t *testing.T) {
	h := newFakeHost()
	res, err := (Version{}).Run(context.Background(), "no-check", h)
	require.NoError(t, err)
	require.Contains(t, res.Text, version.Version)
	require.NotContains(t, res.Text, "update check")
	require.NotContains(t, res.Text, "Newer")
}

func TestVersion_UpdateAvailable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"tag_name":"v99.0.0","html_url":"https://github.com/Ricardo-M-L/anthrogo/releases/tag/v99.0.0","assets":[]}`)
	}))
	defer srv.Close()
	setVersionAPIBase(t, srv.URL)

	h := newFakeHost()
	res, err := (Version{}).Run(context.Background(), "", h)
	require.NoError(t, err)
	require.Contains(t, res.Text, "Newer release available")
	require.Contains(t, res.Text, "v99.0.0")
}

func TestVersion_UpToDate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Return the current version — not newer.
		fmt.Fprintf(w, `{"tag_name":"%s","html_url":"https://github.com/Ricardo-M-L/anthrogo/releases/tag/%s","assets":[]}`+"\n",
			version.Version, version.Version)
	}))
	defer srv.Close()
	setVersionAPIBase(t, srv.URL)

	h := newFakeHost()
	res, err := (Version{}).Run(context.Background(), "", h)
	require.NoError(t, err)
	require.Contains(t, res.Text, "Up to date")
}

func TestVersion_NetworkError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
	}))
	defer srv.Close()
	setVersionAPIBase(t, srv.URL)

	h := newFakeHost()
	res, err := (Version{}).Run(context.Background(), "", h)
	require.NoError(t, err) // graceful: error is embedded in result text
	require.Contains(t, res.Text, "update check")
}

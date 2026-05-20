package selfupdate

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func setAPIBase(t *testing.T, base string) {
	t.Helper()
	orig := APIBase
	APIBase = base
	t.Cleanup(func() { APIBase = orig })
}

func TestIsNewer_Cases(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"v0.11.0", "v0.10.7-dev", true},
		{"v0.10.7", "v0.10.7-dev", false}, // equal numeric
		{"v0.10.6", "v0.10.7-dev", false},
		{"v1.0.0", "v0.99.9", true},
		{"v0.10.7-dev", "v0.10.7", false},
		{"", "v0.10.7", false},
		{"v0.10.7", "", false},
	}
	for _, tc := range cases {
		require.Equal(t, tc.want, IsNewer(tc.latest, tc.current),
			"IsNewer(%q, %q)", tc.latest, tc.current)
	}
}

func TestLatestRelease_404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
	}))
	defer srv.Close()
	setAPIBase(t, srv.URL)

	_, err := LatestRelease(context.Background(), "does-not-exist/repo")
	require.Error(t, err)
	require.Contains(t, err.Error(), "no releases published yet")
}

func TestLatestRelease_NonOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(503)
	}))
	defer srv.Close()
	setAPIBase(t, srv.URL)

	_, err := LatestRelease(context.Background(), "some/repo")
	require.Error(t, err)
	require.Contains(t, err.Error(), "503")
}

func TestLatestRelease_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"tag_name":"v0.20.0","html_url":"https://github.com/Ricardo-M-L/anthrogo/releases/tag/v0.20.0","assets":[]}`)
	}))
	defer srv.Close()
	setAPIBase(t, srv.URL)

	rel, err := LatestRelease(context.Background(), "Ricardo-M-L/anthrogo")
	require.NoError(t, err)
	require.Equal(t, "v0.20.0", rel.TagName)
	require.Equal(t, "https://github.com/Ricardo-M-L/anthrogo/releases/tag/v0.20.0", rel.HTMLURL)
	require.Empty(t, rel.Assets)
}

func TestLatestRelease_DefaultRepo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Contains(t, r.URL.Path, "Ricardo-M-L/anthrogo")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"tag_name":"v1.0.0","html_url":"https://github.com/Ricardo-M-L/anthrogo/releases/tag/v1.0.0","assets":[]}`)
	}))
	defer srv.Close()
	setAPIBase(t, srv.URL)

	t.Setenv("ANTHROGO_RELEASE_REPO", "")
	rel, err := LatestRelease(context.Background(), "")
	require.NoError(t, err)
	require.Equal(t, "v1.0.0", rel.TagName)
}

func TestLatestRelease_EnvRepoOverride(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Contains(t, r.URL.Path, "custom/repo")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"tag_name":"v2.0.0","html_url":"https://github.com/custom/repo/releases/tag/v2.0.0","assets":[]}`)
	}))
	defer srv.Close()
	setAPIBase(t, srv.URL)

	t.Setenv("ANTHROGO_RELEASE_REPO", "custom/repo")
	rel, err := LatestRelease(context.Background(), "")
	require.NoError(t, err)
	require.Equal(t, "v2.0.0", rel.TagName)
}

func TestParseSemver(t *testing.T) {
	cases := []struct {
		in   string
		want []int
	}{
		{"v1.2.3", []int{1, 2, 3}},
		{"0.10.7", []int{0, 10, 7}},
		{"v0.10.7-dev", []int{0, 10, 7}},
		{"1.0.0-alpha", []int{1, 0, 0}},
		{"", nil},
		{"abc", nil},
		{"v", nil},
	}
	for _, tc := range cases {
		got := parseSemver(tc.in)
		require.Equal(t, tc.want, got, "parseSemver(%q)", tc.in)
	}
}

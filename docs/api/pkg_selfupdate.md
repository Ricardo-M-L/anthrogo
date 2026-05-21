# `github.com/ricardo/anthrogo/pkg/selfupdate`

```go
package selfupdate // import "github.com/ricardo/anthrogo/pkg/selfupdate"


VARIABLES

var APIBase = "https://api.github.com"
    APIBase is the GitHub API base URL. Override in tests to point at an
    httptest server; restore with defer after the test.


FUNCTIONS

func IsNewer(latest, current string) bool
    IsNewer reports whether latest is a strictly newer semver than current.
    Strips a leading "v" and any "-suffix" (e.g. "-dev") before comparing.
    Returns false if either string cannot be parsed.


TYPES

type Release struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
	Body    string `json:"body"`
	Assets  []struct {
		Name        string `json:"name"`
		DownloadURL string `json:"browser_download_url"`
		Size        int64  `json:"size"`
	} `json:"assets"`
}
    Release holds the fields we care about from a GitHub Releases API response.

func LatestRelease(ctx context.Context, repo string) (*Release, error)
    LatestRelease fetches the latest tag from GitHub Releases API. If repo
    is empty, ANTHROGO_RELEASE_REPO env var is consulted, falling back to
    "Ricardo-M-L/anthrogo". Set GITHUB_TOKEN for higher rate limits.

```

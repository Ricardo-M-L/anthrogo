package selfupdate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// APIBase is the GitHub API base URL. Override in tests to point at an httptest
// server; restore with defer after the test.
var APIBase = "https://api.github.com"

// Release holds the fields we care about from a GitHub Releases API response.
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

// LatestRelease fetches the latest tag from GitHub Releases API.
// If repo is empty, ANTHROGO_RELEASE_REPO env var is consulted, falling back
// to "Ricardo-M-L/anthrogo". Set GITHUB_TOKEN for higher rate limits.
func LatestRelease(ctx context.Context, repo string) (*Release, error) {
	if repo == "" {
		repo = os.Getenv("ANTHROGO_RELEASE_REPO")
		if repo == "" {
			repo = "Ricardo-M-L/anthrogo"
		}
	}
	url := fmt.Sprintf("%s/repos/%s/releases/latest", APIBase, repo)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("no releases published yet for %s", repo)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub HTTP %d", resp.StatusCode)
	}
	var rel Release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

// IsNewer reports whether latest is a strictly newer semver than current.
// Strips a leading "v" and any "-suffix" (e.g. "-dev") before comparing.
// Returns false if either string cannot be parsed.
func IsNewer(latest, current string) bool {
	lp := parseSemver(latest)
	cp := parseSemver(current)
	if lp == nil || cp == nil {
		return false
	}
	for i := 0; i < 3; i++ {
		if lp[i] > cp[i] {
			return true
		}
		if lp[i] < cp[i] {
			return false
		}
	}
	return false
}

// parseSemver parses "vX.Y.Z" or "X.Y.Z" (with an optional "-suffix") into a
// [3]int slice. Returns nil on parse failure.
func parseSemver(s string) []int {
	s = strings.TrimPrefix(s, "v")
	// Strip everything after the first non-digit, non-dot character.
	cut := len(s)
	for i, c := range s {
		if (c < '0' || c > '9') && c != '.' {
			cut = i
			break
		}
	}
	s = s[:cut]
	parts := strings.Split(s, ".")
	if len(parts) < 1 || len(parts) > 3 {
		return nil
	}
	out := make([]int, 3)
	for i, p := range parts {
		if p == "" {
			return nil
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil
		}
		out[i] = n
	}
	return out
}

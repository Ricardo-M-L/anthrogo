package system

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LoadClaudeMd walks from `start` upward, stopping at (and including) `stopAt`.
// Returns the concatenation of every CLAUDE.md found, root-first, separated by
// a header that names the source file.
//
// `stopAt` is typically the user's home directory; pass an empty string to walk
// all the way to the filesystem root.
func LoadClaudeMd(start, stopAt string) (string, error) {
	absStart, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	var absStop string
	if stopAt != "" {
		absStop, err = filepath.Abs(stopAt)
		if err != nil {
			return "", err
		}
	}

	var paths []string
	cur := absStart
	for {
		p := filepath.Join(cur, "CLAUDE.md")
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			paths = append(paths, p)
		}
		if cur == absStop {
			break
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}

	// Reverse to root-first
	for i, j := 0, len(paths)-1; i < j; i, j = i+1, j-1 {
		paths[i], paths[j] = paths[j], paths[i]
	}

	var b strings.Builder
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, "<!-- from %s -->\n", p)
		b.Write(raw)
		if !strings.HasSuffix(string(raw), "\n") {
			b.WriteByte('\n')
		}
	}
	return b.String(), nil
}

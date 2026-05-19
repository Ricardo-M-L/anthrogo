package tool

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

type Grep struct{ DefaultPermission }

func (Grep) Name() string                      { return "Grep" }
func (Grep) Description(context.Context) string { return grepDescription }
func (Grep) UserFacingName(input map[string]any) string {
	if p, ok := input["pattern"].(string); ok {
		return "Grep " + p
	}
	return "Grep"
}
func (Grep) IsReadOnly() bool        { return true }
func (Grep) IsConcurrencySafe() bool { return true }

func (Grep) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pattern": map[string]any{"type": "string", "description": "Go regexp."},
			"path":    map[string]any{"type": "string", "description": "Directory to search (default cwd)."},
			"glob":    map[string]any{"type": "string", "description": "Optional file-path glob filter."},
			"output_mode": map[string]any{
				"type": "string", "enum": []string{"files_with_matches", "content", "count"},
				"default": "files_with_matches",
			},
			"-i": map[string]any{"type": "boolean", "description": "Case-insensitive."},
			"-n": map[string]any{"type": "boolean", "description": "Include line numbers."},
		},
		"required": []string{"pattern"},
	}
}

func (Grep) Call(ctx context.Context, input map[string]any, tcx *Context) (Result, error) {
	pattern, _ := input["pattern"].(string)
	if pattern == "" {
		return errResult("pattern is required"), nil
	}
	base, _ := input["path"].(string)
	if base == "" {
		if tcx != nil && tcx.Cwd != "" {
			base = tcx.Cwd
		} else {
			base, _ = os.Getwd()
		}
	}
	pathGlob, _ := input["glob"].(string)
	mode, _ := input["output_mode"].(string)
	if mode == "" {
		mode = "files_with_matches"
	}
	caseInsens, _ := input["-i"].(bool)
	withLines, _ := input["-n"].(bool)

	rePat := pattern
	if caseInsens {
		rePat = "(?i)" + rePat
	}
	re, err := regexp.Compile(rePat)
	if err != nil {
		return errResult("invalid regexp: " + err.Error()), nil
	}

	type hit struct {
		file string
		line int
		text string
	}
	var hits []hit
	var files []string
	counts := map[string]int{}

	walkErr := filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(base, p)
		if pathGlob != "" {
			ok, _ := doublestar.PathMatch(pathGlob, rel)
			if !ok {
				return nil
			}
		}
		f, err := os.Open(p)
		if err != nil {
			return nil
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 1024*1024), 4*1024*1024)
		line := 0
		matched := false
		for sc.Scan() {
			line++
			if re.MatchString(sc.Text()) {
				matched = true
				counts[rel]++
				if mode == "content" {
					hits = append(hits, hit{file: rel, line: line, text: sc.Text()})
				}
			}
		}
		if matched && mode != "content" {
			files = append(files, rel)
		}
		return nil
	})
	if walkErr != nil {
		return errResult(walkErr.Error()), nil
	}

	var out strings.Builder
	switch mode {
	case "files_with_matches":
		for _, f := range files {
			out.WriteString(f)
			out.WriteByte('\n')
		}
	case "content":
		for _, h := range hits {
			if withLines {
				fmt.Fprintf(&out, "%s:%d:%s\n", h.file, h.line, h.text)
			} else {
				fmt.Fprintf(&out, "%s:%s\n", h.file, h.text)
			}
		}
	case "count":
		files := make([]string, 0, len(counts))
		for f := range counts {
			files = append(files, f)
		}
		sort.Strings(files)
		for _, f := range files {
			fmt.Fprintf(&out, "%s:%d\n", f, counts[f])
		}
	}
	if out.Len() == 0 {
		return Result{Type: ResultText, Text: "no matches", ForLLM: "no matches"}, nil
	}
	return Result{Type: ResultText, Text: out.String(), ForLLM: out.String()}, nil
}

const grepDescription = `Search files recursively with a Go regexp. output_mode: files_with_matches (default), content (with -n for line numbers), or count. Use glob to filter paths and -i for case-insensitive matching. Skips .git directories.`

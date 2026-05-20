package tool

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// References finds all usages (word-boundary occurrences) of a symbol name
// across the directory tree.
type References struct{ DefaultPermission }

func (References) Name() string                      { return "References" }
func (References) Description(context.Context) string { return referencesDescription }
func (References) UserFacingName(input map[string]any) string {
	if n, ok := input["name"].(string); ok {
		return "References " + n
	}
	return "References"
}
func (References) IsReadOnly() bool        { return true }
func (References) IsConcurrencySafe() bool { return true }

func (References) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{
				"type": "string",
			},
			"path": map[string]any{
				"type": "string",
			},
		},
		"required": []string{"name"},
	}
}

// isBinaryHeuristic reads up to 512 bytes from a file and returns true if a
// NUL byte is found, which strongly suggests binary content.
func isBinaryHeuristic(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if err != nil && n == 0 {
		return false
	}
	for _, b := range buf[:n] {
		if b == 0x00 {
			return true
		}
	}
	return false
}

func (References) Call(ctx context.Context, input map[string]any, tcx *Context) (Result, error) {
	name, _ := input["name"].(string)
	if name == "" {
		return errResult("name is required"), nil
	}
	base, _ := input["path"].(string)
	if base == "" {
		if tcx != nil && tcx.Cwd != "" {
			base = tcx.Cwd
		} else {
			base, _ = os.Getwd()
		}
	}

	// Build word-boundary regex for the name.
	pattern := `\b` + regexp.QuoteMeta(name) + `\b`
	re, err := regexp.Compile(pattern)
	if err != nil {
		return errResult("could not compile pattern: " + err.Error()), nil
	}

	const maxMatches = 200
	const maxFileSize = 1024 * 1024 // 1 MB

	type refHit struct {
		path string
		line int
		col  int
		text string
	}
	var hits []refHit

	walkErr := filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skipDirSymbol(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if len(hits) >= maxMatches {
			return nil
		}

		// Check file size.
		info, infoErr := d.Info()
		if infoErr != nil {
			return nil
		}
		if info.Size() > maxFileSize {
			return nil
		}

		// Skip binary files.
		if isBinaryHeuristic(p) {
			return nil
		}

		f, openErr := os.Open(p)
		if openErr != nil {
			return nil
		}
		defer f.Close()

		sc := bufio.NewScanner(f)
		lineNo := 0
		for sc.Scan() {
			if len(hits) >= maxMatches {
				break
			}
			lineNo++
			text := sc.Text()
			locs := re.FindAllStringIndex(text, -1)
			for _, loc := range locs {
				col := loc[0] + 1 // 1-indexed
				hits = append(hits, refHit{
					path: p,
					line: lineNo,
					col:  col,
					text: text,
				})
				if len(hits) >= maxMatches {
					break
				}
			}
		}
		return nil
	})
	if walkErr != nil {
		return errResult(walkErr.Error()), nil
	}

	if len(hits) == 0 {
		return Result{Type: ResultText, Text: "no references found", ForLLM: "no references found"}, nil
	}

	var sb strings.Builder
	for _, h := range hits {
		fmt.Fprintf(&sb, "%s:%d:%d: %s\n", h.path, h.line, h.col, strings.TrimSpace(h.text))
	}
	out := sb.String()
	return Result{Type: ResultText, Text: out, ForLLM: out}, nil
}

const referencesDescription = `Find all usages of a symbol name via word-boundary regex (\b<name>\b) across the directory tree. Returns up to 200 hits as "path:line:col: matched-line". Skips binary files (heuristic: any NUL byte in first 512 bytes), files larger than 1MB, and vendor/, node_modules/, .git/, .anthrogo/ directories. "path" defaults to cwd.`

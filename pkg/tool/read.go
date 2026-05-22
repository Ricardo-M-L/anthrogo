package tool

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Read is the M1 file reader. Mirrors src/tools/FileReadTool but without
// notebook/image branches (deferred to M2/M3).
type Read struct{ DefaultPermission }

func (Read) Name() string                       { return "Read" }
func (Read) Description(context.Context) string { return readDescription }
func (Read) UserFacingName(input map[string]any) string {
	if p, ok := input["file_path"].(string); ok {
		return "Read " + p
	}
	return "Read"
}
func (Read) IsReadOnly() bool        { return true }
func (Read) IsConcurrencySafe() bool { return true }

func (Read) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"file_path": map[string]any{"type": "string", "description": "Absolute path to file."},
			"offset":    map[string]any{"type": "integer", "description": "1-indexed start line."},
			"limit":     map[string]any{"type": "integer", "description": "Max lines to read."},
		},
		"required": []string{"file_path"},
	}
}

func (Read) Call(ctx context.Context, input map[string]any, _ *Context) (Result, error) {
	path, ok := input["file_path"].(string)
	if !ok || path == "" {
		return errResult("file_path is required"), nil
	}
	offset := intField(input, "offset", 1)
	limit := intField(input, "limit", 2000)

	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errResult("file does not exist: " + path), nil
		}
		return errResult(err.Error()), nil
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 4*1024*1024)
	var out strings.Builder
	lineNo := 0
	emitted := 0
	for sc.Scan() {
		lineNo++
		if lineNo < offset {
			continue
		}
		if emitted >= limit {
			break
		}
		fmt.Fprintf(&out, "%6d\t%s\n", lineNo, sc.Text())
		emitted++
	}
	if err := sc.Err(); err != nil {
		return errResult(err.Error()), nil
	}
	return Result{Type: ResultText, Text: out.String(), ForLLM: out.String()}, nil
}

func intField(input map[string]any, key string, def int) int {
	if v, ok := input[key]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		}
	}
	return def
}

func errResult(msg string) Result {
	return Result{Type: ResultText, Text: msg, ForLLM: msg, IsError: true}
}

// errIfNotAbs validates that a tool's path-typed input is absolute. Returns
// (errResult, true) to be passed back directly when invalid; (Result{}, false)
// when valid. Used by PDFRead, XlsxRead, BrowserAction (screenshot) etc. to
// reject LLM-supplied relative paths that would silently resolve against
// the process CWD.
func errIfNotAbs(field, path string) (Result, bool) {
	if !filepath.IsAbs(path) {
		return errResult(field + " must be an absolute path"), true
	}
	return Result{}, false
}

const readDescription = `Read a file from the local filesystem. Returns up to 2000 lines starting from line 1; use offset/limit for paging. Each line is prefixed with its 1-indexed line number and a tab.`

package tool

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// SymbolSearch finds a symbol's definition site by name.
// Go files are parsed via go/parser; other languages use regex heuristics.
type SymbolSearch struct{ DefaultPermission }

func (SymbolSearch) Name() string                       { return "SymbolSearch" }
func (SymbolSearch) Description(context.Context) string { return symbolSearchDescription }
func (SymbolSearch) UserFacingName(input map[string]any) string {
	if n, ok := input["name"].(string); ok {
		return "SymbolSearch " + n
	}
	return "SymbolSearch"
}
func (SymbolSearch) IsReadOnly() bool        { return true }
func (SymbolSearch) IsConcurrencySafe() bool { return true }

func (SymbolSearch) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "Symbol name (function, type, variable, const).",
			},
			"kind": map[string]any{
				"type":        "string",
				"description": "Optional filter: func | method | type | var | const | any (default any). For Go: method matches receiver-bound funcs only; func matches both bare funcs and methods.",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "Optional directory to search (default cwd).",
			},
		},
		"required": []string{"name"},
	}
}

type symbolHit struct {
	Path string
	Line int
	Text string
}

// skipDirSymbol returns true for directories that should be skipped during
// symbol search.
func skipDirSymbol(name string) bool {
	switch name {
	case "vendor", "node_modules", ".git", ".anthrogo":
		return true
	}
	return false
}

func (SymbolSearch) Call(ctx context.Context, input map[string]any, tcx *Context) (Result, error) {
	name, _ := input["name"].(string)
	if name == "" {
		return errResult("name is required"), nil
	}
	kind, _ := input["kind"].(string)
	if kind == "" {
		kind = "any"
	}
	base, _ := input["path"].(string)
	if base == "" {
		if tcx != nil && tcx.Cwd != "" {
			base = tcx.Cwd
		} else {
			base, _ = os.Getwd()
		}
	}

	const maxHits = 50
	var hits []symbolHit

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
		if len(hits) >= maxHits {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(p))
		switch ext {
		case ".go":
			goHits := searchGoFile(p, name, kind)
			for _, h := range goHits {
				if len(hits) >= maxHits {
					break
				}
				hits = append(hits, h)
			}
		case ".js", ".ts", ".tsx", ".jsx":
			langHits := searchFileRegex(p, name, kind, langPatternJS)
			for _, h := range langHits {
				if len(hits) >= maxHits {
					break
				}
				hits = append(hits, h)
			}
		case ".py":
			langHits := searchFileRegex(p, name, kind, langPatternPython)
			for _, h := range langHits {
				if len(hits) >= maxHits {
					break
				}
				hits = append(hits, h)
			}
		case ".rs":
			langHits := searchFileRegex(p, name, kind, langPatternRust)
			for _, h := range langHits {
				if len(hits) >= maxHits {
					break
				}
				hits = append(hits, h)
			}
		case ".rb":
			langHits := searchFileRegex(p, name, kind, langPatternRuby)
			for _, h := range langHits {
				if len(hits) >= maxHits {
					break
				}
				hits = append(hits, h)
			}
		}
		return nil
	})
	if walkErr != nil {
		return errResult(walkErr.Error()), nil
	}

	if len(hits) == 0 {
		return Result{Type: ResultText, Text: "no matches", ForLLM: "no matches"}, nil
	}

	var sb strings.Builder
	for _, h := range hits {
		fmt.Fprintf(&sb, "%s:%d: %s\n", h.Path, h.Line, strings.TrimSpace(h.Text))
	}
	out := sb.String()
	return Result{Type: ResultText, Text: out, ForLLM: out}, nil
}

// searchGoFile parses a .go file using go/parser and returns hits matching
// the given symbol name and kind filter.
func searchGoFile(path, name, kind string) []symbolHit {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil,
		parser.SkipObjectResolution|parser.DeclarationErrors)
	if err != nil && f == nil {
		return nil
	}
	if f == nil {
		return nil
	}

	src, readErr := os.ReadFile(path)
	if readErr != nil {
		return nil
	}
	lines := bytes.Split(src, []byte("\n"))

	var hits []symbolHit
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Name == nil {
				continue
			}
			if d.Name.Name != name {
				continue
			}
			kindLabel := "func"
			if d.Recv != nil && len(d.Recv.List) > 0 {
				kindLabel = "method"
			}
			// kind filter: "method" matches only methods; "func" matches both
			// bare funcs and methods (common user intent); "any" matches all.
			switch kind {
			case "method":
				if kindLabel != "method" {
					continue
				}
			case "func":
				// accept both func and method
			case "any":
				// accept all
			default:
				continue
			}
			pos := fset.Position(d.Name.Pos())
			lineText := ""
			if pos.Line > 0 && pos.Line <= len(lines) {
				lineText = string(lines[pos.Line-1])
			}
			hits = append(hits, symbolHit{Path: path, Line: pos.Line, Text: lineText})

		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					if s.Name == nil || s.Name.Name != name {
						continue
					}
					if kind != "any" && kind != "type" {
						continue
					}
					pos := fset.Position(s.Name.Pos())
					lineText := ""
					if pos.Line > 0 && pos.Line <= len(lines) {
						lineText = string(lines[pos.Line-1])
					}
					hits = append(hits, symbolHit{Path: path, Line: pos.Line, Text: lineText})

				case *ast.ValueSpec:
					for _, ident := range s.Names {
						if ident == nil || ident.Name != name {
							continue
						}
						wantKind := ""
						switch d.Tok {
						case token.CONST:
							wantKind = "const"
						case token.VAR:
							wantKind = "var"
						}
						if kind != "any" && wantKind != "" && kind != wantKind {
							continue
						}
						pos := fset.Position(ident.Pos())
						lineText := ""
						if pos.Line > 0 && pos.Line <= len(lines) {
							lineText = string(lines[pos.Line-1])
						}
						hits = append(hits, symbolHit{Path: path, Line: pos.Line, Text: lineText})
					}
				}
			}
		}
	}
	return hits
}

// langPattern bundles the regex patterns for a language.
type langPattern struct {
	// funcRe matches function / method definitions.
	funcRe *regexp.Regexp
	// typeRe matches type definitions.
	typeRe *regexp.Regexp
	// varRe matches variable definitions.
	varRe *regexp.Regexp
	// constRe matches constant definitions.
	constRe *regexp.Regexp
}

func buildLangPattern(funcPat, typePat, varPat, constPat string) langPattern {
	compile := func(pat string) *regexp.Regexp {
		if pat == "" {
			return nil
		}
		r, err := regexp.Compile(pat)
		if err != nil {
			return nil
		}
		return r
	}
	return langPattern{
		funcRe:  compile(funcPat),
		typeRe:  compile(typePat),
		varRe:   compile(varPat),
		constRe: compile(constPat),
	}
}

// Pre-built language patterns (substituted with actual symbol name at call time).

var (
	// JS/TS patterns will be built dynamically per-call because the name is variable.
	// We store format strings and compile at search time.
	langPatternJS     = langPattern{} // populated below
	langPatternPython = langPattern{}
	langPatternRust   = langPattern{}
	langPatternRuby   = langPattern{}
)

// buildJSPattern builds JS/TS patterns for a specific name.
func buildJSPattern(name string) langPattern {
	n := regexp.QuoteMeta(name)
	return buildLangPattern(
		`(?:function|(?:const|let|var)\s+`+n+`\s*=\s*(?:async\s+)?(?:function|\())\s*`+n+`?\b|\bfunction\s+`+n+`\b`,
		`(?:class|type|interface)\s+`+n+`\b`,
		`(?:const|let|var)\s+`+n+`\b`,
		"",
	)
}

func buildPythonPattern(name string) langPattern {
	n := regexp.QuoteMeta(name)
	return buildLangPattern(
		`(?:^|\s)def\s+`+n+`\b`,
		`(?:^|\s)class\s+`+n+`\b`,
		"",
		"",
	)
}

func buildRustPattern(name string) langPattern {
	n := regexp.QuoteMeta(name)
	return buildLangPattern(
		`\bfn\s+`+n+`\b`,
		`\b(?:struct|enum|trait|type)\s+`+n+`\b`,
		`\bstatic\s+(?:mut\s+)?`+n+`\b`,
		`\bconst\s+`+n+`\b`,
	)
}

func buildRubyPattern(name string) langPattern {
	n := regexp.QuoteMeta(name)
	return buildLangPattern(
		`\bdef\s+`+n+`\b`,
		`\b(?:class|module)\s+`+n+`\b`,
		"",
		"",
	)
}

// searchFileRegex searches a non-Go file for a symbol using regex patterns.
// The langPattern argument is ignored; patterns are built from name/kind directly.
func searchFileRegex(path, name, kind string, _ langPattern) []symbolHit {
	ext := strings.ToLower(filepath.Ext(path))
	var lp langPattern
	switch ext {
	case ".js", ".ts", ".tsx", ".jsx":
		lp = buildJSPattern(name)
	case ".py":
		lp = buildPythonPattern(name)
	case ".rs":
		lp = buildRustPattern(name)
	case ".rb":
		lp = buildRubyPattern(name)
	default:
		return nil
	}

	// Select which regexes to apply based on kind filter.
	var patterns []*regexp.Regexp
	switch kind {
	case "func":
		if lp.funcRe != nil {
			patterns = append(patterns, lp.funcRe)
		}
	case "type":
		if lp.typeRe != nil {
			patterns = append(patterns, lp.typeRe)
		}
	case "var":
		if lp.varRe != nil {
			patterns = append(patterns, lp.varRe)
		}
	case "const":
		if lp.constRe != nil {
			patterns = append(patterns, lp.constRe)
		}
	default: // "any"
		for _, r := range []*regexp.Regexp{lp.funcRe, lp.typeRe, lp.varRe, lp.constRe} {
			if r != nil {
				patterns = append(patterns, r)
			}
		}
	}
	if len(patterns) == 0 {
		return nil
	}

	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var hits []symbolHit
	sc := bufio.NewScanner(f)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		text := sc.Text()
		for _, re := range patterns {
			if re.MatchString(text) {
				hits = append(hits, symbolHit{Path: path, Line: lineNo, Text: text})
				break // don't double-report same line
			}
		}
	}
	return hits
}

const symbolSearchDescription = `Find a symbol's definition site by name. Go files parsed via go/parser for accurate position + kind classification (func/method/type/var/const). Receiver-bound functions are classified as "method"; bare functions as "func". kind=method matches methods only; kind=func matches both bare funcs and methods. Other languages (.js/.ts/.tsx/.py/.rs/.rb) use language-specific regex heuristics. Returns up to 50 hits as "path:line: matched-line". Use "kind" to filter by func, method, type, var, or const. "path" defaults to cwd. Skips vendor/, node_modules/, .git/, .anthrogo/.`

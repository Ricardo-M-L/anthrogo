package bashscan

import (
	"io"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// Result describes a parsed command's surface: the set of binaries it
// invokes and the first argument to each, plus structural flags.
type Result struct {
	Binaries        []string // every command name (e.g. ["rm", "git"])
	UsesSudo        bool
	UsesPipeOrChain bool   // contains |, &&, ||
	UsesRedirect    bool   // >, >>, <
	UsesSubshell    bool   // (...) or backticks
	ParseError      string // non-empty if syntax.Parse failed
}

// Scan parses one or more shell commands (`bash -c <script>` style).
// On parse error, ParseError is set; partial info may still populate Binaries
// if the parser recovered.
func Scan(script string) *Result {
	res := &Result{}
	parser := syntax.NewParser(syntax.KeepComments(false))
	file, err := parser.Parse(strings.NewReader(script), "command")
	if err != nil {
		res.ParseError = err.Error()
		return res
	}
	syntax.Walk(file, func(node syntax.Node) bool {
		switch n := node.(type) {
		case *syntax.CallExpr:
			if len(n.Args) == 0 {
				return true
			}
			// Reconstruct the first arg literally
			head := litString(n.Args[0])
			if head != "" {
				res.Binaries = append(res.Binaries, head)
				if head == "sudo" || head == "doas" {
					res.UsesSudo = true
					// Also extract the effective command that sudo delegates to.
					// sudo flags start with '-'; skip them to find the real binary.
					for _, arg := range n.Args[1:] {
						name := litString(arg)
						if name == "" {
							break
						}
						if len(name) > 0 && name[0] == '-' {
							continue // skip sudo flags like -u, -E, etc.
						}
						res.Binaries = append(res.Binaries, name)
						break
					}
				}
			}
		case *syntax.BinaryCmd:
			// op is And (&&), Or (||), Pipe (|), PipeAll (|&)
			res.UsesPipeOrChain = true
		case *syntax.Redirect:
			res.UsesRedirect = true
		case *syntax.Subshell, *syntax.CmdSubst:
			res.UsesSubshell = true
		}
		return true
	})
	return res
}

// litString attempts to render a syntax.Word as a literal string. Returns
// empty when the word is not a pure literal (e.g. `$(cmd)`, "$VAR").
func litString(w *syntax.Word) string {
	if w == nil || len(w.Parts) == 0 {
		return ""
	}
	var b strings.Builder
	for _, part := range w.Parts {
		switch p := part.(type) {
		case *syntax.Lit:
			b.WriteString(p.Value)
		case *syntax.SglQuoted:
			b.WriteString(p.Value)
		case *syntax.DblQuoted:
			// Best effort: concatenate Lit children only
			for _, qp := range p.Parts {
				if l, ok := qp.(*syntax.Lit); ok {
					b.WriteString(l.Value)
				}
			}
		}
	}
	return b.String()
}

// HasBinary reports whether scanResult invokes any of the named binaries.
func (r *Result) HasBinary(names ...string) bool {
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	for _, b := range r.Binaries {
		if set[b] {
			return true
		}
	}
	return false
}

// _ keeps the io import in case we add file-based parsing later.
var _ = io.Discard

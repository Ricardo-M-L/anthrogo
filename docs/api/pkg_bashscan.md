# `github.com/ricardo/anthrogo/pkg/bashscan`

```go
package bashscan // import "github.com/ricardo/anthrogo/pkg/bashscan"


TYPES

type Result struct {
	Binaries        []string // every command name (e.g. ["rm", "git"])
	UsesSudo        bool
	UsesPipeOrChain bool   // contains |, &&, ||
	UsesRedirect    bool   // >, >>, <
	UsesSubshell    bool   // (...) or backticks
	ParseError      string // non-empty if syntax.Parse failed
}
    Result describes a parsed command's surface: the set of binaries it invokes
    and the first argument to each, plus structural flags.

func Scan(script string) *Result
    Scan parses one or more shell commands (`bash -c <script>` style). On parse
    error, ParseError is set; partial info may still populate Binaries if the
    parser recovered.

func (r *Result) HasBinary(names ...string) bool
    HasBinary reports whether scanResult invokes any of the named binaries.

```

# `github.com/ricardo/anthrogo/internal/doctor`

```go
package doctor // import "github.com/ricardo/anthrogo/internal/doctor"


FUNCTIONS

func Format(checks []Check) string
    Format renders checks as a human-readable report.


TYPES

type Check struct {
	Name        string
	Severity    Severity
	Message     string
	Remediation string
}
    Check holds the result of a single health check.

func RunAll(ctx context.Context, cfg config.Config) []Check
    RunAll executes all health checks and returns their results.

type Severity string
    Severity describes the outcome level of a health check.

const (
	SeverityPass Severity = "PASS"
	SeverityWarn Severity = "WARN"
	SeverityFail Severity = "FAIL"
)
```

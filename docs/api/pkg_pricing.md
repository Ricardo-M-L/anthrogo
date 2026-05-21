# `github.com/ricardo/anthrogo/pkg/pricing`

```go
package pricing // import "github.com/ricardo/anthrogo/pkg/pricing"


FUNCTIONS

func DefaultRates() map[string]Rate
    DefaultRates returns the built-in per-million-token USD prices for major
    models. Source: each provider's published pricing as of 2026-05; users
    override via the `pricing:` YAML stanza if they have negotiated rates.

    Keys use glob syntax (filepath.Match) so version variants match without
    explicit listings.

func EstimateUSD(rate Rate, inputTokens, outputTokens int) float64
    EstimateUSD returns the estimated dollar cost of (inputTokens, outputTokens)
    at the given rate.

func MergeWithUserRates(user map[string]Rate) map[string]Rate
    MergeWithUserRates returns DefaultRates() with user-supplied overrides
    merged on top. User keys win on exact-name collisions; glob-vs-exact
    resolution is left to Table.Lookup (exact always beats glob).


TYPES

type Rate struct {
	InputPerM  float64
	OutputPerM float64
}
    Rate holds the USD price per one million tokens for a model.

type Table struct {
	// Has unexported fields.
}
    Table maps model names (exact or glob) to rates.

func NewTable(rates map[string]Rate) *Table
    NewTable constructs a Table from the given rates map. Keys may be
    exact model names or glob patterns (filepath.Match syntax, e.g.
    "claude-haiku-4-5-*").

func (t *Table) Lookup(model string) (Rate, bool)
    Lookup finds the best matching Rate for model. Exact match wins; otherwise
    glob match (filepath.Match) iterating in alphabetical key order for
    stability. Returns (zero Rate, false) if no match.

```

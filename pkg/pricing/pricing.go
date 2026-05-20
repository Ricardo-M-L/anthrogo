package pricing

import (
	"path/filepath"
	"sort"
	"strings"
)

// Rate holds the USD price per one million tokens for a model.
type Rate struct {
	InputPerM  float64
	OutputPerM float64
}

// Table maps model names (exact or glob) to rates.
type Table struct {
	rates map[string]Rate
}

// NewTable constructs a Table from the given rates map. Keys may be exact model
// names or glob patterns (filepath.Match syntax, e.g. "claude-haiku-4-5-*").
func NewTable(rates map[string]Rate) *Table {
	return &Table{rates: rates}
}

// Lookup finds the best matching Rate for model. Exact match wins; otherwise
// glob match (filepath.Match) iterating in alphabetical key order for stability.
// Returns (zero Rate, false) if no match.
func (t *Table) Lookup(model string) (Rate, bool) {
	if r, ok := t.rates[model]; ok {
		return r, true
	}
	var keys []string
	for k := range t.rates {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !strings.Contains(k, "*") && !strings.Contains(k, "?") {
			continue
		}
		if ok, _ := filepath.Match(k, model); ok {
			return t.rates[k], true
		}
	}
	return Rate{}, false
}

// EstimateUSD returns the estimated dollar cost of (inputTokens, outputTokens)
// at the given rate.
func EstimateUSD(rate Rate, inputTokens, outputTokens int) float64 {
	return rate.InputPerM*float64(inputTokens)/1_000_000.0 +
		rate.OutputPerM*float64(outputTokens)/1_000_000.0
}

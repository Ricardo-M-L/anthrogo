package pricing

import "strings"

// ContextWindow returns the model's input-context-token capacity. The lookup
// is best-effort: exact match first, then prefix match against known model
// families. Returns 0 when the model is unknown — callers should treat 0 as
// "unknown" and skip the % display.
//
// Values are taken from official provider documentation; refresh occasionally.
func ContextWindow(model string) int {
	if v, ok := contextWindows[model]; ok {
		return v
	}
	for prefix, v := range contextPrefixes {
		if strings.HasPrefix(model, prefix) {
			return v
		}
	}
	return 0
}

// Exact-match table for current named models.
var contextWindows = map[string]int{
	// Anthropic
	"claude-opus-4-7":   200000,
	"claude-sonnet-4-6": 200000,
	"claude-haiku-4-5":  200000,
	"claude-sonnet-4-5": 200000,
	"claude-opus-4-6":   200000,
	"claude-opus-4":     200000,
	"claude-sonnet-4":   200000,
	"claude-3-5-sonnet": 200000,
	"claude-3-5-haiku":  200000,

	// OpenAI compatible (Gemini via openai endpoint)
	"gemini-2.5-flash":         1048576,
	"gemini-2.5-flash-lite":    1048576,
	"gemini-2.5-pro":           2097152,
	"gemini-2.0-flash":         1048576,
	"gemini-flash-latest":      1048576,
	"gemini-flash-lite-latest": 1048576,
	"gemini-3-flash-preview":   1048576,
	"gemini-3-pro-preview":     2097152,

	// DeepSeek
	"deepseek-chat":     65536,
	"deepseek-reasoner": 65536,
	"deepseek-v4-pro":   131072,

	// Kimi / Moonshot
	"kimi-k2-0905-preview": 256000,
	"kimi-k2.6":            256000,

	// GLM / Zhipu
	"glm-4.6":          128000,
	"glm-zero-preview": 32000,
	"glm-5.1":          200000,

	// MiniMax
	"minimax-m2.7":  256000,
	"MiniMax-M2":    256000,
	"abab6.5s-chat": 245760,
}

// Prefix table for less precise fallback. Listed longest-prefix first.
var contextPrefixes = map[string]int{
	"claude-opus-":   200000,
	"claude-sonnet-": 200000,
	"claude-haiku-":  200000,
	"claude-3-":      200000,
	"gemini-3":       1048576,
	"gemini-2.5":     1048576,
	"gemini-2.0":     1048576,
	"gemini-flash":   1048576,
	"deepseek":       65536,
	"kimi":           256000,
	"glm":            128000,
	"minimax":        256000,
	"MiniMax":        256000,
}

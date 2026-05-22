package pricing

// DefaultRates returns the built-in per-million-token USD prices for major
// models. Source: each provider's published pricing as of 2026-05; users
// override via the `pricing:` YAML stanza if they have negotiated rates.
//
// Keys use glob syntax (filepath.Match) so version variants match without
// explicit listings.
func DefaultRates() map[string]Rate {
	return map[string]Rate{
		// Anthropic Claude family
		"claude-opus-4-7":     {InputPerM: 15.00, OutputPerM: 75.00},
		"claude-opus-4-7-*":   {InputPerM: 15.00, OutputPerM: 75.00},
		"claude-opus-4-*":     {InputPerM: 15.00, OutputPerM: 75.00},
		"claude-sonnet-4-6":   {InputPerM: 3.00, OutputPerM: 15.00},
		"claude-sonnet-4-6-*": {InputPerM: 3.00, OutputPerM: 15.00},
		"claude-sonnet-4-*":   {InputPerM: 3.00, OutputPerM: 15.00},
		"claude-haiku-4-5":    {InputPerM: 1.00, OutputPerM: 5.00},
		"claude-haiku-4-5-*":  {InputPerM: 1.00, OutputPerM: 5.00},
		"claude-haiku-4-*":    {InputPerM: 1.00, OutputPerM: 5.00},
		// Bedrock variants (anthropic.claude-<model>-v<N>:<rev>)
		"anthropic.claude-opus-4-7*":   {InputPerM: 15.00, OutputPerM: 75.00},
		"anthropic.claude-sonnet-4-6*": {InputPerM: 3.00, OutputPerM: 15.00},
		"anthropic.claude-haiku-4-5*":  {InputPerM: 1.00, OutputPerM: 5.00},
		"anthropic.claude-*-opus*":     {InputPerM: 15.00, OutputPerM: 75.00},
		"anthropic.claude-*-sonnet*":   {InputPerM: 3.00, OutputPerM: 15.00},
		"anthropic.claude-*-haiku*":    {InputPerM: 1.00, OutputPerM: 5.00},
		// Vertex variants (claude-<model>@<date>)
		"claude-opus-4-7@*":   {InputPerM: 15.00, OutputPerM: 75.00},
		"claude-sonnet-4-6@*": {InputPerM: 3.00, OutputPerM: 15.00},
		"claude-haiku-4-5@*":  {InputPerM: 1.00, OutputPerM: 5.00},
		// OpenAI
		"gpt-5*":        {InputPerM: 10.00, OutputPerM: 40.00},
		"gpt-4o":        {InputPerM: 2.50, OutputPerM: 10.00},
		"gpt-4o-*":      {InputPerM: 2.50, OutputPerM: 10.00},
		"gpt-4-turbo":   {InputPerM: 10.00, OutputPerM: 30.00},
		"gpt-4-turbo-*": {InputPerM: 10.00, OutputPerM: 30.00},
		"gpt-4o-mini":   {InputPerM: 0.15, OutputPerM: 0.60},
		"gpt-4o-mini-*": {InputPerM: 0.15, OutputPerM: 0.60},
		"o1*":           {InputPerM: 15.00, OutputPerM: 60.00},
		"o3*":           {InputPerM: 15.00, OutputPerM: 60.00},
		// DeepSeek
		"deepseek-chat":     {InputPerM: 0.27, OutputPerM: 1.10},
		"deepseek-reasoner": {InputPerM: 0.55, OutputPerM: 2.19},
		// Kimi (Moonshot)
		"kimi-k2*":   {InputPerM: 0.60, OutputPerM: 2.50},
		"moonshot-*": {InputPerM: 1.20, OutputPerM: 12.00},
		// MiniMax
		"MiniMax-M2": {InputPerM: 0.30, OutputPerM: 1.20},
		"abab*":      {InputPerM: 0.60, OutputPerM: 6.00},
		// GLM (Zhipu)
		"glm-4*":     {InputPerM: 0.50, OutputPerM: 1.50},
		"glm-zero-*": {InputPerM: 1.00, OutputPerM: 3.00},
		// Ollama local — no per-token charges
		"llama3*":    {InputPerM: 0.0, OutputPerM: 0.0},
		"llama-*":    {InputPerM: 0.0, OutputPerM: 0.0},
		"qwen2.5-*":  {InputPerM: 0.0, OutputPerM: 0.0},
		"qwen3*":     {InputPerM: 0.0, OutputPerM: 0.0},
		"mistral*":   {InputPerM: 0.0, OutputPerM: 0.0},
		"codellama*": {InputPerM: 0.0, OutputPerM: 0.0},
		"phi*":       {InputPerM: 0.0, OutputPerM: 0.0},
		"gemma*":     {InputPerM: 0.0, OutputPerM: 0.0},
	}
}

// MergeWithUserRates returns DefaultRates() with user-supplied overrides
// merged on top. User keys win on exact-name collisions; glob-vs-exact
// resolution is left to Table.Lookup (exact always beats glob).
func MergeWithUserRates(user map[string]Rate) map[string]Rate {
	out := DefaultRates()
	for k, v := range user {
		out[k] = v
	}
	return out
}

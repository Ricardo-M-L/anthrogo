package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/ricardo/anthrogo/internal/hooks"
	"github.com/ricardo/anthrogo/internal/mcp"
	"github.com/ricardo/anthrogo/internal/yamlsafe"
	"github.com/ricardo/anthrogo/pkg/permissions"
)

// maxConfigYAMLBytes caps the settings.yaml file size we'll load. A malicious
// or accidentally-huge settings file would otherwise be fully read into
// memory by os.ReadFile + yaml.Unmarshal.
const maxConfigYAMLBytes = 1 << 20 // 1 MiB

// WebSearchConfig holds backend-specific settings for the WebSearch tool.
type WebSearchConfig struct {
	Backend  string `yaml:"backend"`
	APIKey   string `yaml:"apiKey"`
	Endpoint string `yaml:"endpoint,omitempty"`
}

// Profile defines a named provider profile. Used for OpenAI-compatible
// endpoints (DeepSeek, Kimi, MiniMax, GLM, etc.), AWS Bedrock, and GCP Vertex.
type Profile struct {
	Type      string `yaml:"type"`               // "openai" | "anthropic" | "ollama" | "bedrock" | "vertex"
	BaseURL   string `yaml:"base_url,omitempty"` // openai / anthropic / ollama types
	Model     string `yaml:"model,omitempty"`
	APIKey    string `yaml:"api_key,omitempty"`    // openai / anthropic; supports "env:VARNAME"
	Region    string `yaml:"region,omitempty"`     // bedrock / vertex; bedrock defaults to AWS_REGION / default config
	ProjectID string `yaml:"project_id,omitempty"` // vertex only
}

// Pricing maps model names to per-million-token USD prices. Keys can be
// exact model names (e.g. "claude-sonnet-4-6") or globs matched at lookup
// time ("deepseek-*"). InputPerM is the price for one million input tokens;
// OutputPerM for one million output tokens.
type Pricing struct {
	InputPerM  float64 `yaml:"input_per_m"`
	OutputPerM float64 `yaml:"output_per_m"`
}

// ThemeConfig holds theme selection and optional per-field colour overrides.
type ThemeConfig struct {
	Name        string `yaml:"name,omitempty"`        // "dark" | "light" | "custom"
	UserPrompt  string `yaml:"user_prompt,omitempty"` // hex colour override
	Assistant   string `yaml:"assistant,omitempty"`
	ToolHeader  string `yaml:"tool_header,omitempty"`
	ToolBody    string `yaml:"tool_body,omitempty"`
	Error       string `yaml:"error,omitempty"`
	StatusLine  string `yaml:"status_line,omitempty"`
	Border      string `yaml:"border,omitempty"`
	ModalBorder string `yaml:"modal_border,omitempty"`
}

// AuthConfig holds OAuth 2.1 IdP configuration for /login.
type AuthConfig struct {
	AuthorizationURL string   `yaml:"authorization_url,omitempty"`
	TokenURL         string   `yaml:"token_url,omitempty"`
	ClientID         string   `yaml:"client_id,omitempty"`
	ClientSecret     string   `yaml:"client_secret,omitempty"`
	Scopes           []string `yaml:"scopes,omitempty"`
	RedirectPort     int      `yaml:"redirect_port,omitempty"`
}

// Config mirrors the on-disk settings.yaml shape.
type Config struct {
	Mode        permissions.Mode               `yaml:"mode"`
	Model       string                         `yaml:"model"`
	APIKey      string                         `yaml:"apiKey,omitempty"`
	Provider    string                         `yaml:"provider,omitempty"` // default "anthropic"
	Auth        AuthConfig                     `yaml:"auth,omitempty"`
	Profiles    map[string]Profile             `yaml:"profiles,omitempty"`
	WebSearch   WebSearchConfig                `yaml:"webSearch,omitempty"`
	MCPServers  map[string]mcp.MCPServerConfig `yaml:"mcpServers,omitempty"`
	AlwaysAllow []permissions.Rule             `yaml:"alwaysAllow"`
	AlwaysDeny  []permissions.Rule             `yaml:"alwaysDeny"`
	AlwaysAsk   []permissions.Rule             `yaml:"alwaysAsk"`
	Hooks       hooks.Config                   `yaml:"hooks,omitempty"`
	Theme       ThemeConfig                    `yaml:"theme,omitempty"`

	AutoCompactThreshold  int `yaml:"auto_compact_threshold,omitempty"`
	AutoCompactKeepRecent int `yaml:"auto_compact_keep_recent,omitempty"`

	// SessionSearchCacheSize overrides the default LRU cap (64) for the session
	// search replay cache. 0 or missing means use the default.
	SessionSearchCacheSize int `yaml:"session_search_cache_size,omitempty"`

	// Pricing maps model names (exact or glob) to per-million-token USD rates.
	// Default empty (no cost tracking).
	Pricing map[string]Pricing `yaml:"pricing,omitempty"`

	// CostLimitUSD, when > 0 and Pricing is configured, denies tool calls once
	// the cumulative estimated session cost reaches or exceeds this amount (USD).
	CostLimitUSD float64 `yaml:"cost_limit_usd,omitempty"`

	// UseAnthropicTokenAPI, when true and the active provider is Anthropic,
	// uses the Anthropic SDK's Messages.CountTokens endpoint for Claude-family
	// models instead of the char/4 approximation. Each call is a network
	// round-trip and counts against quota. Off by default.
	UseAnthropicTokenAPI bool `yaml:"use_anthropic_token_api,omitempty"`

	// ProvidersFailover lists profile names to try after the active provider
	// fails (EventError before any committed event). Each profile is resolved
	// via the same buildFromProfile logic as --provider.
	ProvidersFailover []string `yaml:"providers_failover,omitempty"`

	// Telemetry configures opt-in anonymous usage telemetry. Disabled by default.
	Telemetry TelemetryConfig `yaml:"telemetry,omitempty"`
}

// TelemetryConfig holds opt-in telemetry settings.
// Telemetry is OFF by default; the user must set enabled=true in settings.yaml
// and supply their own endpoint. No central anthrogo collector exists.
type TelemetryConfig struct {
	Enabled  bool   `yaml:"enabled,omitempty"`
	Endpoint string `yaml:"endpoint,omitempty"`
}

func defaults() Config {
	return Config{
		Mode:  permissions.ModeDefault,
		Model: "claude-sonnet-4-6",
	}
}

// Load reads settings.yaml from Home(). Missing file => defaults.
func Load() (Config, error) {
	p, err := SettingsPath()
	if err != nil {
		return Config{}, err
	}
	if st, err := os.Stat(p); err == nil && st.Size() > maxConfigYAMLBytes {
		return Config{}, fmt.Errorf("settings.yaml is %d bytes; refusing to load (cap %d)", st.Size(), maxConfigYAMLBytes)
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return defaults(), nil
		}
		return Config{}, err
	}
	cfg := defaults()
	var yamlWarnings []string
	if err := yamlsafe.Unmarshal(raw, &cfg, "settings.yaml", &yamlWarnings); err != nil {
		return Config{}, err
	}
	for _, w := range yamlWarnings {
		fmt.Fprintf(os.Stderr, "anthrogo: %s\n", w)
	}
	// Expand hook paths and fill default timeouts; warnings are surfaced by callers.
	cfg.Hooks.Expand()
	return cfg, nil
}

// ToPermissionContext realises the config's rules into a permissions.Context.
func (c Config) ToPermissionContext() *permissions.Context {
	pc := permissions.Empty()
	pc.Mode = c.Mode
	if len(c.AlwaysAllow) > 0 {
		pc.AlwaysAllowRules[permissions.SourceUser] = c.AlwaysAllow
	}
	if len(c.AlwaysDeny) > 0 {
		pc.AlwaysDenyRules[permissions.SourceUser] = c.AlwaysDeny
	}
	if len(c.AlwaysAsk) > 0 {
		pc.AlwaysAskRules[permissions.SourceUser] = c.AlwaysAsk
	}
	if c.Mode == permissions.ModeBypassPermissions {
		pc.IsBypassAvailable = true
	}
	return pc
}

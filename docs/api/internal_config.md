# `github.com/ricardo/anthrogo/internal/config`

```go
package config // import "github.com/ricardo/anthrogo/internal/config"


FUNCTIONS

func Home() (string, error)
    Home returns the anthrogo config directory: $ANTHROGO_HOME, else
    ~/.anthrogo, creating it on demand.

func ProjectSystemOverlayPath(cwd string) string
    ProjectSystemOverlayPath returns <cwd>/.anthrogo/system_overlay.md.

func SettingsPath() (string, error)
    SettingsPath returns the settings.yaml path inside Home.

func SkillsDir(home string) string
    SkillsDir returns the absolute path to <home>/.anthrogo/skills/. Pass the
    raw user home (os.UserHomeDir() or os.Getenv("HOME")); do NOT pass the
    already-resolved anthrogo home directory.

func SystemOverlayPath(home string) string
    SystemOverlayPath returns the path to the persistent user system prompt
    overlay file: <home>/.anthrogo/system_overlay.md. Pass the raw user home
    (os.UserHomeDir() or os.Getenv("HOME")).


TYPES

type AuthConfig struct {
	AuthorizationURL string   `yaml:"authorization_url,omitempty"`
	TokenURL         string   `yaml:"token_url,omitempty"`
	ClientID         string   `yaml:"client_id,omitempty"`
	ClientSecret     string   `yaml:"client_secret,omitempty"`
	Scopes           []string `yaml:"scopes,omitempty"`
	RedirectPort     int      `yaml:"redirect_port,omitempty"`
}
    AuthConfig holds OAuth 2.1 IdP configuration for /login.

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
    Config mirrors the on-disk settings.yaml shape.

func Load() (Config, error)
    Load reads settings.yaml from Home(). Missing file => defaults.

func (c Config) ToPermissionContext() *permissions.Context
    ToPermissionContext realises the config's rules into a permissions.Context.

type Pricing struct {
	InputPerM  float64 `yaml:"input_per_m"`
	OutputPerM float64 `yaml:"output_per_m"`
}
    Pricing maps model names to per-million-token USD prices. Keys can be
    exact model names (e.g. "claude-sonnet-4-6") or globs matched at lookup
    time ("deepseek-*"). InputPerM is the price for one million input tokens;
    OutputPerM for one million output tokens.

type Profile struct {
	Type      string `yaml:"type"`               // "openai" | "bedrock" | "vertex"
	BaseURL   string `yaml:"base_url,omitempty"` // openai only
	Model     string `yaml:"model,omitempty"`
	APIKey    string `yaml:"api_key,omitempty"`    // openai only; supports "env:VARNAME"
	Region    string `yaml:"region,omitempty"`     // bedrock / vertex; bedrock defaults to AWS_REGION / default config
	ProjectID string `yaml:"project_id,omitempty"` // vertex only
}
    Profile defines a named provider profile. Used for OpenAI-compatible
    endpoints (DeepSeek, Kimi, MiniMax, GLM, etc.), AWS Bedrock, and GCP Vertex.

type TelemetryConfig struct {
	Enabled  bool   `yaml:"enabled,omitempty"`
	Endpoint string `yaml:"endpoint,omitempty"`
}
    TelemetryConfig holds opt-in telemetry settings. Telemetry is OFF by
    default; the user must set enabled=true in settings.yaml and supply their
    own endpoint. No central anthrogo collector exists.

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
    ThemeConfig holds theme selection and optional per-field colour overrides.

type WebSearchConfig struct {
	Backend  string `yaml:"backend"`
	APIKey   string `yaml:"apiKey"`
	Endpoint string `yaml:"endpoint,omitempty"`
}
    WebSearchConfig holds backend-specific settings for the WebSearch tool.

```

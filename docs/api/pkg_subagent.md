# `github.com/ricardo/anthrogo/pkg/subagent`

```go
package subagent // import "github.com/ricardo/anthrogo/pkg/subagent"


TYPES

type Registry struct {
	// Has unexported fields.
}
    Registry holds named subagent Specs.

func DefaultRegistry() *Registry
    DefaultRegistry returns a Registry pre-populated with the built-in
    "general-purpose" subagent type.

func NewRegistry() *Registry
    NewRegistry returns an empty Registry.

func (r *Registry) Get(name string) (Spec, bool)
    Get retrieves a Spec by name. Returns false if not found.

func (r *Registry) List() []Spec
    List returns all specs sorted by Name.

func (r *Registry) Register(s Spec)
    Register adds a Spec to the registry.

func (r *Registry) Replace(other *Registry)
    Replace swaps r's underlying spec map with other's. Used by /subagents
    reload to atomically replace the registry contents in place.

type RemoteSpec struct {
	Endpoint           string `yaml:"endpoint"`                       // http://host:port or https://host:port
	AuthToken          string `yaml:"auth_token"`                     // optional; supports "env:VARNAME" prefix
	ExecToolsLocally   bool   `yaml:"exec_tools_locally,omitempty"`   // when true, tool calls from the remote subagent execute on the CLIENT process
	TrustKey           string `yaml:"trust_key,omitempty"`            // base64 ed25519 public key (or path) for SSE signature verification
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify,omitempty"` // DEV ONLY: skip TLS certificate verification
	CACertPath         string `yaml:"ca_cert_path,omitempty"`         // path to PEM CA cert for custom/internal CAs
}
    RemoteSpec configures cross-process dispatch for a subagent type. When
    non-nil, Engine.RunSubagent dispatches via HTTP to the KAIROS worker instead
    of spawning a local child Engine.

type Spec struct {
	Name               string
	Description        string
	SystemPromptSuffix string
	ToolAllowlist      []string    // empty = inherit all
	Remote             *RemoteSpec // when non-nil, RunSubagent dispatches via HTTP
}
    Spec describes a subagent type: its name, description (shown to the model
    in the Task tool schema), a system prompt suffix appended to the parent's
    prompt, and an optional tool allowlist (empty = inherit all parent tools).

func LoadAll(homeRoot, cwdRoot string) ([]Spec, []string, error)
    LoadAll scans homeRoot and cwdRoot for *.yaml / *.yml files (non-recursive).
    cwd specs override home specs on same name (with a warning). Refuses to
    register "general-purpose" (reserved built-in name). Returns the merged
    slice, any non-fatal warnings, and any fatal error.

```

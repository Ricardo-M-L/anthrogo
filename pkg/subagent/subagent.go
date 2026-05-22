package subagent

import "sort"

// RemoteSpec configures cross-process dispatch for a subagent type.
// When non-nil, Engine.RunSubagent dispatches via HTTP to the KAIROS worker
// instead of spawning a local child Engine.
type RemoteSpec struct {
	Endpoint           string `yaml:"endpoint"`                       // http://host:port or https://host:port
	AuthToken          string `yaml:"auth_token"`                     // optional; supports "env:VARNAME" prefix
	ExecToolsLocally   bool   `yaml:"exec_tools_locally,omitempty"`   // when true, tool calls from the remote subagent execute on the CLIENT process
	TrustKey           string `yaml:"trust_key,omitempty"`            // base64 ed25519 public key (or path) for SSE signature verification
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify,omitempty"` // DEV ONLY: skip TLS certificate verification
	CACertPath         string `yaml:"ca_cert_path,omitempty"`         // path to PEM CA cert for custom/internal CAs
}

// Spec describes a subagent type: its name, description (shown to the model in
// the Task tool schema), a system prompt suffix appended to the parent's prompt,
// and an optional tool allowlist (empty = inherit all parent tools).
type Spec struct {
	Name               string
	Description        string
	SystemPromptSuffix string
	ToolAllowlist      []string    // empty = inherit all
	Remote             *RemoteSpec // when non-nil, RunSubagent dispatches via HTTP
	Model              string      // optional model override; empty = inherit parent model
}

// Registry holds named subagent Specs.
type Registry struct {
	specs map[string]Spec
}

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{specs: map[string]Spec{}}
}

// Register adds a Spec to the registry.
func (r *Registry) Register(s Spec) {
	r.specs[s.Name] = s
}

// Get retrieves a Spec by name. Returns false if not found.
func (r *Registry) Get(name string) (Spec, bool) {
	s, ok := r.specs[name]
	return s, ok
}

// List returns all specs sorted by Name.
func (r *Registry) List() []Spec {
	out := make([]Spec, 0, len(r.specs))
	for _, s := range r.specs {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Replace swaps r's underlying spec map with other's. Used by /subagents reload
// to atomically replace the registry contents in place.
func (r *Registry) Replace(other *Registry) {
	r.specs = other.specs
}

// DefaultRegistry returns a Registry pre-populated with the built-in
// "general-purpose" and "refactor" subagent types.
func DefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register(Spec{
		Name:               "general-purpose",
		Description:        "General-purpose subagent for complex multi-step research, code search, or independent task execution. Brief it with a self-contained prompt — it has no memory of the parent conversation.",
		SystemPromptSuffix: "\n\n# You are a subagent\n\nYou were spawned by a parent agent to do a focused task. You have no memory of the parent's conversation. Use the tools you have access to, gather what you need, and return a concise final answer.\n",
	})
	r.Register(Spec{
		Name:               "refactor",
		Description:        "Apply a focused multi-file refactor using Read/Edit/Write tools only. Spawned by the /refactor slash command.",
		ToolAllowlist:      []string{"Read", "Edit", "Write", "Glob", "Grep"},
		SystemPromptSuffix: "\n\n# You are a refactor subagent\n\nYou were spawned by the /refactor command to apply a focused code transformation across a set of files. Read each in-scope file with the Read tool, apply the requested change with Edit (preferred) or Write (full rewrite only when necessary). Verify each edit by re-reading. Do not deviate from the instruction. At the end, summarize what changed per file.\n",
	})
	return r
}

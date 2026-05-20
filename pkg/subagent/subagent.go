package subagent

import "sort"

// Spec describes a subagent type: its name, description (shown to the model in
// the Task tool schema), a system prompt suffix appended to the parent's prompt,
// and an optional tool allowlist (empty = inherit all parent tools).
type Spec struct {
	Name               string
	Description        string
	SystemPromptSuffix string
	ToolAllowlist      []string // empty = inherit all
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

// DefaultRegistry returns a Registry pre-populated with the built-in
// "general-purpose" subagent type.
func DefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register(Spec{
		Name:        "general-purpose",
		Description: "General-purpose subagent for complex multi-step research, code search, or independent task execution. Brief it with a self-contained prompt — it has no memory of the parent conversation.",
		SystemPromptSuffix: "\n\n# You are a subagent\n\nYou were spawned by a parent agent to do a focused task. You have no memory of the parent's conversation. Use the tools you have access to, gather what you need, and return a concise final answer.\n",
	})
	return r
}

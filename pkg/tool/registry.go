package tool

import "fmt"

// Registry stores Tools in registration order so the API-side tool list is
// stable run-to-run (important for prompt caching).
type Registry struct {
	byName map[string]Tool
	order  []string
}

func NewRegistry() *Registry {
	return &Registry{byName: map[string]Tool{}}
}

func (r *Registry) Register(t Tool) {
	if _, dup := r.byName[t.Name()]; dup {
		panic(fmt.Sprintf("tool already registered: %s", t.Name()))
	}
	r.byName[t.Name()] = t
	r.order = append(r.order, t.Name())
}

func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.byName[name]
	return t, ok
}

func (r *Registry) All() []Tool {
	out := make([]Tool, 0, len(r.order))
	for _, n := range r.order {
		out = append(out, r.byName[n])
	}
	return out
}

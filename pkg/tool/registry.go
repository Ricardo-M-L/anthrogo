package tool

import (
	"fmt"
	"log"
	"strings"
)

// Registry stores Tools in registration order so the API-side tool list is
// stable run-to-run (important for prompt caching).
type Registry struct {
	byName map[string]Tool
	order  []string
}

func NewRegistry() *Registry {
	return &Registry{byName: map[string]Tool{}}
}

// TryRegister adds t to the registry. Returns an error (without modifying the
// registry) if a tool with the same name is already registered.
func (r *Registry) TryRegister(t Tool) error {
	if _, dup := r.byName[t.Name()]; dup {
		return fmt.Errorf("tool already registered: %s", t.Name())
	}
	r.byName[t.Name()] = t
	r.order = append(r.order, t.Name())
	return nil
}

// Register adds t to the registry. On duplicate name it logs a warning and
// silently discards the new tool (the first registration wins).
func (r *Registry) Register(t Tool) {
	if err := r.TryRegister(t); err != nil {
		log.Printf("tool registry: %v", err)
	}
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

// RemoveByPrefix unregisters every tool whose Name() starts with prefix.
// Returns the number removed.
func (r *Registry) RemoveByPrefix(prefix string) int {
	removed := 0
	kept := r.order[:0]
	for _, n := range r.order {
		if strings.HasPrefix(n, prefix) {
			delete(r.byName, n)
			removed++
			continue
		}
		kept = append(kept, n)
	}
	r.order = kept
	return removed
}

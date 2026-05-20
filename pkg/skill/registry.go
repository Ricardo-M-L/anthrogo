package skill

import (
	"sort"
	"sync"
)

type Registry struct {
	mu     sync.RWMutex
	skills map[string]Skill
}

func NewRegistry(list []Skill) *Registry {
	r := &Registry{skills: map[string]Skill{}}
	for _, s := range list {
		r.skills[s.Name] = s
	}
	return r
}

func (r *Registry) Get(name string) (Skill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.skills[name]
	return s, ok
}

func (r *Registry) List() []Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Skill, 0, len(r.skills))
	for _, s := range r.skills {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Add inserts s into the registry if no skill with that name exists.
// Returns true if added, false if a same-named skill was already present
// (in which case s is discarded).
func (r *Registry) Add(s Skill) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.skills[s.Name]; exists {
		return false
	}
	r.skills[s.Name] = s
	return true
}

func (r *Registry) Reload(homeRoot, cwdRoot string) ([]string, error) {
	skills, warnings, err := LoadAll(homeRoot, cwdRoot)
	if err != nil {
		return warnings, err
	}
	r.mu.Lock()
	r.skills = map[string]Skill{}
	for _, s := range skills {
		r.skills[s.Name] = s
	}
	r.mu.Unlock()
	return warnings, nil
}

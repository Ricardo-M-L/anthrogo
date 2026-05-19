package command

import "strings"

type Registry struct {
	cmds  map[string]Command
	order []string
}

func NewRegistry() *Registry {
	return &Registry{cmds: map[string]Command{}}
}

func (r *Registry) Register(c Command) {
	name := strings.TrimPrefix(c.Name(), "/")
	r.cmds[name] = c
	r.order = append(r.order, name)
	for _, a := range c.Aliases() {
		r.cmds[strings.TrimPrefix(a, "/")] = c
	}
}

func (r *Registry) Lookup(input string) (Command, bool) {
	name := strings.TrimPrefix(strings.TrimSpace(input), "/")
	c, ok := r.cmds[name]
	return c, ok
}

func (r *Registry) Fuzzy(q string) []Command {
	q = strings.TrimPrefix(strings.TrimSpace(q), "/")
	q = strings.ToLower(q)
	seen := map[string]bool{}
	var out []Command
	for _, n := range r.order {
		if strings.HasPrefix(strings.ToLower(n), q) && !seen[n] {
			out = append(out, r.cmds[n])
			seen[n] = true
		}
	}
	return out
}

func (r *Registry) All() []Command {
	out := make([]Command, 0, len(r.order))
	for _, n := range r.order {
		out = append(out, r.cmds[n])
	}
	return out
}

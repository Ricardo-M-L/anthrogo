package hooks

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

// Spec is one hook entry under a given event.
type Spec struct {
	Matcher string        `yaml:"matcher,omitempty"`
	Command string        `yaml:"command"`
	Timeout time.Duration `yaml:"timeout,omitempty"`

	matcherRE *regexp.Regexp // compiled lazily in Validate
}

// Config holds per-event hook lists. Field names match event names so YAML
// keys are PascalCase (PreToolUse, etc.).
type Config struct {
	PreToolUse       []Spec `yaml:"PreToolUse,omitempty"`
	PostToolUse      []Spec `yaml:"PostToolUse,omitempty"`
	UserPromptSubmit []Spec `yaml:"UserPromptSubmit,omitempty"`
	Stop             []Spec `yaml:"Stop,omitempty"`
	SubagentStop     []Spec `yaml:"SubagentStop,omitempty"`
	Notification     []Spec `yaml:"Notification,omitempty"`
	PreCompact       []Spec `yaml:"PreCompact,omitempty"`
	SessionStart     []Spec `yaml:"SessionStart,omitempty"`
	SessionEnd       []Spec `yaml:"SessionEnd,omitempty"`
}

func (c *Config) allLists() []*[]Spec {
	return []*[]Spec{
		&c.PreToolUse, &c.PostToolUse, &c.UserPromptSubmit,
		&c.Stop, &c.SubagentStop, &c.Notification,
		&c.PreCompact, &c.SessionStart, &c.SessionEnd,
	}
}

// Expand replaces ~/ and $VAR in every Command, fills in default Timeout per event.
func (c *Config) Expand() {
	home, _ := os.UserHomeDir()
	defaults := map[string]time.Duration{
		"PreToolUse":       30 * time.Second,
		"PostToolUse":      30 * time.Second,
		"UserPromptSubmit": 30 * time.Second,
		"Stop":             10 * time.Second,
		"SubagentStop":     10 * time.Second,
		"Notification":     5 * time.Second,
		"PreCompact":       30 * time.Second,
		"SessionStart":     5 * time.Second,
		"SessionEnd":       5 * time.Second,
	}
	apply := func(list []Spec, defTimeout time.Duration) []Spec {
		out := make([]Spec, 0, len(list))
		for _, s := range list {
			s.Command = expandPath(s.Command, home)
			if s.Timeout == 0 {
				s.Timeout = defTimeout
			}
			out = append(out, s)
		}
		return out
	}
	c.PreToolUse = apply(c.PreToolUse, defaults["PreToolUse"])
	c.PostToolUse = apply(c.PostToolUse, defaults["PostToolUse"])
	c.UserPromptSubmit = apply(c.UserPromptSubmit, defaults["UserPromptSubmit"])
	c.Stop = apply(c.Stop, defaults["Stop"])
	c.SubagentStop = apply(c.SubagentStop, defaults["SubagentStop"])
	c.Notification = apply(c.Notification, defaults["Notification"])
	c.PreCompact = apply(c.PreCompact, defaults["PreCompact"])
	c.SessionStart = apply(c.SessionStart, defaults["SessionStart"])
	c.SessionEnd = apply(c.SessionEnd, defaults["SessionEnd"])
}

// Validate compiles all matchers; invalid ones drop their spec and append a warning.
func (c *Config) Validate() []string {
	var warnings []string
	for _, listPtr := range c.allLists() {
		filtered := (*listPtr)[:0]
		for _, s := range *listPtr {
			if s.Matcher != "" {
				re, err := regexp.Compile(s.Matcher)
				if err != nil {
					warnings = append(warnings,
						fmt.Sprintf("dropped hook %s: bad matcher %q (%v)", s.Command, s.Matcher, err))
					continue
				}
				s.matcherRE = re
			}
			filtered = append(filtered, s)
		}
		*listPtr = filtered
	}
	return warnings
}

// AppendOverlay returns a new Config = c with each event's list extended by overlay's list.
func (c Config) AppendOverlay(overlay Config) Config {
	out := c
	out.PreToolUse = append(out.PreToolUse, overlay.PreToolUse...)
	out.PostToolUse = append(out.PostToolUse, overlay.PostToolUse...)
	out.UserPromptSubmit = append(out.UserPromptSubmit, overlay.UserPromptSubmit...)
	out.Stop = append(out.Stop, overlay.Stop...)
	out.SubagentStop = append(out.SubagentStop, overlay.SubagentStop...)
	out.Notification = append(out.Notification, overlay.Notification...)
	out.PreCompact = append(out.PreCompact, overlay.PreCompact...)
	out.SessionStart = append(out.SessionStart, overlay.SessionStart...)
	out.SessionEnd = append(out.SessionEnd, overlay.SessionEnd...)
	return out
}

func expandPath(p, home string) string {
	if strings.HasPrefix(p, "~/") {
		p = home + p[1:]
	}
	return os.ExpandEnv(p)
}

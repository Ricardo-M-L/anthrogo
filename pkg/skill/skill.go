package skill

// Skill is one parsed SKILL.md.
type Skill struct {
	Name        string
	Description string
	BasePath    string // absolute path to the skill's directory
	Body        string // markdown after frontmatter
	Source      string // "home" | "cwd"
}

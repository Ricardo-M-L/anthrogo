package tui

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ricardo/anthrogo/pkg/command"
)

// paletteMode discriminates the kind of completion currently showing.
type paletteMode int

const (
	paletteOff paletteMode = iota
	paletteCommand                 // input starts with '/'
	paletteFile                    // input has '@<partial>' at the cursor
)

// paletteMatch is the generic shape — works for both slash commands and
// @-file completions.
type paletteMatch struct {
	display     string // shown in the dropdown left column
	desc        string // shown in the dim right column
	completion  string // text that replaces the user's draft
}

// palette is the overlay shown while the user is typing a slash command or
// a file reference (`@some/path`). Reuses the same dropdown UI for both.
type palette struct {
	theme    Theme
	reg      *command.Registry
	cwd      string // root for file completions; defaults to engine cwd
	mode     paletteMode
	matches  []paletteMatch
	selected int
}

func newPalette(theme Theme, reg *command.Registry) palette {
	return palette{theme: theme, reg: reg}
}

// setCwd updates the directory used as the @-completion root. Called once
// per app from the construction site.
func (p *palette) setCwd(dir string) { p.cwd = dir }

func (p *palette) visible() bool { return p.mode != paletteOff && len(p.matches) > 0 }

func (p *palette) updateForInput(input string) {
	switch {
	case strings.HasPrefix(input, "/"):
		p.populateCommands(input)
	case hasFileTrigger(input):
		p.populateFiles(extractFilePartial(input))
	default:
		p.mode = paletteOff
		p.matches = nil
		p.selected = 0
	}
}

func (p *palette) populateCommands(input string) {
	if p.reg == nil {
		p.mode = paletteOff
		return
	}
	cmds := p.reg.Fuzzy(input)
	p.matches = make([]paletteMatch, 0, len(cmds))
	for _, c := range cmds {
		p.matches = append(p.matches, paletteMatch{
			display:    c.Name(),
			desc:       c.Description(),
			completion: c.Name() + " ",
		})
	}
	p.mode = paletteCommand
	if p.selected >= len(p.matches) {
		p.selected = 0
	}
}

// populateFiles lists up to 50 files under p.cwd whose basename or path
// starts with the partial after '@'. Hidden directories (.git, node_modules,
// etc.) are skipped. Sorted by depth then name.
func (p *palette) populateFiles(partial string) {
	if p.cwd == "" {
		p.cwd = "."
	}
	matches := paletteFileMatches(p.cwd, partial, 50)
	p.matches = make([]paletteMatch, 0, len(matches))
	for _, m := range matches {
		// Replace '@<partial>' with '@<full path>' in the input.
		p.matches = append(p.matches, paletteMatch{
			display:    "@" + m.rel,
			desc:       m.tag,
			completion: "REPLACE_PARTIAL:" + m.rel,
		})
	}
	p.mode = paletteFile
	if p.selected >= len(p.matches) {
		p.selected = 0
	}
}

// hasFileTrigger reports whether the input ends with '@<chars>' or just
// '@' — the contexts in which we offer file completion.
func hasFileTrigger(input string) bool {
	// Trigger: last token starts with '@'. Use the last whitespace-
	// separated chunk so '@' embedded in arbitrary text (e.g. an email
	// address typed earlier) doesn't trigger.
	i := strings.LastIndexAny(input, " \t\n")
	last := input
	if i >= 0 {
		last = input[i+1:]
	}
	return strings.HasPrefix(last, "@")
}

// extractFilePartial returns the chars after the trailing '@'.
func extractFilePartial(input string) string {
	i := strings.LastIndexAny(input, " \t\n")
	last := input
	if i >= 0 {
		last = input[i+1:]
	}
	return strings.TrimPrefix(last, "@")
}

type fileMatch struct {
	rel string // path relative to cwd
	tag string // 'dir' / size hint / type
}

func paletteFileMatches(cwdRoot, partial string, max int) []fileMatch {
	var matches []fileMatch
	// Walk shallowly (depth ≤ 4) and skip obvious noise dirs. Hidden
	// dotfiles are skipped except for the explicitly-requested ones.
	skipDirs := map[string]bool{
		".git": true, "node_modules": true, "vendor": true, ".idea": true,
		".vscode": true, "__pycache__": true, "dist": true, "build": true,
		".anthrogo": true,
	}
	_ = filepath.WalkDir(cwdRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(cwdRoot, path)
		if rel == "." {
			return nil
		}
		base := d.Name()
		if d.IsDir() {
			if skipDirs[base] || (strings.HasPrefix(base, ".") && partial == "") {
				return filepath.SkipDir
			}
			// Limit depth.
			if strings.Count(rel, string(filepath.Separator)) > 3 {
				return filepath.SkipDir
			}
		} else if strings.HasPrefix(base, ".") && !strings.HasPrefix(partial, ".") {
			return nil
		}
		// Match: partial is empty OR rel starts with partial OR basename starts.
		if partial != "" && !strings.HasPrefix(rel, partial) && !strings.HasPrefix(base, partial) {
			return nil
		}
		tag := "file"
		if d.IsDir() {
			tag = "dir"
		} else if info, err := d.Info(); err == nil {
			if sz := info.Size(); sz > 1024 {
				tag = fmt.Sprintf("%d KB", sz/1024)
			} else {
				tag = fmt.Sprintf("%d B", sz)
			}
		}
		matches = append(matches, fileMatch{rel: rel, tag: tag})
		if len(matches) >= max {
			return fs.SkipAll
		}
		return nil
	})
	// Sort by depth (shallower first) then lexicographically.
	sort.Slice(matches, func(i, j int) bool {
		di := strings.Count(matches[i].rel, string(filepath.Separator))
		dj := strings.Count(matches[j].rel, string(filepath.Separator))
		if di != dj {
			return di < dj
		}
		return matches[i].rel < matches[j].rel
	})
	return matches
}

// handleKey returns (consumed, newInputValue). newInputValue == "" means input unchanged.
// For file mode, the returned string includes a 'REPLACE_PARTIAL:<full>'
// sentinel which the caller (app.go) decodes by substituting the trailing
// '@<partial>' token with '@<full>'.
func (p *palette) handleKey(msg tea.KeyMsg) (bool, string) {
	if !p.visible() {
		return false, ""
	}
	switch msg.String() {
	case "tab":
		next := (p.selected + 1) % len(p.matches)
		p.selected = next
		return true, p.matches[next].completion
	case "shift+tab":
		next := (p.selected - 1 + len(p.matches)) % len(p.matches)
		p.selected = next
		return true, p.matches[next].completion
	case "esc":
		p.mode = paletteOff
		return true, ""
	}
	return false, ""
}

const paletteVisible = 7

func (p *palette) view() string {
	if !p.visible() {
		return ""
	}
	start := 0
	if len(p.matches) > paletteVisible {
		half := paletteVisible / 2
		start = p.selected - half
		if start < 0 {
			start = 0
		}
		if start+paletteVisible > len(p.matches) {
			start = len(p.matches) - paletteVisible
		}
	}
	end := start + paletteVisible
	if end > len(p.matches) {
		end = len(p.matches)
	}

	var b strings.Builder
	// Mode label so the user knows what they're cycling.
	switch p.mode {
	case paletteCommand:
		b.WriteString(p.theme.StatusLine.Render("  slash commands"))
	case paletteFile:
		b.WriteString(p.theme.StatusLine.Render("  files in " + p.cwd))
	}
	b.WriteByte('\n')
	for i := start; i < end; i++ {
		m := p.matches[i]
		var line string
		if i == p.selected {
			line = p.theme.UserPrompt.Render("  "+m.display) + p.theme.StatusLine.Render("  "+m.desc)
		} else {
			line = p.theme.StatusLine.Render("  " + m.display + "  " + m.desc)
		}
		b.WriteString(line)
		if i < end-1 {
			b.WriteByte('\n')
		}
	}
	if len(p.matches) > paletteVisible {
		b.WriteByte('\n')
		b.WriteString(p.theme.StatusLine.Render(fmt.Sprintf("  · %d/%d ·  Tab to cycle", p.selected+1, len(p.matches))))
	}
	return b.String()
}

// applyFileCompletion replaces the trailing '@<partial>' of `input` with
// '@<full>'. Used by app.go to decode the REPLACE_PARTIAL sentinel.
func applyFileCompletion(input, full string) string {
	i := strings.LastIndexAny(input, " \t\n")
	prefix := ""
	if i >= 0 {
		prefix = input[:i+1]
	}
	return prefix + "@" + full + " "
}

// We need an os.* import touch so editors don't strip the import if all
// uses are inside conditional branches.
var _ = os.PathSeparator

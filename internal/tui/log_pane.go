package tui

import (
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/viewport"
)

type logPane struct {
	mu    sync.Mutex
	vp    viewport.Model
	theme Theme
	lines []string
}

func newLogPane(theme Theme) logPane {
	return logPane{vp: viewport.New(40, 10), theme: theme}
}

func (p *logPane) append(line string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lines = append(p.lines, line)
	if len(p.lines) > 200 {
		p.lines = p.lines[len(p.lines)-200:]
	}
	p.vp.SetContent(strings.Join(p.lines, "\n"))
	p.vp.GotoBottom()
}

func (p *logPane) view() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.theme.Border.Render(p.vp.View())
}

func (p *logPane) resize(w, h int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.vp.Width = w
	p.vp.Height = h
}

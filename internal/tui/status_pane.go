package tui

import (
	"fmt"
	"strings"

	"github.com/ricardo/anthrogo/pkg/permissions"
)

type statusPane struct {
	theme  Theme
	width  int
	height int

	// injected reads:
	getMode      func() permissions.Mode
	getCwd       func() string
	getModel     func() string
	getUsageLine func() string // already-formatted tokens/cost line
}

func (p *statusPane) view() string {
	var b strings.Builder
	b.WriteString("anthrogo\n\n")
	if p.getMode != nil {
		fmt.Fprintf(&b, "mode:  %s\n", p.getMode())
	}
	if p.getModel != nil {
		fmt.Fprintf(&b, "model: %s\n", p.getModel())
	}
	if p.getCwd != nil {
		fmt.Fprintf(&b, "cwd:   %s\n", truncCwd(p.getCwd(), p.width-7))
	}
	b.WriteString("\n")
	if p.getUsageLine != nil {
		b.WriteString(p.getUsageLine() + "\n")
	}
	return p.theme.Border.Width(p.width).Height(p.height).Render(b.String())
}

func truncCwd(s string, maxw int) string {
	if maxw <= 4 || len(s) <= maxw {
		return s
	}
	return "…" + s[len(s)-(maxw-1):]
}

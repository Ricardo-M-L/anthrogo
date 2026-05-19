package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ricardo/anthrogo/pkg/tool"
)

type permissionAsk struct {
	req   tool.PromptRequest
	reply chan tool.PromptResponse
}

type permission struct {
	theme   Theme
	visible bool
	pending permissionAsk
}

func newPermission(theme Theme) permission { return permission{theme: theme} }

func (p *permission) show(req permissionAsk) {
	p.visible = true
	p.pending = req
}

func (p *permission) hide() { p.visible = false; p.pending = permissionAsk{} }

func (p *permission) update(msg tea.Msg) bool {
	if !p.visible {
		return false
	}
	k, ok := msg.(tea.KeyMsg)
	if !ok {
		return true
	}
	if p.pending.req.Kind == tool.PromptQuestion {
		switch k.String() {
		case "1", "2", "3", "4":
			idx := int(k.String()[0] - '1')
			if idx < len(p.pending.req.Options) {
				p.pending.reply <- tool.PromptResponse{
					SelectedLabel: p.pending.req.Options[idx].Label,
				}
				close(p.pending.reply)
				p.hide()
			}
			return true
		case "esc":
			p.pending.reply <- tool.PromptResponse{Reason: "user cancelled"}
			close(p.pending.reply)
			p.hide()
			return true
		}
		return true
	}
	resp := tool.PromptResponse{}
	switch k.String() {
	case "y":
		resp.Allow = true
	case "a":
		resp.Allow = true
		resp.Remember = true
	case "n", "esc":
		resp.Allow = false
		resp.Reason = "user denied"
	default:
		return true
	}
	p.pending.reply <- resp
	close(p.pending.reply)
	p.hide()
	return true
}

func (p permission) view() string {
	if !p.visible {
		return ""
	}
	switch p.pending.req.Kind {
	case tool.PromptQuestion:
		var body strings.Builder
		body.WriteString("Question: " + p.pending.req.Question + "\n\n")
		for i, opt := range p.pending.req.Options {
			fmt.Fprintf(&body, "  [%d] %s", i+1, opt.Label)
			if opt.Description != "" {
				fmt.Fprintf(&body, " — %s", opt.Description)
			}
			body.WriteByte('\n')
		}
		body.WriteString("\nPress 1–4 to select   |   [esc] cancel")
		return p.theme.ModalBorder.Padding(1, 2).Render(body.String())
	default:
		raw, _ := json.MarshalIndent(p.pending.req.ToolInput, "", "  ")
		body := fmt.Sprintf("Tool: %s\nInput:\n%s\n\n[y] allow once   [a] always allow   [n] deny",
			p.pending.req.ToolName, string(raw))
		return p.theme.ModalBorder.Padding(1, 2).Render(body)
	}
}

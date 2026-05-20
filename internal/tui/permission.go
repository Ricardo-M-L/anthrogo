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
	theme      Theme
	visible    bool
	pending    permissionAsk
	formBuffer []rune // accumulates user input for PromptElicitForm
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
	if p.pending.req.Kind == tool.PromptElicitForm {
		return p.handleElicitForm(k)
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

func (p *permission) handleElicitForm(k tea.KeyMsg) bool {
	switch k.Type {
	case tea.KeyEsc:
		p.pending.reply <- tool.PromptResponse{Action: "cancel"}
		close(p.pending.reply)
		p.hide()
		p.formBuffer = nil
		return true
	case tea.KeyEnter:
		raw := strings.TrimSpace(string(p.formBuffer))
		if raw == "" {
			p.pending.reply <- tool.PromptResponse{Action: "decline"}
			close(p.pending.reply)
			p.hide()
			p.formBuffer = nil
			return true
		}
		var data map[string]any
		if err := json.Unmarshal([]byte(raw), &data); err != nil {
			p.pending.reply <- tool.PromptResponse{Action: "decline", Reason: "invalid JSON: " + err.Error()}
			close(p.pending.reply)
			p.hide()
			p.formBuffer = nil
			return true
		}
		p.pending.reply <- tool.PromptResponse{Action: "accept", FormData: data}
		close(p.pending.reply)
		p.hide()
		p.formBuffer = nil
		return true
	case tea.KeyBackspace, tea.KeyDelete:
		if len(p.formBuffer) > 0 {
			p.formBuffer = p.formBuffer[:len(p.formBuffer)-1]
		}
		return true
	case tea.KeyRunes:
		p.formBuffer = append(p.formBuffer, k.Runes...)
		return true
	case tea.KeySpace:
		p.formBuffer = append(p.formBuffer, ' ')
		return true
	}
	return true
}

func (p permission) view() string {
	if !p.visible {
		return ""
	}
	switch p.pending.req.Kind {
	case tool.PromptElicitForm:
		var body strings.Builder
		body.WriteString("MCP elicitation\n\n")
		if p.pending.req.Message != "" {
			body.WriteString(p.pending.req.Message + "\n\n")
		}
		if p.pending.req.Schema != nil {
			rawSchema, _ := json.MarshalIndent(p.pending.req.Schema, "", "  ")
			body.WriteString("Schema:\n" + string(rawSchema) + "\n\n")
		}
		body.WriteString("Type a JSON object matching the schema, then press Enter:\n")
		body.WriteString("> " + string(p.formBuffer) + "\n\n")
		body.WriteString("[Enter] submit   [Esc] cancel/decline")
		return p.theme.ModalBorder.Padding(1, 2).Render(body.String())
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

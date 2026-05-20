package tui

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ricardo/anthrogo/pkg/tool"
)

type permissionAsk struct {
	req   tool.PromptRequest
	reply chan tool.PromptResponse
}

type formField struct {
	name        string
	typ         string // "string" | "number" | "integer" | "boolean"
	description string
	required    bool
	buffer      []rune // current input
}

type permission struct {
	theme      Theme
	visible    bool
	pending    permissionAsk
	formBuffer []rune      // accumulates user input for PromptElicitForm (single mode)
	formFields []formField // populated when multi-field mode
	formFocus  int         // currently focused field
	formMode   string      // "multi" | "single" | ""
}

func newPermission(theme Theme) permission { return permission{theme: theme} }

func (p *permission) show(req permissionAsk) {
	p.visible = true
	p.pending = req
	if req.req.Kind == tool.PromptElicitForm {
		fields, ok := parseSimpleSchema(req.req.Schema)
		if ok && len(fields) > 0 {
			p.formMode = "multi"
			p.formFields = fields
			p.formFocus = 0
			p.formBuffer = nil
		} else {
			p.formMode = "single"
			p.formFields = nil
			p.formBuffer = nil
		}
	}
}

func (p *permission) hide() {
	p.visible = false
	p.pending = permissionAsk{}
}

// parseSimpleSchema returns (fields, true) iff the schema is an object with
// only flat primitive properties.
func parseSimpleSchema(schema map[string]any) ([]formField, bool) {
	if schema == nil {
		return nil, false
	}
	typ, _ := schema["type"].(string)
	if typ != "object" {
		return nil, false
	}
	props, ok := schema["properties"].(map[string]any)
	if !ok || len(props) == 0 {
		return nil, false
	}
	requiredSet := map[string]bool{}
	if reqList, ok := schema["required"].([]any); ok {
		for _, r := range reqList {
			if s, ok := r.(string); ok {
				requiredSet[s] = true
			}
		}
	}
	// sort keys for stable order
	var names []string
	for n := range props {
		names = append(names, n)
	}
	sort.Strings(names)
	var fields []formField
	for _, name := range names {
		prop, ok := props[name].(map[string]any)
		if !ok {
			return nil, false
		}
		ptyp, _ := prop["type"].(string)
		switch ptyp {
		case "string", "number", "integer", "boolean":
			// ok
		default:
			return nil, false
		}
		desc, _ := prop["description"].(string)
		fields = append(fields, formField{
			name:        name,
			typ:         ptyp,
			description: desc,
			required:    requiredSet[name],
		})
	}
	return fields, true
}

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
	if p.formMode == "multi" {
		return p.handleElicitMulti(k)
	}
	return p.handleElicitSingle(k)
}

func (p *permission) handleElicitSingle(k tea.KeyMsg) bool {
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

func (p *permission) handleElicitMulti(k tea.KeyMsg) bool {
	switch k.Type {
	case tea.KeyEsc:
		p.pending.reply <- tool.PromptResponse{Action: "cancel"}
		close(p.pending.reply)
		p.hide()
		p.formFields = nil
		p.formMode = ""
		return true
	case tea.KeyEnter:
		data := map[string]any{}
		for _, f := range p.formFields {
			raw := strings.TrimSpace(string(f.buffer))
			if raw == "" {
				if f.required {
					p.pending.reply <- tool.PromptResponse{Action: "decline", Reason: "required field empty: " + f.name}
					close(p.pending.reply)
					p.hide()
					p.formFields = nil
					p.formMode = ""
					return true
				}
				continue // optional empty → omit
			}
			switch f.typ {
			case "string":
				data[f.name] = raw
			case "boolean":
				lower := strings.ToLower(raw)
				if lower == "true" || lower == "yes" || lower == "y" || lower == "1" {
					data[f.name] = true
				} else if lower == "false" || lower == "no" || lower == "n" || lower == "0" {
					data[f.name] = false
				} else {
					p.pending.reply <- tool.PromptResponse{Action: "decline", Reason: "invalid boolean for " + f.name}
					close(p.pending.reply)
					p.hide()
					p.formFields = nil
					p.formMode = ""
					return true
				}
			case "integer":
				n, err := strconv.Atoi(raw)
				if err != nil {
					p.pending.reply <- tool.PromptResponse{Action: "decline", Reason: "invalid integer for " + f.name + ": " + err.Error()}
					close(p.pending.reply)
					p.hide()
					p.formFields = nil
					p.formMode = ""
					return true
				}
				data[f.name] = n
			case "number":
				n, err := strconv.ParseFloat(raw, 64)
				if err != nil {
					p.pending.reply <- tool.PromptResponse{Action: "decline", Reason: "invalid number for " + f.name + ": " + err.Error()}
					close(p.pending.reply)
					p.hide()
					p.formFields = nil
					p.formMode = ""
					return true
				}
				data[f.name] = n
			}
		}
		p.pending.reply <- tool.PromptResponse{Action: "accept", FormData: data}
		close(p.pending.reply)
		p.hide()
		p.formFields = nil
		p.formMode = ""
		return true
	case tea.KeyTab:
		p.formFocus = (p.formFocus + 1) % len(p.formFields)
		return true
	case tea.KeyShiftTab:
		p.formFocus = (p.formFocus - 1 + len(p.formFields)) % len(p.formFields)
		return true
	case tea.KeyBackspace, tea.KeyDelete:
		if len(p.formFields[p.formFocus].buffer) > 0 {
			p.formFields[p.formFocus].buffer = p.formFields[p.formFocus].buffer[:len(p.formFields[p.formFocus].buffer)-1]
		}
		return true
	case tea.KeyRunes:
		p.formFields[p.formFocus].buffer = append(p.formFields[p.formFocus].buffer, k.Runes...)
		return true
	case tea.KeySpace:
		p.formFields[p.formFocus].buffer = append(p.formFields[p.formFocus].buffer, ' ')
		return true
	}
	return true
}

func renderInput(s string, focused bool) string {
	cursor := ""
	if focused {
		cursor = "█"
	}
	return fmt.Sprintf("> %s%s", s, cursor)
}

func (p permission) view() string {
	if !p.visible {
		return ""
	}
	switch p.pending.req.Kind {
	case tool.PromptElicitForm:
		if p.formMode == "multi" {
			var body strings.Builder
			body.WriteString("MCP elicitation\n\n")
			if p.pending.req.Message != "" {
				body.WriteString(p.pending.req.Message + "\n\n")
			}
			for i, f := range p.formFields {
				marker := " "
				if i == p.formFocus {
					marker = "▶"
				}
				req := ""
				if f.required {
					req = " (required)"
				}
				fmt.Fprintf(&body, "%s %s (%s)%s\n   %s\n",
					marker, f.name, f.typ, req,
					renderInput(string(f.buffer), i == p.formFocus))
				if f.description != "" {
					fmt.Fprintf(&body, "   %s\n", f.description)
				}
				body.WriteString("\n")
			}
			body.WriteString("[Tab/Shift-Tab] navigate   [Enter] submit   [Esc] cancel")
			return p.theme.ModalBorder.Padding(1, 2).Render(body.String())
		}
		// single-textarea (M6.3 fallback)
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

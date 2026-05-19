package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ricardo/anthrogo/internal/mcp"
	"github.com/ricardo/anthrogo/internal/session"
	"github.com/ricardo/anthrogo/pkg/command"
	"github.com/ricardo/anthrogo/pkg/message"
	"github.com/ricardo/anthrogo/pkg/permissions"
	"github.com/ricardo/anthrogo/pkg/provider"
	"github.com/ricardo/anthrogo/pkg/query"
	"github.com/ricardo/anthrogo/pkg/tool"
)

type Options struct {
	Provider        provider.Provider
	Tools           *tool.Registry
	Permissions     *permissions.Context
	Model           string
	SystemPrompt    string
	Cwd             string
	ClaudeMd        string
	Commands        *command.Registry
	Session         *session.Store
	InitialMessages []message.Message
	RecordHook      func(session.Record)
	MCP             *mcp.Manager
}

// serverLogMsg is dispatched via tea.Program.Send from AppendServerLog so that
// chat mutations always happen on the bubbletea Update goroutine.
type serverLogMsg struct{ server, msg string }

type App struct {
	theme      Theme
	chat       chat
	input      promptInput
	perm       permission
	opts       Options
	engine     *query.Engine
	stream     <-chan query.Event
	width      int
	height     int
	cancelTurn context.CancelFunc

	asks    chan permissionAsk
	palette palette
	cmdReg  *command.Registry
	program *tea.Program
}

func New(opts Options) *App {
	theme := DefaultTheme()
	a := &App{
		theme: theme,
		chat:  newChat(theme),
		input: newPromptInput(theme),
		perm:  newPermission(theme),
		opts:  opts,
		asks:  make(chan permissionAsk, 4),
	}
	a.cmdReg = opts.Commands
	a.palette = newPalette(theme, opts.Commands)

	a.engine = query.NewEngine(query.Config{
		Provider:     opts.Provider,
		Tools:        opts.Tools,
		Permissions:  opts.Permissions,
		Model:        opts.Model,
		SystemPrompt: opts.SystemPrompt,
		Cwd:          opts.Cwd,
		RecordHook:   opts.RecordHook,
		RequestPrompt: func(_ string, req tool.PromptRequest) (tool.PromptResponse, error) {
			reply := make(chan tool.PromptResponse, 1)
			a.asks <- permissionAsk{req: req, reply: reply}
			return <-reply, nil
		},
	})

	if len(opts.InitialMessages) > 0 {
		a.engine.SetInitialMessages(opts.InitialMessages)
		for _, m := range opts.InitialMessages {
			switch m.Role {
			case message.RoleUser:
				for _, b := range m.Content {
					if b.Type == message.BlockText {
						a.chat.appendUser(b.Text)
					}
				}
			case message.RoleAssistant:
				for _, b := range m.Content {
					if b.Type == message.BlockText {
						a.chat.appendAssistantDelta(b.Text)
					}
				}
				a.chat.finishAssistant()
			}
		}
	}

	return a
}

func (a *App) Init() tea.Cmd { return waitForAsk(a.asks) }

type askMsg permissionAsk

func waitForAsk(ch <-chan permissionAsk) tea.Cmd {
	return func() tea.Msg {
		ask, ok := <-ch
		if !ok {
			return nil
		}
		return askMsg(ask)
	}
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch m := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = m.Width, m.Height
		a.chat.resize(m.Width, m.Height-4)
		a.input.setWidth(m.Width - 2)
		return a, nil

	case tea.KeyMsg:
		if a.perm.update(msg) {
			return a, nil
		}
		// Palette consumes Tab / Shift+Tab / Esc when visible
		if consumed, newInput := a.palette.handleKey(m); consumed {
			if newInput != "" {
				a.input.ti.SetValue(newInput)
			}
			return a, nil
		}
		if m.Type == tea.KeyCtrlC {
			if a.cancelTurn != nil {
				a.cancelTurn()
				a.cancelTurn = nil
				a.chat.appendError("turn cancelled")
				a.drainAsks()
				return a, nil
			}
			a.drainAsks()
			return a, tea.Quit
		}

	case submitMsg:
		text := m.text
		if strings.HasPrefix(text, "/") && a.cmdReg != nil {
			firstField := strings.Fields(text)[0]
			if cmd, ok := a.cmdReg.Lookup(firstField); ok {
				args := strings.TrimSpace(strings.TrimPrefix(text, firstField))
				a.chat.appendUser(text)
				res, err := cmd.Run(context.Background(), args, a)
				if err != nil {
					a.chat.appendError(err.Error())
				} else if res.Text != "" {
					a.chat.appendError(res.Text)
				}
				a.input.setEnabled(true)
				return a, nil
			}
		}
		a.chat.appendUser(text)
		a.input.setEnabled(false)
		turnCtx, cancel := context.WithCancel(context.Background())
		a.cancelTurn = cancel
		a.stream = a.engine.SubmitMessage(turnCtx, text)
		return a, pumpStream(a.stream)

	case engineEventMsg:
		a.handleEvent(m.ev)
		return a, pumpStream(a.stream)

	case streamClosedMsg:
		a.cancelTurn = nil
		a.input.setEnabled(true)
		return a, nil

	case askMsg:
		a.perm.show(permissionAsk(m))
		return a, waitForAsk(a.asks)

	case serverLogMsg:
		a.chat.appendServerLog(m.server, m.msg)
		return a, nil
	}

	var c tea.Cmd
	a.input, c = a.input.update(msg)
	cmds = append(cmds, c)
	a.palette.updateForInput(a.input.ti.Value())
	a.chat, c = a.chat.update(msg)
	cmds = append(cmds, c)
	return a, tea.Batch(cmds...)
}

func (a *App) handleEvent(ev query.Event) {
	switch ev.Kind {
	case query.KindAssistantDelta:
		a.chat.appendAssistantDelta(ev.Text)
	case query.KindAssistantStop:
		a.chat.finishAssistant()
	case query.KindToolUseRequest:
		a.chat.finishAssistant()
		a.chat.appendTool(ev.ToolName, fmt.Sprintf("%v", ev.ToolInput), false)
	case query.KindToolResult:
		summary := ev.Text
		if len(summary) > 200 {
			summary = summary[:200] + "…"
		}
		a.chat.appendTool(ev.ToolName, summary, ev.IsError)
	case query.KindError:
		a.chat.appendError(ev.Err.Error())
	case query.KindTurnComplete:
		a.chat.finishAssistant()
	}
}

func (a *App) drainAsks() {
	for {
		select {
		case ask := <-a.asks:
			ask.reply <- tool.PromptResponse{Allow: false, Reason: "cancelled"}
			close(ask.reply)
		default:
			return
		}
	}
}

func (a *App) View() string {
	if a.perm.view() != "" {
		return a.perm.view()
	}
	planOn := a.opts.Permissions != nil && a.opts.Permissions.Mode == permissions.ModePlan
	status := a.theme.StatusLine.Render(fmt.Sprintf("model=%s  cwd=%s", a.opts.Model, a.opts.Cwd))
	if badge := renderPlanBadge(a.theme, planOn); badge != "" {
		status = badge + "   " + status
	}
	if pal := a.palette.view(); pal != "" {
		return fmt.Sprintf("%s\n%s\n%s\n%s", a.chat.view(), pal, a.input.view(), status)
	}
	return fmt.Sprintf("%s\n%s\n%s", a.chat.view(), a.input.view(), status)
}

// command.Host implementation

func (a *App) Engine() *query.Engine                 { return a.engine }
func (a *App) Permissions() *permissions.Context     { return a.opts.Permissions }
func (a *App) Tools() *tool.Registry                 { return a.opts.Tools }
func (a *App) Session() *session.Store               { return a.opts.Session }
func (a *App) Messages() []message.Message           { return a.engine.Messages() }
func (a *App) ReplaceMessages(msgs []message.Message) { a.engine.SetInitialMessages(msgs) }
func (a *App) ResetSession() error {
	// M2 stub: clear in-memory engine state. Real file-rotation deferred to M3.
	a.engine.SetInitialMessages(nil)
	return nil
}
func (a *App) AppendUIMessage(s string)          { a.chat.appendError(s) }
func (a *App) ClaudeMd() string                  { return a.opts.ClaudeMd }
func (a *App) Quit()                             { /* triggered by tea.Quit in Update */ }
func (a *App) Cwd() string                       { return a.opts.Cwd }
func (a *App) Registry() *command.Registry       { return a.cmdReg }
func (a *App) MCP() *mcp.Manager                 { return a.opts.MCP }

// SetProgram must be called with the tea.Program before Run so that
// AppendServerLog can route through Program.Send instead of mutating chat
// directly from a background goroutine.
func (a *App) SetProgram(p *tea.Program) { a.program = p }

// AppendServerLog routes MCP log lines through the bubbletea event loop to
// avoid data races with other chat mutations that run on the Update goroutine.
func (a *App) AppendServerLog(server, msg string) {
	if a.program != nil {
		a.program.Send(serverLogMsg{server: server, msg: msg})
		return
	}
	// pre-Run fallback (rare; e.g. during boot before SetProgram)
	a.chat.appendServerLog(server, msg)
}

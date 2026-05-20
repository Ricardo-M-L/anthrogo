package tui

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ricardo/anthrogo/internal/hooks"
	"github.com/ricardo/anthrogo/internal/mcp"
	"github.com/ricardo/anthrogo/internal/session"
	"github.com/ricardo/anthrogo/pkg/command"
	"github.com/ricardo/anthrogo/pkg/message"
	"github.com/ricardo/anthrogo/pkg/permissions"
	"github.com/ricardo/anthrogo/pkg/pricing"
	"github.com/ricardo/anthrogo/pkg/provider"
	"github.com/ricardo/anthrogo/pkg/query"
	"github.com/ricardo/anthrogo/pkg/skill"
	"github.com/ricardo/anthrogo/pkg/subagent"
	"github.com/ricardo/anthrogo/pkg/tool"
)

// PromptHookSink is the subset of hooks.Manager the TUI (and engine via tui.New) needs.
// Includes both per-prompt methods and tool-level methods so the same concrete
// value (*hooks.Manager) can be forwarded to query.Config.Hooks without a cast.
// All methods are nil-safe when the Hooks field is nil.
type PromptHookSink interface {
	FireUserPromptSubmit(ctx context.Context, prompt string) (context.Context, string, bool, string)
	FireSessionStart(ctx context.Context, kind string)
	FireSessionEnd(ctx context.Context, kind string)
	FireNotification(ctx context.Context, message, kind string)
	FirePostToolUse(ctx context.Context, toolName string, input, response map[string]any) string
	FireStop(ctx context.Context, reason string)
	FirePreCompact(ctx context.Context, trigger string)
	FireSubagentStop(ctx context.Context, reason string)
}

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
	Hooks       PromptHookSink
	// HooksConfig is the raw hooks.Config used to build the Hooks manager.
	// Forwarded to the engine so KAIROS workers can inherit the client's hook rules.
	HooksConfig *hooks.Config
	Skills      *skill.Registry
	Subagents       *subagent.Registry
	// Plugins is the *plugin.Registry. Typed as any to avoid an import cycle
	// between tui and pkg/plugin (which imports pkg/command which tui uses).
	Plugins any
	// OnEngineReady, if non-nil, is called with the newly-constructed engine
	// before New returns. Callers can use this to wire deferred runners (e.g.
	// the Task tool's runner).
	OnEngineReady func(*query.Engine)

	// AutoCompactThreshold and AutoCompactKeepRecent are forwarded to the engine.
	AutoCompactThreshold  int
	AutoCompactKeepRecent int

	// Pricing is the optional pricing table for cost tracking. nil = disabled.
	Pricing *pricing.Table

	// CostLimitUSD, when > 0, enables hard budget enforcement via IsOverBudget().
	CostLimitUSD float64

	// KairosTrustKey, when non-nil, is the global ed25519 public key for verifying
	// SSE signatures on all KAIROS subagent dispatches.
	KairosTrustKey ed25519.PublicKey

	// Theme, when non-nil, overrides the default dark theme.
	Theme *Theme
}

type layout int

const (
	layoutSingle layout = iota
	layoutSplit
	layoutTriple
)

func (l layout) String() string {
	switch l {
	case layoutSplit:
		return "split"
	case layoutTriple:
		return "triple"
	default:
		return "single"
	}
}

// serverLogMsg is dispatched via tea.Program.Send from AppendServerLog so that
// chat mutations always happen on the bubbletea Update goroutine.
type serverLogMsg struct{ server, msg string }

// hookLogMsg is dispatched via tea.Program.Send from AppendHookLog so that
// chat mutations always happen on the bubbletea Update goroutine.
type hookLogMsg struct{ event, msg string }

// execCompleteMsg is sent after tea.ExecProcess returns from a subprocess
// (e.g. $EDITOR). The string payload is the OnComplete-returned status line.
type execCompleteMsg string

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

	asks       chan permissionAsk
	palette    palette
	cmdReg     *command.Registry
	program    *tea.Program
	layout     layout
	logPane    logPane
	statusPane statusPane
}

func New(opts Options) *App {
	theme := DefaultTheme()
	if opts.Theme != nil {
		theme = *opts.Theme
	}
	historyPath := filepath.Join(os.Getenv("HOME"), ".anthrogo", "input_history")
	a := &App{
		theme: theme,
		chat:  newChat(theme),
		input: newPromptInput(theme, historyPath),
		perm:  newPermission(theme),
		opts:  opts,
		asks:  make(chan permissionAsk, 4),
	}
	a.cmdReg = opts.Commands
	a.palette = newPalette(theme, opts.Commands)
	a.logPane = newLogPane(theme)
	a.statusPane = statusPane{
		theme:    theme,
		getMode:  func() permissions.Mode {
			if opts.Permissions != nil {
				return opts.Permissions.Mode
			}
			return permissions.ModeDefault
		},
		getCwd:       func() string { return opts.Cwd },
		getModel:     func() string { return opts.Model },
		getUsageLine: func() string { return a.formatUsageLine() },
	}

	a.engine = query.NewEngine(query.Config{
		Provider:              opts.Provider,
		Tools:                 opts.Tools,
		Permissions:           opts.Permissions,
		Model:                 opts.Model,
		SystemPrompt:          opts.SystemPrompt,
		Cwd:                   opts.Cwd,
		RecordHook:            opts.RecordHook,
		Hooks:                 opts.Hooks,
		HooksConfig:           opts.HooksConfig,
		Session:               opts.Session,
		SubagentRegistry:      opts.Subagents,
		RequestPrompt:         a.RequestPrompt,
		AutoCompactThreshold:  opts.AutoCompactThreshold,
		AutoCompactKeepRecent: opts.AutoCompactKeepRecent,
		Pricing:               opts.Pricing,
		CostLimitUSD:          opts.CostLimitUSD,
		KairosTrustKey:        opts.KairosTrustKey,
	})
	if opts.OnEngineReady != nil {
		opts.OnEngineReady(a.engine)
	}

	// Fire SessionStart hook now that construction is complete.
	if opts.Hooks != nil {
		kind := "new"
		if len(opts.InitialMessages) > 0 {
			kind = "resume"
		}
		opts.Hooks.FireSessionStart(context.Background(), kind)
	}

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

type tickMsg time.Time

func tickEvery() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (a *App) Init() tea.Cmd { return tea.Batch(waitForAsk(a.asks), tickEvery()) }

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
		a.applyLayout()
		a.input.setWidth(m.Width - 2)
		return a, nil

	case tea.KeyMsg:
		if a.perm.update(msg) {
			return a, nil
		}
		if m.Type == tea.KeyF2 {
			a.layout = (a.layout + 1) % 3
			a.applyLayout()
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
		// Run UserPromptSubmit hooks before dispatching.
		if a.opts.Hooks != nil {
			_, finalPrompt, abort, reason := a.opts.Hooks.FireUserPromptSubmit(context.Background(), text)
			if abort {
				a.chat.appendError("user prompt blocked: " + reason)
				return a, nil
			}
			text = finalPrompt
		}
		if strings.HasPrefix(text, "/") && a.cmdReg != nil {
			firstField := strings.Fields(text)[0]
			if cmd, ok := a.cmdReg.Lookup(firstField); ok {
				args := strings.TrimSpace(strings.TrimPrefix(text, firstField))
				a.chat.appendUser(text)
				res, err := cmd.Run(context.Background(), args, a)
				if err != nil {
					a.chat.appendError(err.Error())
					a.input.setEnabled(true)
					return a, nil
				}
				if res.ExecCmd != nil && res.ExecCmd.Cmd != nil {
					// Suspend the TUI, run the editor, resume on exit.
					if res.Text != "" {
						a.chat.appendError(res.Text)
					}
					onComplete := res.ExecCmd.OnComplete
					execCmd := tea.ExecProcess(res.ExecCmd.Cmd, func(execErr error) tea.Msg {
						status := ""
						if onComplete != nil {
							status = onComplete(execErr)
						} else if execErr != nil {
							status = "exec error: " + execErr.Error()
						}
						return execCompleteMsg(status)
					})
					a.input.setEnabled(true)
					return a, execCmd
				}
				if res.Text != "" {
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
		blocks, err := message.ParseUserPrompt(text)
		if err != nil {
			a.chat.appendError("prompt: " + err.Error())
			a.input.setEnabled(true)
			cancel()
			a.cancelTurn = nil
			return a, nil
		}
		if len(blocks) == 0 {
			blocks = []message.Block{{Type: message.BlockText, Text: text}}
		}
		a.stream = a.engine.SubmitMessageBlocks(turnCtx, blocks)
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
		a.logPane.append(fmt.Sprintf("[mcp:%s] %s", m.server, m.msg))
		if a.layout == layoutSingle {
			a.chat.appendServerLog(m.server, m.msg)
		}
		return a, nil

	case hookLogMsg:
		a.logPane.append(fmt.Sprintf("[hook:%s] %s", m.event, m.msg))
		if a.layout == layoutSingle {
			a.chat.appendHookLog(m.event, m.msg)
		}
		return a, nil

	case execCompleteMsg:
		if string(m) != "" {
			a.chat.appendError(string(m))
		}
		return a, nil

	case tea.MouseMsg:
		return a.handleMouse(m)

	case tickMsg:
		// 1Hz tick — re-calls View() to repaint the status line live.
		return a, tickEvery()
	}

	var c tea.Cmd
	a.input, c = a.input.update(msg)
	cmds = append(cmds, c)
	a.palette.updateForInput(a.input.ti.Value())
	c = a.chat.update(msg)
	cmds = append(cmds, c)
	return a, tea.Batch(cmds...)
}

// urlRegex matches http and https URLs in plain text.
var urlRegex = regexp.MustCompile(`https?://[^\s\]\)>"]+`)

// ansiRegex strips ANSI escape sequences.
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

func openInBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

func (a *App) handleMouse(m tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch m.Button {
	case tea.MouseButtonWheelUp:
		return a, a.chat.scrollUp()
	case tea.MouseButtonWheelDown:
		return a, a.chat.scrollDown()
	case tea.MouseButtonLeft:
		// Released-after-click handling: detect URL under the click position.
		if m.Action == tea.MouseActionRelease {
			url := a.urlAtPosition(m.X, m.Y)
			if url != "" {
				if err := openInBrowser(url); err != nil {
					a.chat.appendError("open URL: " + err.Error())
				}
			}
		}
		return a, nil
	}
	return a, nil
}

func (a *App) urlAtPosition(x, y int) string {
	a.chat.mu.Lock()
	lines := a.chat.lines
	a.chat.mu.Unlock()
	if y < 0 || y >= len(lines) {
		return ""
	}
	line := lines[y].rawText
	if line == "" {
		line = stripANSI(lines[y].rendered)
	}
	urls := urlRegex.FindAllString(line, -1)
	if len(urls) == 0 {
		return ""
	}
	closest := urls[0]
	bestDist := 999999
	for _, u := range urls {
		col := strings.Index(line, u)
		dist := col - x
		if dist < 0 {
			dist = -dist
		}
		if dist < bestDist {
			closest = u
			bestDist = dist
		}
	}
	return closest
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

// applyLayout resizes panes based on the current layout and terminal dimensions.
func (a *App) applyLayout() {
	w, h := a.width, a.height
	// Reserve input row + status row + 1 separator at bottom.
	contentH := h - 3
	if contentH < 1 {
		contentH = 1
	}
	switch a.layout {
	case layoutSingle:
		a.chat.resize(w, contentH)
	case layoutSplit:
		topH := contentH * 7 / 10
		botH := contentH - topH
		if botH < 1 {
			botH = 1
		}
		a.chat.resize(w, topH)
		a.logPane.resize(w, botH)
	case layoutTriple:
		sideW := w * 3 / 10
		mainW := w - sideW
		if mainW < 1 {
			mainW = 1
		}
		a.chat.resize(mainW, contentH)
		a.statusPane.width = sideW
		a.statusPane.height = contentH
	}
}

// formatUsageLine returns the token/cost portion of the status line.
func (a *App) formatUsageLine() string {
	lu := a.engine.Usage()
	sc := a.engine.UsageSinceLastCompact()
	sinceTotal := sc.InputTokens + sc.OutputTokens
	s := fmt.Sprintf("tok: %sin/%sout (since: %s)",
		formatTokens(lu.InputTokens), formatTokens(lu.OutputTokens),
		formatTokens(sinceTotal))
	if a.opts.AutoCompactThreshold > 0 {
		s += fmt.Sprintf(" [⚙ %s]", formatTokens(a.opts.AutoCompactThreshold))
	}
	if usd, ok := a.engine.EstimatedCost(); ok {
		s += fmt.Sprintf("  $%.4f", usd)
	}
	return s
}

// statusLineString builds the bottom status bar including F2 hint.
func (a *App) statusLineString() string {
	planOn := a.opts.Permissions != nil && a.opts.Permissions.Mode == permissions.ModePlan
	tokenInfo := a.formatUsageLine()
	hint := fmt.Sprintf("[F2: %s]", a.layout)
	status := a.theme.StatusLine.Render(fmt.Sprintf("model=%s  cwd=%s  %s  %s", a.opts.Model, a.opts.Cwd, tokenInfo, hint))
	if badge := renderPlanBadge(a.theme, planOn); badge != "" {
		status = badge + "   " + status
	}
	return status
}

func (a *App) View() string {
	if a.perm.view() != "" {
		return a.perm.view()
	}
	status := a.statusLineString()
	if pal := a.palette.view(); pal != "" {
		return fmt.Sprintf("%s\n%s\n%s\n%s", a.chat.view(), pal, a.input.view(), status)
	}
	switch a.layout {
	case layoutSplit:
		return fmt.Sprintf("%s\n%s\n%s\n%s", a.chat.view(), a.logPane.view(), a.input.view(), status)
	case layoutTriple:
		top := lipgloss.JoinHorizontal(lipgloss.Top, a.chat.view(), a.statusPane.view())
		return fmt.Sprintf("%s\n%s\n%s", top, a.input.view(), status)
	default: // layoutSingle
		return fmt.Sprintf("%s\n%s\n%s", a.chat.view(), a.input.view(), status)
	}
}

// formatTokens formats an integer token count with comma-thousands separators.
func formatTokens(n int) string {
	if n == 0 {
		return "0"
	}
	s := fmt.Sprintf("%d", n)
	// Insert commas every 3 digits from the right.
	out := make([]byte, 0, len(s)+len(s)/3)
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, byte(c))
	}
	return string(out)
}

// RequestPrompt routes a prompt request through the TUI permission modal.
// It is safe to call from any goroutine (e.g. the MCP elicitation handler).
func (a *App) RequestPrompt(source string, req tool.PromptRequest) (tool.PromptResponse, error) {
	if a.opts.Hooks != nil && req.Kind == tool.PromptToolPermission {
		a.opts.Hooks.FireNotification(context.Background(), "permission ask: "+req.ToolName, "permission_ask")
	}
	reply := make(chan tool.PromptResponse, 1)
	a.asks <- permissionAsk{req: req, reply: reply}
	return <-reply, nil
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
func (a *App) Skills() *skill.Registry           { return a.opts.Skills }
func (a *App) Subagents() *subagent.Registry     { return a.opts.Subagents }
func (a *App) Plugins() any                      { return a.opts.Plugins }

// ThemeName returns the name of the currently active theme.
func (a *App) ThemeName() string { return a.theme.Name }

// SetTheme swaps the active theme by name and propagates it to all
// sub-components. bubbletea will re-render on the next event automatically.
func (a *App) SetTheme(name string) error {
	t, ok := ThemeByName(name)
	if !ok {
		return fmt.Errorf("unknown theme: %s", name)
	}
	a.theme = t
	a.chat.theme = t
	a.input.theme = t
	a.perm.theme = t
	a.palette.theme = t
	a.logPane.theme = t
	a.statusPane.theme = t
	return nil
}

// SetProgram must be called with the tea.Program before Run so that
// AppendServerLog can route through Program.Send instead of mutating chat
// directly from a background goroutine.
func (a *App) SetProgram(p *tea.Program) { a.program = p }

// AppendServerLog routes MCP log lines through the bubbletea event loop when
// the program is running. Always pushes to logPane; also pushes to chat in
// single layout (back-compat). When program is nil (tests), mutates directly.
func (a *App) AppendServerLog(server, msg string) {
	if a.program != nil {
		a.program.Send(serverLogMsg{server: server, msg: msg})
		return
	}
	// No program running (test / headless): write directly.
	a.logPane.append(fmt.Sprintf("[mcp:%s] %s", server, msg))
	if a.layout == layoutSingle {
		a.chat.appendServerLog(server, msg)
	}
}

// AppendHookLog routes hook log lines through the bubbletea event loop when
// the program is running. Always pushes to logPane; also pushes to chat in
// single layout (back-compat). When program is nil (tests), mutates directly.
func (a *App) AppendHookLog(event, msg string) {
	if a.program != nil {
		a.program.Send(hookLogMsg{event: event, msg: msg})
		return
	}
	// No program running (test / headless): write directly.
	a.logPane.append(fmt.Sprintf("[hook:%s] %s", event, msg))
	if a.layout == layoutSingle {
		a.chat.appendHookLog(event, msg)
	}
}

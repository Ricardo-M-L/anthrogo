package builtins

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/internal/mcp"
	"github.com/ricardo/anthrogo/internal/session"
	"github.com/ricardo/anthrogo/pkg/command"
	"github.com/ricardo/anthrogo/pkg/message"
	"github.com/ricardo/anthrogo/pkg/permissions"
	"github.com/ricardo/anthrogo/pkg/query"
	"github.com/ricardo/anthrogo/pkg/skill"
	"github.com/ricardo/anthrogo/pkg/subagent"
	"github.com/ricardo/anthrogo/pkg/tool"
)

type fakeHost struct {
	perms      *permissions.Context
	tools      *tool.Registry
	cmdReg     *command.Registry
	uiMsgs     []string
	cwd        string
	claudeMd   string
	mgr        *mcp.Manager
	engine     *query.Engine
	skills     *skill.Registry
	subagents  *subagent.Registry
	plugins    any
}

func newFakeHost() *fakeHost {
	return &fakeHost{perms: permissions.Empty(), tools: tool.NewRegistry()}
}

func (f *fakeHost) Engine() *query.Engine             { return f.engine }
func (f *fakeHost) Permissions() *permissions.Context { return f.perms }
func (f *fakeHost) Tools() *tool.Registry             { return f.tools }
func (f *fakeHost) Session() *session.Store           { return nil }
func (f *fakeHost) Messages() []message.Message       { return nil }
func (f *fakeHost) ReplaceMessages([]message.Message) {}
func (f *fakeHost) ResetSession() error               { return nil }
func (f *fakeHost) AppendUIMessage(s string)          { f.uiMsgs = append(f.uiMsgs, s) }
func (f *fakeHost) ClaudeMd() string                  { return f.claudeMd }
func (f *fakeHost) Quit()                             {}
func (f *fakeHost) Cwd() string                       { return f.cwd }
func (f *fakeHost) Registry() *command.Registry       { return f.cmdReg }
func (f *fakeHost) MCP() *mcp.Manager                 { return f.mgr }
func (f *fakeHost) Skills() *skill.Registry           { return f.skills }
func (f *fakeHost) Subagents() *subagent.Registry     { return f.subagents }
func (f *fakeHost) Plugins() any                      { return f.plugins }

func TestHelp_ListsRegisteredCommands(t *testing.T) {
	h := newFakeHost()
	h.cmdReg = command.NewRegistry()
	help := &Help{Reg: h.cmdReg}
	h.cmdReg.Register(help)
	h.cmdReg.Register(Tools{})
	res, err := help.Run(context.Background(), "", h)
	require.NoError(t, err)
	require.Contains(t, res.Text, "/help")
	require.Contains(t, res.Text, "/tools")
}

func TestTools_ListsRegistered(t *testing.T) {
	h := newFakeHost()
	h.tools.Register(tool.Read{})
	h.tools.Register(tool.Bash{})
	res, err := (Tools{}).Run(context.Background(), "", h)
	require.NoError(t, err)
	require.Contains(t, res.Text, "Read")
	require.Contains(t, res.Text, "Bash")
}

func TestMemory_PrintsClaudeMd(t *testing.T) {
	h := newFakeHost()
	h.claudeMd = "# rules"
	res, _ := (Memory{}).Run(context.Background(), "", h)
	require.Contains(t, res.Text, "# rules")
}

func TestCwd_PrintsAndChanges(t *testing.T) {
	h := newFakeHost()
	h.cwd = filepath.Clean("/tmp")
	res, _ := (Cwd{}).Run(context.Background(), "", h)
	require.Contains(t, res.Text, "/tmp")
}

func TestAddDir_AddsAdditional(t *testing.T) {
	h := newFakeHost()
	_, err := (AddDir{}).Run(context.Background(), "/foo", h)
	require.NoError(t, err)
	require.Contains(t, h.perms.AdditionalWorkingDirectories, "/foo")
}

func TestClear_ResetsSession(t *testing.T) {
	h := newFakeHost()
	res, err := (Clear{}).Run(context.Background(), "", h)
	require.NoError(t, err)
	require.Contains(t, res.Text, "cleared")
}

func TestMode_ParsesValidValue(t *testing.T) {
	h := newFakeHost()
	res, _ := (Mode{}).Run(context.Background(), "acceptEdits", h)
	require.Contains(t, res.Text, "acceptEdits")
	require.Equal(t, permissions.ModeAcceptEdits, h.perms.Mode)
}

func TestMode_RejectsUnknown(t *testing.T) {
	h := newFakeHost()
	res, _ := (Mode{}).Run(context.Background(), "yolo", h)
	require.Contains(t, res.Text, "usage")
}

func TestModel_ListsWhenEmpty(t *testing.T) {
	h := newFakeHost()
	res, _ := (Model{}).Run(context.Background(), "", h)
	require.Contains(t, res.Text, "available models")
}

func TestCompact_NoEngine(t *testing.T) {
	h := newFakeHost() // engine is nil
	res, _ := (Compact{}).Run(context.Background(), "", h)
	require.Contains(t, res.Text, "no active engine")
}

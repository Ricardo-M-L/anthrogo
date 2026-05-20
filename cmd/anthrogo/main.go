package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/ricardo/anthrogo/internal/config"
	"github.com/ricardo/anthrogo/internal/headless"
	"github.com/ricardo/anthrogo/internal/hooks"
	"github.com/ricardo/anthrogo/internal/mcp"
	"github.com/ricardo/anthrogo/internal/session"
	"github.com/ricardo/anthrogo/internal/system"
	"github.com/ricardo/anthrogo/internal/tui"
	"github.com/ricardo/anthrogo/internal/version"
	"github.com/ricardo/anthrogo/pkg/command"
	"github.com/ricardo/anthrogo/pkg/command/builtins"
	"github.com/ricardo/anthrogo/pkg/message"
	"github.com/ricardo/anthrogo/pkg/permissions"
	"github.com/ricardo/anthrogo/pkg/plugin"
	"github.com/ricardo/anthrogo/pkg/provider/anthropic"
	"github.com/ricardo/anthrogo/pkg/skill"
	"github.com/ricardo/anthrogo/pkg/tool"
)

func main() {
	var (
		prompt    string
		modelFlag string
		modeFlag  string
		cwdFlag   string
		resumeID  string
		cont      bool
		showVer   bool
	)

	root := &cobra.Command{
		Use:   "anthrogo",
		Short: "anthrogo — Go port of Claude Code",
		RunE: func(cmd *cobra.Command, args []string) error {
			if showVer {
				fmt.Println("anthrogo", version.Version)
				return nil
			}
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			if modelFlag != "" {
				cfg.Model = modelFlag
			}
			if modeFlag != "" {
				cfg.Mode = permissions.Mode(modeFlag)
			}
			perms := cfg.ToPermissionContext()
			// Skill tool is benign on its own (returns prepared markdown); ship a CLI-level
			// alwaysAllow so it doesn't prompt by default. User alwaysDeny / PreToolUse
			// hooks still take precedence (deny > allow in the gate).
			if perms.AlwaysAllowRules == nil {
				perms.AlwaysAllowRules = permissions.RulesBySource{}
			}
			perms.AlwaysAllowRules[permissions.SourceCLI] = append(
				perms.AlwaysAllowRules[permissions.SourceCLI],
				permissions.Rule{Tool: "Skill", Source: permissions.SourceCLI},
			)
			// Validate hook configuration; print any warnings but don't abort.
			for _, w := range cfg.Hooks.Validate() {
				fmt.Fprintln(os.Stderr, "hooks:", w)
			}
			cwd, err := resolveCwd(cwdFlag)
			if err != nil {
				return err
			}
			// Load project-level hooks overlay from <cwd>/.anthrogo/hooks.yaml if present.
			overlayPath := filepath.Join(cwd, ".anthrogo", "hooks.yaml")
			if raw, readErr := os.ReadFile(overlayPath); readErr == nil {
				var overlay hooks.Config
				if unmarshalErr := yaml.Unmarshal(raw, &overlay); unmarshalErr != nil {
					fmt.Fprintln(os.Stderr, "hooks overlay:", unmarshalErr)
				} else {
					overlay.Expand()
					cfg.Hooks = cfg.Hooks.AppendOverlay(overlay)
					// Re-validate after merging.
					for _, w := range cfg.Hooks.Validate() {
						fmt.Fprintln(os.Stderr, "hooks:", w)
					}
				}
			}

			// Bring up MCP servers and merge their tools into the registry.
			// logSinkRef is an atomic pointer so the MCP reader goroutine can
			// load it safely after the TUI's tea.Program is set up.
			// appRef is a separate atomic pointer for hook log routing.
			var logSinkRef atomic.Pointer[func(string, string)]
			var appRef atomic.Pointer[tui.App]
			mcpMgr := mcp.NewManager(func(name, msg string) {
				if f := logSinkRef.Load(); f != nil {
					(*f)(name, msg)
					return
				}
				fmt.Fprintf(os.Stderr, "[mcp:%s] %s\n", name, msg)
			})
			for name, scfg := range cfg.MCPServers {
				mcpMgr.AddServer(name, scfg)
			}
			// Use a signal-aware context so Ctrl-C during slow MCP startup
			// cancels every in-flight srv.Start instead of waiting 60 s.
			bootCtx, bootCancel := signal.NotifyContext(context.Background(), os.Interrupt)
			defer bootCancel()
			mcpStartCtx, mcpStartCancelTimeout := context.WithTimeout(bootCtx, 60*time.Second)
			_ = mcpMgr.Start(mcpStartCtx)
			mcpStartCancelTimeout()
			defer mcpMgr.Close()

			homeSkillsRoot := config.SkillsDir(os.Getenv("HOME"))
			cwdSkillsRoot := filepath.Join(cwd, ".anthrogo", "skills")
			loadedSkills, skillWarnings, _ := skill.LoadAll(homeSkillsRoot, cwdSkillsRoot)
			for _, w := range skillWarnings {
				fmt.Fprintln(os.Stderr, "skills:", w)
			}
			skillReg := skill.NewRegistry(loadedSkills)

			// Load plugins from ~/.anthrogo/plugins/ and <cwd>/.anthrogo/plugins/.
			homePluginsRoot := filepath.Join(os.Getenv("HOME"), ".anthrogo", "plugins")
			cwdPluginsRoot := filepath.Join(cwd, ".anthrogo", "plugins")
			loadedPlugins, pluginWarnings, _ := plugin.LoadAll(homePluginsRoot, cwdPluginsRoot)
			for _, w := range pluginWarnings {
				fmt.Fprintln(os.Stderr, "plugins:", w)
			}
			pluginReg := plugin.NewRegistry(loadedPlugins)

			// Merge plugin skill contributions into the skill registry.
			for _, p := range loadedPlugins {
				for _, s := range p.Skills {
					if !skillReg.Add(s) {
						fmt.Fprintf(os.Stderr, "plugins: skill %s from %s skipped (already loaded)\n", s.Name, p.Name)
					}
				}
			}

			// Merge plugin hook contributions into cfg.Hooks BEFORE hookMgr is built.
			for _, p := range loadedPlugins {
				cfg.Hooks = cfg.Hooks.AppendOverlay(p.Hooks)
			}
			for _, w := range cfg.Hooks.Validate() {
				fmt.Fprintln(os.Stderr, "hooks (after plugins):", w)
			}

			// Merge plugin MCP server contributions.
			for _, p := range loadedPlugins {
				for name, mcfg := range p.MCPServers {
					mcpMgr.AddServer(name, mcfg)
				}
			}

			tools := registerTools(cfg)
			for _, t := range mcpMgr.AllTools() {
				tools.Register(t)
			}
			tools.Register(tool.NewSkill(skillReg))
			claudeMd, _ := system.LoadClaudeMd(cwd, os.Getenv("HOME"))
			gitStatus, _ := system.GitStatusSnapshot(cwd)
			systemPrompt := system.BuildSystemPrompt(system.Options{
				ToolNames:   toolNameList(tools),
				ClaudeMd:    claudeMd,
				GitStatus:   gitStatus,
				CurrentDate: time.Now().Format("2006-01-02"),
				Cwd:         cwd,
				PlanModeOn:  cfg.Mode == permissions.ModePlan,
				Skills:      skillReg.List(),
			})

			var sess *session.Store
			var initialMessages []message.Message
			switch {
			case resumeID != "":
				full, err := session.ResolveResume(cwd, resumeID)
				if err != nil {
					return err
				}
				sess, err = session.Resume(cwd, full)
				if err != nil {
					return err
				}
				records, err := session.Replay(sess.Path())
				if err != nil {
					return err
				}
				initialMessages = session.Messages(records)
			case cont:
				full, err := session.ResolveContinue(cwd)
				if err != nil {
					return err
				}
				sess, err = session.Resume(cwd, full)
				if err != nil {
					return err
				}
				records, err := session.Replay(sess.Path())
				if err != nil {
					return err
				}
				initialMessages = session.Messages(records)
			default:
				sess, err = session.New(session.NewOptions{Cwd: cwd, Model: cfg.Model, PermissionMode: string(cfg.Mode)})
				if err != nil {
					return err
				}
			}
			defer sess.Close()

			// Build the hooks Manager and wire it into the permission gate.
			hookMgr := hooks.NewManager(cfg.Hooks, hooks.ManagerOptions{
				SessionID: sess.ID(),
				Cwd:       cwd,
				Version:   version.Version,
				LogSink: func(event, msg string) {
					if a := appRef.Load(); a != nil {
						(*a).AppendHookLog(event, msg)
						return
					}
					fmt.Fprintf(os.Stderr, "[hook:%s] %s\n", event, msg)
				},
			})
			defer func() {
				hookMgr.FireSessionEnd(context.Background(), "user_quit")
				hookMgr.Drain(5 * time.Second)
			}()

			perms.HookDecide = func(toolName string, input map[string]any) permissions.HookOutcome {
				d := hookMgr.FirePreToolUse(context.Background(), toolName, input)
				switch d.Behavior {
				case hooks.DecisionAllow:
					return permissions.HookOutcome{Allow: true, Reason: d.Reason, ModifiedInput: d.ModifiedInput}
				case hooks.DecisionDeny:
					return permissions.HookOutcome{Deny: true, Reason: d.Reason, ModifiedInput: d.ModifiedInput}
				default:
					return permissions.HookOutcome{Pass: true, ModifiedInput: d.ModifiedInput}
				}
			}

			p := anthropic.New(cfg.APIKey, cfg.Model)

			if prompt != "" {
				perms.ShouldAvoidPrompts = true
				return headless.Run(context.Background(), headless.Options{
					Prompt:          prompt,
					Model:           cfg.Model,
					SystemPrompt:    systemPrompt,
					Cwd:             cwd,
					Provider:        p,
					Tools:           tools,
					Permissions:     perms,
					RecordHook:      sess.NewRecordHook(),
					InitialMessages: initialMessages,
					Stdout:          os.Stdout,
					Stderr:          os.Stderr,
					Hooks:           hookMgr,
				})
			}

			cmds := registerCommands(homeSkillsRoot, cwdSkillsRoot)
			// Register plugin commands; warn on duplicates (last-writer-wins).
			for _, p := range loadedPlugins {
				for _, c := range p.Commands {
					if _, exists := cmds.Lookup(c.Name()); exists {
						fmt.Fprintf(os.Stderr, "plugins: command %s from %s shadows an existing command\n", c.Name(), p.Name)
					}
					cmds.Register(c)
				}
			}
			cmds.Register(builtins.Plugin{HomeRoot: homePluginsRoot, CwdRoot: cwdPluginsRoot})
			app := tui.New(tui.Options{
				Provider:        p,
				Tools:           tools,
				Permissions:     perms,
				Model:           cfg.Model,
				SystemPrompt:    systemPrompt,
				Cwd:             cwd,
				ClaudeMd:        claudeMd,
				Session:         sess,
				Commands:        cmds,
				InitialMessages: initialMessages,
				RecordHook:      sess.NewRecordHook(),
				MCP:             mcpMgr,
				Hooks:           hookMgr,
				Skills:          skillReg,
				Plugins:         pluginReg,
			})
			program := tea.NewProgram(app, tea.WithAltScreen())
			app.SetProgram(program)
			appRef.Store(app)
			appender := app.AppendServerLog
			logSinkRef.Store(&appender)
			_, err = program.Run()
			return err
		},
	}

	root.Flags().StringVarP(&prompt, "print", "p", "", "Headless mode: run prompt and exit")
	root.Flags().StringVar(&modelFlag, "model", "", "Override model from settings.yaml")
	root.Flags().StringVar(&modeFlag, "permission-mode", "", "default | plan | acceptEdits | bypassPermissions")
	root.Flags().StringVar(&cwdFlag, "cwd", "", "Override working directory")
	root.Flags().StringVarP(&resumeID, "resume", "r", "", "Resume a session by id prefix")
	root.Flags().BoolVarP(&cont, "continue", "c", false, "Resume the most-recent session for this cwd")
	root.Flags().BoolVar(&showVer, "version", false, "Print version and exit")

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func registerTools(cfg config.Config) *tool.Registry {
	r := tool.NewRegistry()
	r.Register(tool.Bash{})
	r.Register(tool.Read{})
	r.Register(tool.Write{})
	r.Register(tool.Edit{})
	r.Register(tool.Glob{})
	r.Register(tool.Grep{})
	r.Register(&tool.TodoWrite{})
	r.Register(tool.NewWebFetch())
	r.Register(tool.NewWebSearch(tool.WebSearchConfig{
		Backend:  cfg.WebSearch.Backend,
		APIKey:   os.ExpandEnv(cfg.WebSearch.APIKey),
		Endpoint: cfg.WebSearch.Endpoint,
	}))
	r.Register(tool.AskUserQuestion{})
	r.Register(tool.NotebookEdit{})
	r.Register(tool.EnterPlanMode{})
	r.Register(tool.ExitPlanMode{})
	return r
}

func registerCommands(skillsHome, skillsCwd string) *command.Registry {
	reg := command.NewRegistry()
	reg.Register(&builtins.Help{Reg: reg})
	reg.Register(builtins.Tools{})
	reg.Register(builtins.Memory{})
	reg.Register(builtins.Cwd{})
	reg.Register(builtins.AddDir{})
	reg.Register(builtins.Clear{})
	reg.Register(builtins.Compact{})
	reg.Register(builtins.Resume{})
	reg.Register(builtins.Model{})
	reg.Register(builtins.Mode{})
	reg.Register(builtins.MCP{})
	reg.Register(builtins.Skills{HomeRoot: skillsHome, CwdRoot: skillsCwd})
	return reg
}

func toolNameList(r *tool.Registry) []string {
	all := r.All()
	out := make([]string, 0, len(all))
	for _, t := range all {
		out = append(out, t.Name())
	}
	return out
}

func resolveCwd(flag string) (string, error) {
	if flag != "" {
		return flag, nil
	}
	return os.Getwd()
}

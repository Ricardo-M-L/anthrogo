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

	"strings"

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
	"github.com/ricardo/anthrogo/pkg/kairos"
	"github.com/ricardo/anthrogo/pkg/message"
	"github.com/ricardo/anthrogo/pkg/permissions"
	"github.com/ricardo/anthrogo/pkg/plugin"
	"github.com/ricardo/anthrogo/pkg/pricing"
	"github.com/ricardo/anthrogo/pkg/provider"
	"github.com/ricardo/anthrogo/pkg/provider/anthropic"
	openaiProvider "github.com/ricardo/anthrogo/pkg/provider/openai"
	"github.com/ricardo/anthrogo/pkg/query"
	"github.com/ricardo/anthrogo/pkg/skill"
	"github.com/ricardo/anthrogo/pkg/subagent"
	"github.com/ricardo/anthrogo/pkg/tool"
)

func main() {
	var (
		prompt              string
		modelFlag           string
		modeFlag            string
		cwdFlag             string
		resumeID            string
		cont                bool
		showVer             bool
		kairosServeAddr     string
		providerFlag        string
		autoCompactFlag     int
		costLimitFlag       float64
	)

	root := &cobra.Command{
		Use:   "anthrogo",
		Short: "anthrogo — Go port of Claude Code",
		RunE: func(cmd *cobra.Command, args []string) error {
			if showVer {
				fmt.Println("anthrogo", version.Version)
				return nil
			}

			// --kairos-serve: run as a KAIROS worker. Build a minimal engine
			// setup (provider + tools + permissions from config) and serve subagent
			// requests over HTTP. The worker excludes Remote subagent types from its
			// own registry to prevent multi-hop redirect loops.
			if kairosServeAddr != "" {
				cfg, err := config.Load()
				if err != nil {
					return err
				}
				if modelFlag != "" {
					cfg.Model = modelFlag
				}
				workerPerms := cfg.ToPermissionContext()
				workerCwd, err := resolveCwd(cwdFlag)
				if err != nil {
					return err
				}
				claudeMd, _ := system.LoadClaudeMd(workerCwd, os.Getenv("HOME"))
				gitStatus, _ := system.GitStatusSnapshot(workerCwd)
				workerTools := registerTools(cfg)
				workerSystemPrompt := system.BuildSystemPrompt(system.Options{
					ToolNames:   toolNameList(workerTools),
					ClaudeMd:    claudeMd,
					GitStatus:   gitStatus,
					CurrentDate: time.Now().Format("2006-01-02"),
					Cwd:         workerCwd,
				})
				// Build a subagent registry that only contains local (non-Remote) types.
				// This prevents multi-hop: the worker will not forward requests to
				// another remote worker.
				workerSubReg := subagent.DefaultRegistry()
				homeSubRoot := filepath.Join(os.Getenv("HOME"), ".anthrogo", "subagents")
				cwdSubRoot := filepath.Join(workerCwd, ".anthrogo", "subagents")
				userSubs, _, _ := subagent.LoadAll(homeSubRoot, cwdSubRoot)
				for _, s := range userSubs {
					if s.Remote != nil {
						continue // exclude remote types to prevent multi-hop
					}
					if s.Name == "general-purpose" {
						continue
					}
					workerSubReg.Register(s)
				}
				p, workerModel, err := buildProvider(cfg, providerFlag)
				if err != nil {
					return err
				}
				cfg.Model = workerModel
				kHandler := func(ctx context.Context, req kairos.RunRequest, emit func(string)) (string, error) {
					eng := query.NewEngine(query.Config{
						Provider:         p,
						Model:            cfg.Model,
						Tools:            workerTools,
						Permissions:      workerPerms,
						SystemPrompt:     workerSystemPrompt,
						Cwd:              workerCwd,
						SubagentRegistry: workerSubReg,
					})
					return eng.RunSubagent(ctx, query.SubagentOptions{
						Type:        req.SubagentType,
						Description: req.Description,
						Prompt:      req.Prompt,
					})
				}
				authToken := os.Getenv("KAIROS_AUTH_TOKEN")
				srv := kairos.NewServer(kHandler, authToken)
				fmt.Fprintln(os.Stderr, "anthrogo kairos worker listening on", kairosServeAddr)
				return srv.Run(context.Background(), kairosServeAddr)
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
			if autoCompactFlag > 0 {
				cfg.AutoCompactThreshold = autoCompactFlag
			}
			if costLimitFlag > 0 {
				cfg.CostLimitUSD = costLimitFlag
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
				permissions.Rule{Tool: "MCPResource", Source: permissions.SourceCLI},
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
			// Wire TUI elicitation: when the TUI is running, route elicitation
			// requests through its permission modal. Headless mode declines.
			mcpMgr.SetElicitationHandler(func(serverName, message string, schema map[string]any) (string, map[string]any, error) {
				a := appRef.Load()
				if a == nil {
					return "decline", nil, nil
				}
				resp, err := a.RequestPrompt(serverName, tool.PromptRequest{
					Kind:    tool.PromptElicitForm,
					Message: message,
					Schema:  schema,
				})
				if err != nil {
					return "decline", nil, err
				}
				action := resp.Action
				if action == "" {
					action = "decline"
				}
				return action, resp.FormData, nil
			})
			// Use a signal-aware context so Ctrl-C during slow MCP startup
			// cancels every in-flight srv.Start instead of waiting 60 s.
			bootCtx, bootCancel := signal.NotifyContext(context.Background(), os.Interrupt)
			defer bootCancel()
			mcpStartCtx, mcpStartCancelTimeout := context.WithTimeout(bootCtx, 60*time.Second)
			_ = mcpMgr.Start(mcpStartCtx)
			mcpStartCancelTimeout()
			defer mcpMgr.Close()

			// Collect advertised MCP resources (short timeout; failures are logged, not fatal).
			resCtx, resCancel := context.WithTimeout(context.Background(), 5*time.Second)
			mcpResources := mcpMgr.AllResources(resCtx)
			resCancel()

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

			// Build the subagent registry and Task tool.
			// The Task tool's runner needs the engine (circular dependency);
			// we break the cycle by capturing engineRef via OnEngineReady.
			// engineRef is an atomic.Pointer so the Task tool's runner closure
			// (called on the SubmitMessage goroutine) can safely Load() while
			// OnEngineReady Store()s from the startup path.
			subagentReg := subagent.DefaultRegistry()
			homeSubRoot := filepath.Join(os.Getenv("HOME"), ".anthrogo", "subagents")
			cwdSubRoot := filepath.Join(cwd, ".anthrogo", "subagents")
			userSubs, swarn, _ := subagent.LoadAll(homeSubRoot, cwdSubRoot)
			for _, w := range swarn {
				fmt.Fprintln(os.Stderr, "subagents:", w)
			}
			for _, s := range userSubs {
				if s.Name == "general-purpose" {
					continue // reserved; loader already warned
				}
				subagentReg.Register(s)
			}
			var engineRef atomic.Pointer[query.Engine]
			taskTool := tool.NewTask(subagentReg, func(ctx context.Context, opts tool.TaskOptions) (string, error) {
				e := engineRef.Load()
				if e == nil {
					return "", fmt.Errorf("Task: engine not initialized")
				}
				return e.RunSubagent(ctx, query.SubagentOptions{
					Type:        opts.SubagentType,
					Description: opts.Description,
					Prompt:      opts.Prompt,
				})
			})

			tools := registerTools(cfg)
			for _, t := range mcpMgr.AllTools() {
				tools.Register(t)
			}
			tools.Register(tool.NewSkill(skillReg))
			tools.Register(taskTool)
			tools.Register(tool.NewMCPResource(mcpMgr))
			claudeMd, _ := system.LoadClaudeMd(cwd, os.Getenv("HOME"))
			gitStatus, _ := system.GitStatusSnapshot(cwd)
			sysOverlayPath := config.SystemOverlayPath(os.Getenv("HOME"))
			var userOverlay string
			if data, err := os.ReadFile(sysOverlayPath); err == nil {
				userOverlay = string(data)
			}
			systemPrompt := system.BuildSystemPrompt(system.Options{
				ToolNames:    toolNameList(tools),
				ClaudeMd:     claudeMd,
				GitStatus:    gitStatus,
				CurrentDate:  time.Now().Format("2006-01-02"),
				Cwd:          cwd,
				PlanModeOn:   cfg.Mode == permissions.ModePlan,
				Skills:       skillReg.List(),
				Subagents:    subagentReg.List(),
				MCPResources: mcpResources,
				UserOverlay:  userOverlay,
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

			hookDecide := func(toolName string, input map[string]any) permissions.HookOutcome {
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
			perms.HookDecide = func(toolName string, input map[string]any) permissions.HookOutcome {
				if e := engineRef.Load(); e != nil {
					if over, cur, lim := e.IsOverBudget(); over {
						return permissions.HookOutcome{
							Deny:   true,
							Reason: fmt.Sprintf("budget exceeded: $%.4f >= $%.2f (set --cost-limit higher or 0 to disable)", cur, lim),
						}
					}
				}
				return hookDecide(toolName, input)
			}

			p, effectiveModel, err := buildProvider(cfg, providerFlag)
			if err != nil {
				return err
			}
			cfg.Model = effectiveModel

			userRates := make(map[string]pricing.Rate, len(cfg.Pricing))
			for k, v := range cfg.Pricing {
				userRates[k] = pricing.Rate{InputPerM: v.InputPerM, OutputPerM: v.OutputPerM}
			}
			pricingTable := pricing.NewTable(pricing.MergeWithUserRates(userRates))

			if prompt != "" {
				perms.ShouldAvoidPrompts = true
				return headless.Run(context.Background(), headless.Options{
					Prompt:                prompt,
					Model:                 cfg.Model,
					SystemPrompt:          systemPrompt,
					Cwd:                   cwd,
					Provider:              p,
					Tools:                 tools,
					Permissions:           perms,
					RecordHook:            sess.NewRecordHook(),
					InitialMessages:       initialMessages,
					Stdout:                os.Stdout,
					Stderr:                os.Stderr,
					Hooks:                 hookMgr,
					Subagents:             subagentReg,
					OnEngineReady:         func(e *query.Engine) { engineRef.Store(e) },
					AutoCompactThreshold:  cfg.AutoCompactThreshold,
					AutoCompactKeepRecent: cfg.AutoCompactKeepRecent,
					Pricing:               pricingTable,
					CostLimitUSD:          cfg.CostLimitUSD,
				})
			}

			cmds := registerCommands(homeSkillsRoot, cwdSkillsRoot, homeSubRoot, cwdSubRoot)
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
				Provider:              p,
				Tools:                 tools,
				Permissions:           perms,
				Model:                 cfg.Model,
				SystemPrompt:          systemPrompt,
				Cwd:                   cwd,
				ClaudeMd:              claudeMd,
				Session:               sess,
				Commands:              cmds,
				InitialMessages:       initialMessages,
				RecordHook:            sess.NewRecordHook(),
				MCP:                   mcpMgr,
				Hooks:                 hookMgr,
				Skills:                skillReg,
				Plugins:               pluginReg,
				Subagents:             subagentReg,
				OnEngineReady:         func(e *query.Engine) { engineRef.Store(e) },
				AutoCompactThreshold:  cfg.AutoCompactThreshold,
				AutoCompactKeepRecent: cfg.AutoCompactKeepRecent,
				Pricing:               pricingTable,
				CostLimitUSD:          cfg.CostLimitUSD,
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
	root.Flags().StringVar(&kairosServeAddr, "kairos-serve", "", "Serve as a KAIROS worker on this addr (e.g. :9001)")
	root.Flags().StringVar(&providerFlag, "provider", "", "Override active provider profile (see profiles in settings.yaml)")
	root.Flags().IntVar(&autoCompactFlag, "auto-compact", 0, "Auto-compact when combined input+output tokens of the latest turn exceed this threshold (0 = disabled)")
	root.Flags().Float64Var(&costLimitFlag, "cost-limit", 0, "Deny tool calls once estimated session cost (USD) reaches this amount; 0 = disabled")

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

func registerCommands(skillsHome, skillsCwd, subagentsHome, subagentsCwd string) *command.Registry {
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
	reg.Register(builtins.Subagents{HomeRoot: subagentsHome, CwdRoot: subagentsCwd})
	reg.Register(builtins.Usage{})
	reg.Register(builtins.Cost{})
	reg.Register(builtins.Sessions{})
	reg.Register(builtins.System{})
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

// expandEnvKey expands an API key that may be in the form "env:VARNAME".
// If the value does not start with "env:", it is returned as-is.
func expandEnvKey(s string) string {
	if strings.HasPrefix(s, "env:") {
		return os.Getenv(strings.TrimPrefix(s, "env:"))
	}
	return s
}

// buildProvider selects and constructs the active provider based on config and
// optional CLI override flag. It also returns the effective model name (which
// may be overridden by the active profile).
func buildProvider(cfg config.Config, providerFlagValue string) (provider.Provider, string, error) {
	providerName := cfg.Provider
	if v := strings.TrimSpace(providerFlagValue); v != "" {
		providerName = v
	}
	switch {
	case providerName == "" || providerName == "anthropic":
		return anthropic.New(expandEnvKey(cfg.APIKey), cfg.Model), cfg.Model, nil
	default:
		prof, ok := cfg.Profiles[providerName]
		if !ok {
			return nil, "", fmt.Errorf("provider %q not found in profiles", providerName)
		}
		apiKey := expandEnvKey(prof.APIKey)
		model := cfg.Model
		if prof.Model != "" {
			model = prof.Model
		}
		switch prof.Type {
		case "openai":
			return openaiProvider.New(prof.BaseURL, apiKey), model, nil
		default:
			return nil, "", fmt.Errorf("unknown profile type %q for provider %q", prof.Type, providerName)
		}
	}
}

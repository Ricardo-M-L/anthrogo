package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/ricardo/anthrogo/internal/config"
	"github.com/ricardo/anthrogo/internal/serve"
	"github.com/ricardo/anthrogo/pkg/permissions"
	"github.com/ricardo/anthrogo/pkg/provider"
	"github.com/ricardo/anthrogo/pkg/skill"
	"github.com/ricardo/anthrogo/pkg/subagent"
	"github.com/ricardo/anthrogo/pkg/tool"
)

// resolveServeToken picks the bearer token from (in priority order)
// --token, --token-file, $ANTHROGO_SERVE_TOKEN, and warns the operator
// when --token is used since the value is visible in `ps`.
func resolveServeToken(flag, file string) (string, error) {
	if flag != "" {
		fmt.Fprintln(os.Stderr,
			"warning: --token passes the secret on the command line and is visible in `ps`. "+
				"Prefer --token-file or $ANTHROGO_SERVE_TOKEN.")
		return flag, nil
	}
	if file != "" {
		b, err := os.ReadFile(file)
		if err != nil {
			return "", fmt.Errorf("read --token-file %s: %w", file, err)
		}
		return strings.TrimSpace(string(b)), nil
	}
	if v := os.Getenv("ANTHROGO_SERVE_TOKEN"); v != "" {
		return strings.TrimSpace(v), nil
	}
	return "", nil
}

// newServeCmd constructs the `anthrogo serve` subcommand.
func newServeCmd() *cobra.Command {
	var (
		addrFlag        string
		tokenFlag       string
		tokenFileFlag   string
		corsOriginFlag  string
		sessionsDirFlag string
		modelFlag       string
		providerFlag    string
	)

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start an HTTP API server exposing the anthrogo engine",
		Long: `anthrogo serve starts a long-lived HTTP server that exposes the engine
as a REST/SSE API. Clients can create chat sessions, stream responses, list
sessions, and inspect registered tools.

Endpoints:
  POST   /v1/chat             — send a message (stream=true → SSE)
  GET    /v1/sessions         — list recent sessions (up to 100)
  GET    /v1/sessions/{id}    — fetch full JSONL records for a session
  DELETE /v1/sessions/{id}    — delete a session file
  GET    /v1/tools            — list registered tools
  GET    /v1/health           — server health and uptime
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				// Non-fatal: use zero Config.
				cfg = config.Config{}
			}

			// Override model from flag if provided.
			if modelFlag != "" {
				cfg.Model = modelFlag
			}
			if cfg.Model == "" {
				cfg.Model = "claude-opus-4-7"
			}

			// Build the tool registry.
			tools, browserTool := registerTools(cfg)
			if browserTool != nil {
				defer browserTool.Close()
			}

			// Register agentic tools: Skill, Task, MCPResource.
			// serve uses empty registries (no MCP servers loaded in daemon mode).
			skillReg := skill.NewRegistry(nil)
			subagentReg := subagent.NewRegistry()
			taskTool := tool.NewTask(subagentReg, func(ctx context.Context, opts tool.TaskOptions) (string, error) {
				return "", fmt.Errorf("Task subagents not available in serve mode")
			})
			tools.Register(tool.NewSkill(skillReg))
			tools.Register(taskTool)
			tools.Register(tool.NewMCPResource(nil))

			// Permissions: bypass for daemon mode (no interactive user present).
			perms := &permissions.Context{
				Mode:               permissions.ModeBypassPermissions,
				ShouldAvoidPrompts: true,
			}

			// ProviderFactory is called lazily per session.
			effectiveProviderFlag := providerFlag
			providerFactory := func() (provider.Provider, string, error) {
				p, model, err := buildProvider(cfg, effectiveProviderFlag)
				return p, model, err
			}

			token, err := resolveServeToken(tokenFlag, tokenFileFlag)
			if err != nil {
				return err
			}

			srvCfg := serve.Config{
				Addr:            addrFlag,
				Token:           token,
				CORSOrigin:      corsOriginFlag,
				SessionsDir:     sessionsDirFlag,
				ProviderFactory: providerFactory,
				Tools:           tools,
				Permissions:     perms,
				Model:           cfg.Model,
			}

			srv := serve.New(srvCfg)

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			fmt.Fprintf(os.Stderr, "anthrogo serve listening on http://%s\n", addrFlag)
			if err := srv.ListenAndServe(ctx); err != nil && err != context.Canceled {
				return fmt.Errorf("serve: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&addrFlag, "addr", "127.0.0.1:8765", "Listen address for the HTTP server")
	cmd.Flags().StringVar(&tokenFlag, "token", "", "Bearer auth token (insecure: visible in `ps`; prefer --token-file or $ANTHROGO_SERVE_TOKEN)")
	cmd.Flags().StringVar(&tokenFileFlag, "token-file", "", "Path to a file containing the Bearer auth token")
	cmd.Flags().StringVar(&corsOriginFlag, "cors-origin", "", "Access-Control-Allow-Origin header value (e.g. https://myapp.com)")
	cmd.Flags().StringVar(&sessionsDirFlag, "sessions-dir", "", "Override session storage directory (default: ~/.anthrogo)")
	cmd.Flags().StringVar(&modelFlag, "model", "", "Model alias (overrides settings.yaml)")
	cmd.Flags().StringVar(&providerFlag, "provider", "", "Provider profile name (overrides settings.yaml)")

	return cmd
}

package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/ricardo/anthrogo/internal/config"
	"github.com/ricardo/anthrogo/internal/serve"
	"github.com/ricardo/anthrogo/internal/web"
	"github.com/ricardo/anthrogo/pkg/permissions"
	"github.com/ricardo/anthrogo/pkg/provider"
)

// newWebCmd constructs the `anthrogo web` subcommand.
func newWebCmd() *cobra.Command {
	var (
		addrFlag        string
		tokenFlag       string
		corsOriginFlag  string
		sessionsDirFlag string
		modelFlag       string
		providerFlag    string
		noBrowserFlag   bool
	)

	cmd := &cobra.Command{
		Use:   "web",
		Short: "Start the anthrogo browser UI",
		Long: `anthrogo web starts an HTTP server that serves the embedded browser UI at /
and exposes the full REST/SSE API at /v1/*.

The browser is opened automatically unless --no-browser is passed.

Endpoints (same as 'anthrogo serve'):
  POST   /v1/chat             — send a message (stream=true → SSE)
  GET    /v1/sessions         — list recent sessions
  GET    /v1/sessions/{id}    — fetch JSONL records
  DELETE /v1/sessions/{id}    — delete a session
  GET    /v1/tools            — list registered tools
  GET    /v1/health           — health + uptime

UI:
  GET    /                    — browser SPA (index.html)
  GET    /app.js              — SPA JavaScript
  GET    /styles.css          — SPA stylesheet
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				cfg = config.Config{}
			}
			if modelFlag != "" {
				cfg.Model = modelFlag
			}
			if cfg.Model == "" {
				cfg.Model = "claude-opus-4-7"
			}

			// Resolve listen address, auto-detecting a free port if needed.
			addr, err := resolveWebAddr(addrFlag)
			if err != nil {
				return fmt.Errorf("web: resolve addr: %w", err)
			}

			tools, _ := registerTools(cfg)

			perms := &permissions.Context{
				Mode:               permissions.ModeBypassPermissions,
				ShouldAvoidPrompts: true,
			}

			effectiveProviderFlag := providerFlag
			providerFactory := func() (provider.Provider, string, error) {
				p, model, err := buildProvider(cfg, effectiveProviderFlag)
				return p, model, err
			}

			srvCfg := serve.Config{
				Addr:            addr,
				Token:           tokenFlag,
				CORSOrigin:      corsOriginFlag,
				SessionsDir:     sessionsDirFlag,
				ProviderFactory: providerFactory,
				Tools:           tools,
				Permissions:     perms,
				Model:           cfg.Model,
			}

			// Build the web handler (embedded SPA).
			webHandler, err := web.Handler()
			if err != nil {
				return fmt.Errorf("web: build handler: %w", err)
			}

			srv := serve.New(srvCfg).WithRoot(webHandler)

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			url := "http://" + addr + "/"
			fmt.Fprintf(os.Stderr, "anthrogo web listening on %s\n", url)

			if !noBrowserFlag {
				// Open the browser after the listener starts.
				// We fire it in a goroutine so we don't block ListenAndServe.
				go openBrowser(url)
			}

			if err := srv.ListenAndServe(ctx); err != nil && err != context.Canceled {
				return fmt.Errorf("web: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&addrFlag, "addr", "", "Listen address (default: auto-detect 8766–8775)")
	cmd.Flags().StringVar(&tokenFlag, "token", "", "Optional Bearer auth token")
	cmd.Flags().StringVar(&corsOriginFlag, "cors-origin", "", "Access-Control-Allow-Origin header value")
	cmd.Flags().StringVar(&sessionsDirFlag, "sessions-dir", "", "Override session storage directory")
	cmd.Flags().StringVar(&modelFlag, "model", "", "Model alias (overrides settings.yaml)")
	cmd.Flags().StringVar(&providerFlag, "provider", "", "Provider profile name (overrides settings.yaml)")
	cmd.Flags().BoolVar(&noBrowserFlag, "no-browser", false, "Do not open the browser automatically")

	return cmd
}

// resolveWebAddr returns the effective listen address.
// If addrFlag is non-empty it is used as-is.
// Otherwise we probe ports 8766–8775 and return the first available one.
func resolveWebAddr(addrFlag string) (string, error) {
	if addrFlag != "" {
		return addrFlag, nil
	}
	for port := 8766; port <= 8775; port++ {
		addr := fmt.Sprintf("127.0.0.1:%d", port)
		ln, err := net.Listen("tcp", addr)
		if err == nil {
			_ = ln.Close()
			return addr, nil
		}
	}
	return "", fmt.Errorf("no free port in range 8766–8775")
}

// openBrowser opens url in the user's default browser.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default: // linux and others
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

package builtins

import (
	"context"
	"strings"

	"github.com/ricardo/anthrogo/pkg/command"
	"github.com/ricardo/anthrogo/pkg/telemetry"
)

// Telemetry implements the /telemetry builtin command.
type Telemetry struct {
	Reporter *telemetry.Reporter
}

func (Telemetry) Name() string      { return "/telemetry" }
func (Telemetry) Aliases() []string { return nil }
func (Telemetry) Description() string {
	return "Show or toggle telemetry status (status — requires restart to actually change)"
}
func (Telemetry) Type() command.Type { return command.TypeLocal }

func (t Telemetry) Run(_ context.Context, args string, _ command.Host) (command.Result, error) {
	args = strings.TrimSpace(args)
	if args == "" || args == "status" {
		if t.Reporter == nil || !t.Reporter.IsEnabled() {
			return command.Result{
				Text: "telemetry: DISABLED (default). Enable in settings.yaml: telemetry.enabled=true + telemetry.endpoint=<url>",
			}, nil
		}
		return command.Result{
			Text: "telemetry: ENABLED (anonymous; flushes every 60s; sent to " + t.Reporter.Endpoint() + ")",
		}, nil
	}
	return command.Result{Text: "usage: /telemetry [status]"}, nil
}

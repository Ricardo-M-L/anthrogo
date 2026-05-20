package builtins

import (
	"context"
	"fmt"
	"strings"

	"github.com/ricardo/anthrogo/internal/version"
	"github.com/ricardo/anthrogo/pkg/command"
	"github.com/ricardo/anthrogo/pkg/selfupdate"
)

// Version implements the /version slash command. It prints the running binary's
// version and checks GitHub Releases for a newer tag. Pass "no-check" to skip
// the network round-trip.
type Version struct{}

func (Version) Name() string        { return "/version" }
func (Version) Aliases() []string   { return nil }
func (Version) Description() string { return "Show anthrogo version and check for updates" }
func (Version) Type() command.Type  { return command.TypeLocal }

func (Version) Run(ctx context.Context, args string, _ command.Host) (command.Result, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "anthrogo %s\n\n", version.Version)

	if strings.TrimSpace(args) == "no-check" {
		return command.Result{Text: b.String()}, nil
	}

	rel, err := selfupdate.LatestRelease(ctx, "")
	if err != nil {
		b.WriteString("update check: " + err.Error() + "\n")
		return command.Result{Text: b.String()}, nil
	}

	if selfupdate.IsNewer(rel.TagName, version.Version) {
		fmt.Fprintf(&b, "→ Newer release available: %s\n  %s\n", rel.TagName, rel.HTMLURL)
	} else {
		fmt.Fprintf(&b, "Up to date (latest release: %s)\n", rel.TagName)
	}
	return command.Result{Text: b.String()}, nil
}

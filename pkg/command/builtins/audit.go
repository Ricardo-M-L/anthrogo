package builtins

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ricardo/anthrogo/internal/session"
	"github.com/ricardo/anthrogo/pkg/command"
)

// Audit implements the /audit builtin command.
type Audit struct{}

func (Audit) Name() string      { return "/audit" }
func (Audit) Aliases() []string { return nil }
func (Audit) Description() string {
	return "Audit tool calls + errors across sessions (subcommands: list [N], search <kw>, by-tool <name>, errors)"
}
func (Audit) Type() command.Type { return command.TypeLocal }

func (Audit) Run(ctx context.Context, args string, host command.Host) (command.Result, error) {
	dir, err := session.ProjectDir(host.Cwd())
	if err != nil {
		return command.Result{Text: "audit: " + err.Error()}, nil
	}
	args = strings.TrimSpace(args)
	switch {
	case args == "" || args == "list" || strings.HasPrefix(args, "list "):
		limit := 50
		if strings.HasPrefix(args, "list ") {
			if n, ferr := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(args, "list "))); ferr == nil && n > 0 {
				limit = n
			}
		}
		return listAudit(dir, limit)
	case strings.HasPrefix(args, "by-tool "):
		name := strings.TrimSpace(strings.TrimPrefix(args, "by-tool "))
		return byToolAudit(dir, name)
	case args == "errors":
		return errorsAudit(dir)
	case strings.HasPrefix(args, "search "):
		kw := strings.TrimSpace(strings.TrimPrefix(args, "search "))
		return searchAudit(dir, kw)
	default:
		return command.Result{Text: "usage: /audit [list [N] | by-tool <name> | errors | search <kw>]"}, nil
	}
}

type auditEvent struct {
	Timestamp time.Time
	Session   string
	Kind      string // "tool" | "error" | "compact" | "subagent"
	ToolName  string
	Summary   string
}

func collectAuditEvents(dir string) ([]auditEvent, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var events []auditEvent
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		sessionID := strings.TrimSuffix(e.Name(), ".jsonl")
		recs, err := session.Replay(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		for _, r := range recs {
			switch r.Kind {
			case session.KindToolUseRequest:
				if r.ToolUseRequest == nil {
					continue
				}
				events = append(events, auditEvent{
					Timestamp: r.Timestamp,
					Session:   sessionID,
					Kind:      "tool",
					ToolName:  r.ToolUseRequest.ToolName,
					Summary:   auditToolInputSummary(r.ToolUseRequest.ToolInput),
				})
			case session.KindToolResult:
				if r.ToolResult == nil || !r.ToolResult.IsError {
					continue
				}
				events = append(events, auditEvent{
					Timestamp: r.Timestamp,
					Session:   sessionID,
					Kind:      "error",
					Summary:   auditTruncStr(r.ToolResult.Text, 100),
				})
			case session.KindCompact:
				if r.Compact == nil {
					continue
				}
				events = append(events, auditEvent{
					Timestamp: r.Timestamp,
					Session:   sessionID,
					Kind:      "compact",
					Summary:   fmt.Sprintf("%d → %d msgs (%s)", r.Compact.OriginalCount, r.Compact.NewCount, r.Compact.Trigger),
				})
			case session.KindSubagentStart:
				if r.Subagent == nil {
					continue
				}
				events = append(events, auditEvent{
					Timestamp: r.Timestamp,
					Session:   sessionID,
					Kind:      "subagent",
					ToolName:  r.Subagent.Type,
					Summary:   r.Subagent.Description,
				})
			}
		}
	}
	sort.Slice(events, func(i, j int) bool { return events[i].Timestamp.After(events[j].Timestamp) })
	return events, nil
}

func auditToolInputSummary(input map[string]any) string {
	if len(input) == 0 {
		return ""
	}
	var parts []string
	for k, v := range input {
		parts = append(parts, fmt.Sprintf("%s=%s", k, auditTruncStr(fmt.Sprintf("%v", v), 30)))
	}
	sort.Strings(parts)
	return strings.Join(parts, " ")
}

func auditTruncStr(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func listAudit(dir string, limit int) (command.Result, error) {
	events, err := collectAuditEvents(dir)
	if err != nil {
		return command.Result{Text: "audit: " + err.Error()}, nil
	}
	if len(events) == 0 {
		return command.Result{Text: "(no audit events yet)"}, nil
	}
	if len(events) > limit {
		events = events[:limit]
	}
	return renderAudit(events), nil
}

func byToolAudit(dir, name string) (command.Result, error) {
	events, err := collectAuditEvents(dir)
	if err != nil {
		return command.Result{Text: "audit: " + err.Error()}, nil
	}
	var filtered []auditEvent
	for _, e := range events {
		if e.ToolName == name {
			filtered = append(filtered, e)
		}
	}
	return renderAudit(filtered), nil
}

func errorsAudit(dir string) (command.Result, error) {
	events, err := collectAuditEvents(dir)
	if err != nil {
		return command.Result{Text: "audit: " + err.Error()}, nil
	}
	var filtered []auditEvent
	for _, e := range events {
		if e.Kind == "error" {
			filtered = append(filtered, e)
		}
	}
	return renderAudit(filtered), nil
}

func searchAudit(dir, kw string) (command.Result, error) {
	events, err := collectAuditEvents(dir)
	if err != nil {
		return command.Result{Text: "audit: " + err.Error()}, nil
	}
	var filtered []auditEvent
	lkw := strings.ToLower(kw)
	for _, e := range events {
		hay := strings.ToLower(e.ToolName + " " + e.Summary)
		if strings.Contains(hay, lkw) {
			filtered = append(filtered, e)
		}
	}
	return renderAudit(filtered), nil
}

func renderAudit(events []auditEvent) command.Result {
	if len(events) == 0 {
		return command.Result{Text: "(no matches)"}
	}
	var b strings.Builder
	for _, e := range events {
		fmt.Fprintf(&b, "%s  [%s]  %-18s  %s\n",
			e.Timestamp.Format("2006-01-02 15:04"),
			auditShortID(e.Session),
			auditLabel(e.Kind, e.ToolName),
			e.Summary,
		)
	}
	return command.Result{Text: b.String()}
}

func auditShortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func auditLabel(kind, toolName string) string {
	if toolName != "" {
		return kind + ":" + toolName
	}
	return kind
}

package builtins

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ricardo/anthrogo/internal/session"
	"github.com/ricardo/anthrogo/pkg/command"
	"github.com/ricardo/anthrogo/pkg/message"
)

// Sessions implements the /sessions builtin command.
type Sessions struct{}

func (Sessions) Name() string        { return "/sessions" }
func (Sessions) Aliases() []string   { return nil }
func (Sessions) Description() string { return "List session JSONLs for the current cwd (subcommands: show <id-prefix>, replay <id-prefix>, search <keyword>)" }
func (Sessions) Type() command.Type  { return command.TypeLocal }

func (Sessions) Run(ctx context.Context, args string, host command.Host) (command.Result, error) {
	cwd := host.Cwd()
	args = strings.TrimSpace(args)
	dir, err := session.ProjectDir(cwd)
	if err != nil {
		return command.Result{Text: "sessions: " + err.Error()}, nil
	}
	switch {
	case args == "" || args == "list":
		return listSessions(dir)
	case strings.HasPrefix(args, "show "):
		prefix := strings.TrimSpace(strings.TrimPrefix(args, "show "))
		return showSession(dir, prefix)
	case strings.HasPrefix(args, "replay "):
		prefix := strings.TrimSpace(strings.TrimPrefix(args, "replay "))
		return replaySession(dir, prefix)
	case strings.HasPrefix(args, "search "):
		keyword := strings.TrimSpace(strings.TrimPrefix(args, "search "))
		return searchSessions(dir, keyword)
	default:
		return command.Result{Text: "usage: /sessions [list | show <id-prefix> | replay <id-prefix> | search <keyword>]"}, nil
	}
}

func listSessions(dir string) (command.Result, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return command.Result{Text: "(no sessions yet)"}, nil
		}
		return command.Result{Text: "sessions: " + err.Error()}, nil
	}
	type row struct {
		ID, Modified, Size string
	}
	var rows []row
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".jsonl")
		rows = append(rows, row{
			ID:       id,
			Modified: info.ModTime().Format("2006-01-02 15:04"),
			Size:     fmt.Sprintf("%d B", info.Size()),
		})
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Modified > rows[j].Modified })
	if len(rows) == 0 {
		return command.Result{Text: "(no sessions yet)"}, nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%-38s  %-16s  %s\n", "ID", "Modified", "Size")
	for _, r := range rows {
		fmt.Fprintf(&b, "%-38s  %-16s  %s\n", r.ID, r.Modified, r.Size)
	}
	return command.Result{Text: b.String()}, nil
}

func showSession(dir, prefix string) (command.Result, error) {
	if prefix == "" {
		return command.Result{Text: "usage: /sessions show <id-prefix>"}, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return command.Result{Text: "sessions: " + err.Error()}, nil
	}
	var matched []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), prefix) && strings.HasSuffix(e.Name(), ".jsonl") {
			matched = append(matched, e.Name())
		}
	}
	if len(matched) == 0 {
		return command.Result{Text: "sessions: no match for " + prefix}, nil
	}
	if len(matched) > 1 {
		return command.Result{Text: "sessions: ambiguous prefix " + prefix + " (matches: " + strings.Join(matched, ", ") + ")"}, nil
	}
	info, err := os.Stat(filepath.Join(dir, matched[0]))
	if err != nil {
		return command.Result{Text: "sessions: " + err.Error()}, nil
	}
	return command.Result{Text: fmt.Sprintf("session: %s\npath: %s\nmodified: %s\nsize: %d bytes\n",
		matched[0],
		filepath.Join(dir, matched[0]),
		info.ModTime().Format("2006-01-02 15:04:05"),
		info.Size(),
	)}, nil
}

// matchPrefix is shared by showSession and replaySession to find an unambiguous file.
func matchPrefix(dir, prefix string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var matched []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), prefix) && strings.HasSuffix(e.Name(), ".jsonl") {
			matched = append(matched, e.Name())
		}
	}
	return matched, nil
}

// replaySession renders every record in the matched session JSONL as a timeline.
func replaySession(dir, prefix string) (command.Result, error) {
	if prefix == "" {
		return command.Result{Text: "usage: /sessions replay <id-prefix>"}, nil
	}
	matched, err := matchPrefix(dir, prefix)
	if err != nil {
		return command.Result{Text: "sessions: " + err.Error()}, nil
	}
	if len(matched) == 0 {
		return command.Result{Text: "sessions: no match for " + prefix}, nil
	}
	if len(matched) > 1 {
		return command.Result{Text: "sessions: ambiguous prefix " + prefix + " (matches: " + strings.Join(matched, ", ") + ")"}, nil
	}
	records, err := session.Replay(filepath.Join(dir, matched[0]))
	if err != nil {
		return command.Result{Text: "sessions: replay error: " + err.Error()}, nil
	}
	var lines []string
	for _, r := range records {
		line := renderRecord(r)
		lines = append(lines, line)
	}
	return command.Result{Text: strings.Join(lines, "\n")}, nil
}

// renderRecord converts one session.Record to a single display line.
func renderRecord(r session.Record) string {
	switch r.Kind {
	case session.KindSessionMeta:
		m := r.SessionMeta
		if m == nil {
			return "[meta] (empty)"
		}
		created := m.CreatedAt.Format("2006-01-02 15:04:05")
		return truncate(fmt.Sprintf("[meta] session=%s model=%s created=%s", m.SessionID, m.Model, created), 250)
	case session.KindUserMessage:
		if r.UserMessage == nil {
			return "[user] (empty)"
		}
		return truncate("[user] "+textFromBlocks(r.UserMessage.Content), 250)
	case session.KindAssistantMessage:
		if r.AssistantMessage == nil {
			return "[asst] (empty)"
		}
		return truncate("[asst] "+textFromBlocks(r.AssistantMessage.Content), 250)
	case session.KindToolUseRequest:
		if r.ToolUseRequest == nil {
			return "[tool] (empty)"
		}
		inputJSON, _ := json.Marshal(r.ToolUseRequest.ToolInput)
		inputStr := truncate(string(inputJSON), 80)
		return truncate(fmt.Sprintf("[tool] %s(%s)", r.ToolUseRequest.ToolName, inputStr), 250)
	case session.KindToolResult:
		if r.ToolResult == nil {
			return "[result] (empty)"
		}
		status := "ok"
		if r.ToolResult.IsError {
			status = "ERR"
		}
		return truncate(fmt.Sprintf("[result] %s %s", status, r.ToolResult.Text), 250)
	case session.KindCompact:
		if r.Compact == nil {
			return "[compact] (empty)"
		}
		return truncate(fmt.Sprintf("[compact] %d→%d messages, %s", r.Compact.OriginalCount, r.Compact.NewCount, r.Compact.Trigger), 250)
	case session.KindSubagentStart:
		if r.Subagent == nil {
			return "[subagent] (empty)"
		}
		shortID := r.Subagent.ID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}
		return truncate(fmt.Sprintf("[subagent] type=%s id=%s desc=%s", r.Subagent.Type, shortID, r.Subagent.Description), 250)
	case session.KindUsage:
		if r.Usage == nil {
			return "[usage] (empty)"
		}
		return fmt.Sprintf("[usage] in=%d out=%d", r.Usage.InputTokens, r.Usage.OutputTokens)
	case session.KindTurnComplete:
		if r.TurnComplete == nil {
			return "[turn-end] (empty)"
		}
		return "[turn-end] " + r.TurnComplete.StopReason
	case session.KindError:
		if r.Error == nil {
			return "[error] (empty)"
		}
		return truncate(fmt.Sprintf("[error] during=%s: %s", r.Error.During, r.Error.Error), 250)
	default:
		return fmt.Sprintf("[%s]", r.Kind)
	}
}

// textFromBlocks extracts text from a slice of content blocks, skipping thinking blocks.
func textFromBlocks(blocks []message.Block) string {
	var parts []string
	for _, b := range blocks {
		switch b.Type {
		case message.BlockText:
			parts = append(parts, b.Text)
		case message.BlockThinking:
			// skip
		}
	}
	return strings.Join(parts, " ")
}

// truncate collapses newlines and limits s to at most n runes, appending "…" if truncated.
func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "…"
}

const searchMaxMatches = 200

// searchSessions performs case-insensitive cross-session text search.
func searchSessions(dir, keyword string) (command.Result, error) {
	if keyword == "" {
		return command.Result{Text: "usage: /sessions search <keyword>"}, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return command.Result{Text: "(no sessions yet)"}, nil
		}
		return command.Result{Text: "sessions: " + err.Error()}, nil
	}

	kwLower := strings.ToLower(keyword)
	var lines []string
	total := 0
	capped := false

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		sessionID := strings.TrimSuffix(e.Name(), ".jsonl")
		shortID := sessionID
		if len(shortID) > 8 {
			shortID = shortID[:8]
		}

		records, err := session.Replay(filepath.Join(dir, e.Name()))
		if err != nil {
			lines = append(lines, fmt.Sprintf("[skip %s: %s]", shortID, err.Error()))
			continue
		}

		for _, r := range records {
			haystack, kindLabel := searchableText(r)
			if haystack == "" {
				continue
			}
			haystackLower := strings.ToLower(haystack)
			idx := strings.Index(haystackLower, kwLower)
			if idx < 0 {
				continue
			}
			// Truncate around match: 40 chars before + match + 40 chars after.
			context := contextAround(haystack, idx, len(keyword), 40)
			total++
			if total > searchMaxMatches {
				capped = true
				break
			}
			lines = append(lines, fmt.Sprintf("%s [%s] %s", shortID, kindLabel, context))
		}
		if capped {
			break
		}
	}

	if len(lines) == 0 && !capped {
		return command.Result{Text: "(no matches)"}, nil
	}

	if capped {
		lines = append(lines, fmt.Sprintf("... and %d more", total-searchMaxMatches))
	}

	// Sort results for determinism (by line content).
	sort.Strings(lines)
	return command.Result{Text: strings.Join(lines, "\n")}, nil
}

// searchableText returns the text to search within a record and a kind label.
func searchableText(r session.Record) (string, string) {
	switch r.Kind {
	case session.KindUserMessage:
		if r.UserMessage == nil {
			return "", ""
		}
		return textFromBlocks(r.UserMessage.Content), "user"
	case session.KindAssistantMessage:
		if r.AssistantMessage == nil {
			return "", ""
		}
		return textFromBlocks(r.AssistantMessage.Content), "asst"
	case session.KindToolResult:
		if r.ToolResult == nil {
			return "", ""
		}
		return r.ToolResult.Text, "result"
	case session.KindSessionMeta:
		if r.SessionMeta == nil {
			return "", ""
		}
		return r.SessionMeta.SessionID + " " + r.SessionMeta.Model, "meta"
	default:
		return "", ""
	}
}

// contextAround returns up to before chars before the match and after chars after,
// collapsed to a single line, max ~250 chars total.
func contextAround(s string, matchIdx, matchLen, window int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	runes := []rune(s)
	start := matchIdx - window
	if start < 0 {
		start = 0
	}
	end := matchIdx + matchLen + window
	if end > len(runes) {
		end = len(runes)
	}
	snippet := string(runes[start:end])
	return truncate(snippet, 120)
}

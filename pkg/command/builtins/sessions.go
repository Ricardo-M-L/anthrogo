package builtins

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/ricardo/anthrogo/internal/session"
	"github.com/ricardo/anthrogo/pkg/command"
	"github.com/ricardo/anthrogo/pkg/message"
)

// Sessions implements the /sessions builtin command.
type Sessions struct{}

func (Sessions) Name() string        { return "/sessions" }
func (Sessions) Aliases() []string   { return nil }
func (Sessions) Description() string {
	return "List session JSONLs for the current cwd (subcommands: show <id-prefix>, replay <id-prefix>, search <keyword>, export <id-prefix> [-o file.md])"
}
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
	case strings.HasPrefix(args, "delete "):
		rest := strings.TrimSpace(strings.TrimPrefix(args, "delete "))
		return deleteSession(dir, rest)
	case strings.HasPrefix(args, "export "):
		return exportSession(dir, strings.TrimSpace(strings.TrimPrefix(args, "export ")))
	default:
		return command.Result{Text: "usage: /sessions [list | show <id-prefix> | replay <id-prefix> | search <keyword> | delete <id-prefix> [--yes] | export <id-prefix> [-o file.md]]"}, nil
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

// deleteSession deletes a session JSONL (and its subagents dir) identified by
// an unambiguous prefix. rest may contain --yes anywhere; without it the
// function performs a dry-run only.
func deleteSession(dir, rest string) (command.Result, error) {
	// Parse --yes flag out of rest.
	tokens := strings.Fields(rest)
	confirm := false
	var prefixTokens []string
	for _, tok := range tokens {
		if tok == "--yes" {
			confirm = true
		} else {
			prefixTokens = append(prefixTokens, tok)
		}
	}
	if len(prefixTokens) != 1 {
		return command.Result{Text: "usage: /sessions delete [--yes] <id-prefix>"}, nil
	}
	prefix := prefixTokens[0]

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

	jsonlPath := filepath.Join(dir, matched[0])
	sessionID := strings.TrimSuffix(matched[0], ".jsonl")
	subagentsDir := filepath.Join(dir, sessionID, "subagents")

	// Stat the JSONL for size.
	jsonlInfo, err := os.Stat(jsonlPath)
	if err != nil {
		return command.Result{Text: "sessions: " + err.Error()}, nil
	}

	// Check whether subagents dir exists.
	subInfo, statErr := os.Stat(subagentsDir)
	hasSubagents := statErr == nil && subInfo.IsDir()

	if !confirm {
		var b strings.Builder
		fmt.Fprintf(&b, "would delete:\n")
		fmt.Fprintf(&b, "  %s\t(%d bytes)\n", jsonlPath, jsonlInfo.Size())
		if hasSubagents {
			count, total := dirStats(subagentsDir)
			fmt.Fprintf(&b, "  %s/\t(%d files, total %d bytes)\n", subagentsDir, count, total)
		}
		fmt.Fprintf(&b, "run with --yes to actually delete:\n")
		fmt.Fprintf(&b, "  /sessions delete --yes %s", prefix)
		return command.Result{Text: b.String()}, nil
	}

	// Perform actual deletion.
	if err := os.Remove(jsonlPath); err != nil {
		return command.Result{Text: "sessions: " + err.Error()}, nil
	}
	msg := "deleted " + jsonlPath
	if hasSubagents {
		if err := os.RemoveAll(subagentsDir); err != nil {
			return command.Result{Text: "sessions: deleted " + jsonlPath + " but failed to remove subagents/: " + err.Error()}, nil
		}
		msg += " and subagents/"
	}
	return command.Result{Text: msg}, nil
}

// dirStats returns (file count, total bytes) for all files under dir (recursive).
// Returns (0, 0) if dir doesn't exist or is unreadable.
func dirStats(dir string) (count int, bytes int64) {
	_ = filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		count++
		bytes += info.Size()
		return nil
	})
	return
}

// exportSession renders a matched session JSONL as a markdown document.
// rest is parsed for an optional -o / --out <file> flag; remaining token is the id-prefix.
func exportSession(dir, rest string) (command.Result, error) {
	if rest == "" {
		return command.Result{Text: "usage: /sessions export <id-prefix> [-o file.md]"}, nil
	}

	// Parse tokens for -o / --out flag.
	tokens := strings.Fields(rest)
	var outFile string
	var prefixTokens []string
	for i := 0; i < len(tokens); i++ {
		if (tokens[i] == "-o" || tokens[i] == "--out") && i+1 < len(tokens) {
			outFile = tokens[i+1]
			i++ // skip next token
		} else {
			prefixTokens = append(prefixTokens, tokens[i])
		}
	}
	if len(prefixTokens) != 1 {
		return command.Result{Text: "usage: /sessions export <id-prefix> [-o file.md]"}, nil
	}
	prefix := prefixTokens[0]

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

	md := renderSessionMarkdown(records)

	if outFile != "" {
		if err := os.WriteFile(outFile, []byte(md), 0o644); err != nil {
			return command.Result{Text: "sessions: write error: " + err.Error()}, nil
		}
		return command.Result{Text: fmt.Sprintf("exported %s (%d bytes)", outFile, len(md))}, nil
	}
	return command.Result{Text: md}, nil
}

// renderSessionMarkdown converts a slice of session records to a markdown document.
func renderSessionMarkdown(records []session.Record) string {
	var b strings.Builder

	// Find the session meta record for the header.
	var meta *session.SessionMeta
	for _, r := range records {
		if r.Kind == session.KindSessionMeta && r.SessionMeta != nil {
			meta = r.SessionMeta
			break
		}
	}

	// Write document header.
	sessionID := ""
	if meta != nil {
		sessionID = meta.SessionID
	}
	fmt.Fprintf(&b, "# anthrogo session: %s\n\n", sessionID)
	if meta != nil {
		fmt.Fprintf(&b, "**Created:** %s\n", meta.CreatedAt.Format("2006-01-02 15:04:05"))
		fmt.Fprintf(&b, "**Model:** %s\n", meta.Model)
		fmt.Fprintf(&b, "**Permission mode:** %s\n", meta.PermissionMode)
		fmt.Fprintf(&b, "**Cwd:** %s\n", meta.Cwd)
	}
	fmt.Fprintf(&b, "\n---\n\n")

	// Render each record.
	for _, r := range records {
		switch r.Kind {
		case session.KindUserMessage:
			if r.UserMessage == nil {
				continue
			}
			text := renderBlockText(r.UserMessage.Content)
			fmt.Fprintf(&b, "### 👤 User\n\n%s\n\n", text)

		case session.KindAssistantMessage:
			if r.AssistantMessage == nil {
				continue
			}
			text := renderBlockText(r.AssistantMessage.Content)
			fmt.Fprintf(&b, "### 🤖 Assistant\n\n%s\n\n", text)

		case session.KindToolUseRequest:
			if r.ToolUseRequest == nil {
				continue
			}
			inputJSON, _ := json.MarshalIndent(r.ToolUseRequest.ToolInput, "", "  ")
			fmt.Fprintf(&b, "#### 🔧 Tool: %s\n\n```json\n%s\n```\n\n", r.ToolUseRequest.ToolName, string(inputJSON))

		case session.KindToolResult:
			if r.ToolResult == nil {
				continue
			}
			errSuffix := ""
			if r.ToolResult.IsError {
				errSuffix = " (error)"
			}
			fmt.Fprintf(&b, "##### ↩ Result%s\n\n", errSuffix)
			text := r.ToolResult.Text
			if looksLikeCode(text) {
				fmt.Fprintf(&b, "```\n%s\n```\n\n", text)
			} else {
				fmt.Fprintf(&b, "%s\n\n", text)
			}

		case session.KindCompact:
			if r.Compact == nil {
				continue
			}
			fmt.Fprintf(&b, "> **Compacted:** %d → %d messages (%s)\n\n",
				r.Compact.OriginalCount, r.Compact.NewCount, r.Compact.Trigger)

		case session.KindSubagentStart:
			if r.Subagent == nil {
				continue
			}
			fmt.Fprintf(&b, "> **Subagent started:** type=%s desc=%s\n\n",
				r.Subagent.Type, r.Subagent.Description)

		case session.KindError:
			if r.Error == nil {
				continue
			}
			fmt.Fprintf(&b, "> ❗ **Error** during %s: %s\n\n", r.Error.During, r.Error.Error)

		case session.KindUsage, session.KindTurnComplete, session.KindSessionMeta:
			// skip — meta already rendered at top
		}
	}

	return b.String()
}

// renderBlockText extracts and joins text from a slice of content blocks.
// Image blocks are rendered as a placeholder; thinking blocks are skipped.
func renderBlockText(blocks []message.Block) string {
	var parts []string
	for _, b := range blocks {
		switch b.Type {
		case message.BlockText:
			parts = append(parts, b.Text)
		case message.BlockImage:
			if b.ImageSource != nil {
				parts = append(parts, "_[image: "+b.ImageSource.MediaType+", "+strconv.Itoa(len(b.ImageSource.Data))+" base64 bytes]_")
			} else {
				parts = append(parts, "_[image]_")
			}
		case message.BlockThinking:
			// skip
		}
	}
	return strings.Join(parts, "\n\n")
}

// looksLikeCode is a soft heuristic that returns true when the text is
// likely structured/code content that benefits from a fenced code block.
func looksLikeCode(s string) bool {
	if !strings.Contains(s, "\n") {
		return false
	}
	head := strings.TrimSpace(strings.SplitN(s, "\n", 2)[0])
	if strings.HasPrefix(head, "{") || strings.HasPrefix(head, "[") || strings.HasPrefix(head, "<") {
		return true
	}
	keywords := []string{"func ", "def ", "package ", "import ", "class ", "fn "}
	for _, k := range keywords {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
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

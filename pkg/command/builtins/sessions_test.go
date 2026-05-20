package builtins

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/internal/session"
	"github.com/ricardo/anthrogo/pkg/message"
)

// sessionsFakeHost is a minimal host used only by the sessions tests.
// It overrides Cwd() to return a temp dir that acts as a fake "project dir"
// (i.e. the dir that session.ProjectDir would resolve to). Because session.ProjectDir
// hashes the cwd and places files under ~/.anthrogo/projects/<hash>/, we can't
// directly control its output path from outside. Instead, we unit-test listSessions
// and showSession directly with a synthetic directory.

func TestSessions_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	res, err := listSessions(dir)
	require.NoError(t, err)
	require.Equal(t, "(no sessions yet)", res.Text)
}

func TestSessions_ListsJSONLs(t *testing.T) {
	dir := t.TempDir()
	// Create two .jsonl files.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "aaa-111.jsonl"), []byte(`{}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bbb-222.jsonl"), []byte(`{}{}`), 0o644))
	// A non-.jsonl file — should be ignored.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notajsonl.txt"), []byte(`x`), 0o644))

	res, err := listSessions(dir)
	require.NoError(t, err)
	require.Contains(t, res.Text, "aaa-111")
	require.Contains(t, res.Text, "bbb-222")
	require.NotContains(t, res.Text, "notajsonl")
	// Header present.
	require.Contains(t, res.Text, "ID")
	require.Contains(t, res.Text, "Modified")
}

func TestSessions_ShowKnown(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "abc-123-def.jsonl"), []byte(`{}`), 0o644))

	res, err := showSession(dir, "abc-123")
	require.NoError(t, err)
	require.Contains(t, res.Text, "abc-123-def.jsonl")
	require.Contains(t, res.Text, "path:")
	require.Contains(t, res.Text, "modified:")
	require.Contains(t, res.Text, "size:")
}

func TestSessions_ShowUnknown(t *testing.T) {
	dir := t.TempDir()
	res, err := showSession(dir, "no-such-prefix")
	require.NoError(t, err)
	require.Contains(t, res.Text, "no match")
}

func TestSessions_ShowAmbiguous(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "abc-aaa.jsonl"), []byte(`{}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "abc-bbb.jsonl"), []byte(`{}`), 0o644))

	res, err := showSession(dir, "abc-")
	require.NoError(t, err)
	require.Contains(t, res.Text, "ambiguous")
}

func TestSessions_UsageMessage(t *testing.T) {
	h := newFakeHost()
	h.cwd = t.TempDir() // non-hash path — session.ProjectDir will create a subdir
	res, err := (Sessions{}).Run(context.Background(), "badcmd", h)
	require.NoError(t, err)
	require.Contains(t, res.Text, "usage")
}

// writeJSONL creates a JSONL file in dir named <name>.jsonl, populated with records.
func writeJSONL(t *testing.T, dir, name string, records []session.Record) {
	t.Helper()
	var buf bytes.Buffer
	for _, r := range records {
		line, err := r.MarshalJSONLine()
		require.NoError(t, err)
		buf.Write(line)
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, name+".jsonl"), buf.Bytes(), 0o644))
}

// makeSessionRecords returns a minimal set of session records for replay tests.
func makeSessionRecords() []session.Record {
	ts := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	return []session.Record{
		{
			Kind:      session.KindSessionMeta,
			Timestamp: ts,
			SessionMeta: &session.SessionMeta{
				SessionID: "test-session-id",
				Model:     "claude-test",
				CreatedAt: ts,
			},
		},
		{
			Kind:      session.KindUserMessage,
			Timestamp: ts,
			UserMessage: &session.UserMessage{
				Content: []message.Block{{Type: message.BlockText, Text: "hello world"}},
			},
		},
		{
			Kind:      session.KindAssistantMessage,
			Timestamp: ts,
			AssistantMessage: &session.AssistantMessage{
				Content:    []message.Block{{Type: message.BlockText, Text: "goodbye world"}},
				StopReason: "end_turn",
			},
		},
		{
			Kind:      session.KindToolUseRequest,
			Timestamp: ts,
			ToolUseRequest: &session.ToolUseRequest{
				ToolUseID: "tu1",
				ToolName:  "Bash",
				ToolInput: map[string]any{"command": "ls"},
			},
		},
		{
			Kind:      session.KindToolResult,
			Timestamp: ts,
			ToolResult: &session.ToolResult{
				ToolUseID: "tu1",
				Text:      "file1.go file2.go",
				IsError:   false,
			},
		},
		{
			Kind:      session.KindTurnComplete,
			Timestamp: ts,
			TurnComplete: &session.TurnComplete{StopReason: "end_turn"},
		},
		{
			Kind:      session.KindUsage,
			Timestamp: ts,
			Usage:     &session.UsageRecord{InputTokens: 100, OutputTokens: 50},
		},
	}
}

// TestSessions_ReplayUnknownPrefix — prefix has no match.
func TestSessions_ReplayUnknownPrefix(t *testing.T) {
	dir := t.TempDir()
	res, err := replaySession(dir, "no-such-prefix")
	require.NoError(t, err)
	require.Contains(t, res.Text, "no match")
}

// TestSessions_ReplayAmbiguousPrefix — multiple files share prefix.
func TestSessions_ReplayAmbiguousPrefix(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "abc-aaa.jsonl"), []byte{}, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "abc-bbb.jsonl"), []byte{}, 0o644))
	res, err := replaySession(dir, "abc-")
	require.NoError(t, err)
	require.Contains(t, res.Text, "ambiguous")
}

// TestSessions_ReplayRenders — all rendered lines appear for a sample JSONL.
func TestSessions_ReplayRenders(t *testing.T) {
	dir := t.TempDir()
	records := makeSessionRecords()
	writeJSONL(t, dir, "test-session-id", records)

	res, err := replaySession(dir, "test-session-id")
	require.NoError(t, err)
	require.Contains(t, res.Text, "[meta]")
	require.Contains(t, res.Text, "claude-test")
	require.Contains(t, res.Text, "[user]")
	require.Contains(t, res.Text, "hello world")
	require.Contains(t, res.Text, "[asst]")
	require.Contains(t, res.Text, "goodbye world")
	require.Contains(t, res.Text, "[tool]")
	require.Contains(t, res.Text, "Bash")
	require.Contains(t, res.Text, "[result]")
	require.Contains(t, res.Text, "file1.go")
	require.Contains(t, res.Text, "[turn-end]")
	require.Contains(t, res.Text, "end_turn")
	require.Contains(t, res.Text, "[usage]")
	require.Contains(t, res.Text, "in=100")
}

// TestSessions_SearchEmpty — keyword "" returns usage message.
func TestSessions_SearchEmpty(t *testing.T) {
	dir := t.TempDir()
	res, err := searchSessions(dir, "")
	require.NoError(t, err)
	require.Contains(t, res.Text, "usage")
}

// TestSessions_SearchFindsInUser — keyword matches in only one session's user message.
func TestSessions_SearchFindsInUser(t *testing.T) {
	dir := t.TempDir()

	ts := time.Now()
	matchRecords := []session.Record{{
		Kind:      session.KindUserMessage,
		Timestamp: ts,
		UserMessage: &session.UserMessage{
			Content: []message.Block{{Type: message.BlockText, Text: "unique-keyword-xyz found here"}},
		},
	}}
	noMatchRecords := []session.Record{{
		Kind:      session.KindUserMessage,
		Timestamp: ts,
		UserMessage: &session.UserMessage{
			Content: []message.Block{{Type: message.BlockText, Text: "nothing interesting"}},
		},
	}}

	writeJSONL(t, dir, "session-match", matchRecords)
	writeJSONL(t, dir, "session-nomatch", noMatchRecords)

	res, err := searchSessions(dir, "unique-keyword-xyz")
	require.NoError(t, err)
	require.Contains(t, res.Text, "unique-keyword-xyz")
	require.NotContains(t, res.Text, "nothing interesting")
	// Only one match line (plus the "session-match" short ID).
	lines := splitNonEmpty(res.Text)
	require.Len(t, lines, 1)
}

// TestSessions_SearchFindsInAssistantAndToolResult — keyword appears in both kinds.
func TestSessions_SearchFindsInAssistantAndToolResult(t *testing.T) {
	dir := t.TempDir()
	ts := time.Now()
	records := []session.Record{
		{
			Kind:      session.KindAssistantMessage,
			Timestamp: ts,
			AssistantMessage: &session.AssistantMessage{
				Content:    []message.Block{{Type: message.BlockText, Text: "the word foobar appears here"}},
				StopReason: "end_turn",
			},
		},
		{
			Kind:      session.KindToolResult,
			Timestamp: ts,
			ToolResult: &session.ToolResult{
				ToolUseID: "tu1",
				Text:      "tool output foobar result",
			},
		},
	}
	writeJSONL(t, dir, "session-both", records)

	res, err := searchSessions(dir, "foobar")
	require.NoError(t, err)
	lines := splitNonEmpty(res.Text)
	// Two records matched.
	require.Len(t, lines, 2)
	require.Contains(t, res.Text, "[asst]")
	require.Contains(t, res.Text, "[result]")
}

// TestSessions_SearchNoMatches — keyword found nowhere.
func TestSessions_SearchNoMatches(t *testing.T) {
	dir := t.TempDir()
	ts := time.Now()
	records := []session.Record{{
		Kind:      session.KindUserMessage,
		Timestamp: ts,
		UserMessage: &session.UserMessage{
			Content: []message.Block{{Type: message.BlockText, Text: "nothing here"}},
		},
	}}
	writeJSONL(t, dir, "session-empty", records)

	res, err := searchSessions(dir, "zzz-not-present-zzz")
	require.NoError(t, err)
	require.Contains(t, res.Text, "(no matches)")
}

// TestSessions_DeleteDryRun_DoesNotRemove verifies that delete without --yes
// prints "would delete" and does NOT remove the file.
func TestSessions_DeleteDryRun_DoesNotRemove(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "abc-111.jsonl"), []byte(`{}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "def-222.jsonl"), []byte(`{}`), 0o644))

	res, err := deleteSession(dir, "abc-111")
	require.NoError(t, err)
	require.Contains(t, res.Text, "would delete")
	require.Contains(t, res.Text, "abc-111.jsonl")
	// File must still exist.
	_, statErr := os.Stat(filepath.Join(dir, "abc-111.jsonl"))
	require.NoError(t, statErr)
}

// TestSessions_DeleteYes_RemovesFile verifies that --yes actually deletes the JSONL.
func TestSessions_DeleteYes_RemovesFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "abc-111.jsonl"), []byte(`{}`), 0o644))

	res, err := deleteSession(dir, "--yes abc-111")
	require.NoError(t, err)
	require.Contains(t, res.Text, "deleted")
	_, statErr := os.Stat(filepath.Join(dir, "abc-111.jsonl"))
	require.True(t, os.IsNotExist(statErr), "file should be gone")
}

// TestSessions_DeleteYes_RemovesSubagentsDir verifies that the subagents dir is
// also removed when it exists.
func TestSessions_DeleteYes_RemovesSubagentsDir(t *testing.T) {
	dir := t.TempDir()
	sessionID := "abc-subagent-session"
	require.NoError(t, os.WriteFile(filepath.Join(dir, sessionID+".jsonl"), []byte(`{}`), 0o644))
	// Create subagents dir with a file inside.
	subDir := filepath.Join(dir, sessionID, "subagents")
	require.NoError(t, os.MkdirAll(subDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "sub1.jsonl"), []byte(`{}`), 0o644))

	res, err := deleteSession(dir, "--yes "+sessionID)
	require.NoError(t, err)
	require.Contains(t, res.Text, "deleted")
	require.Contains(t, res.Text, "subagents/")
	_, statErr := os.Stat(filepath.Join(dir, sessionID+".jsonl"))
	require.True(t, os.IsNotExist(statErr), "jsonl should be gone")
	_, subStatErr := os.Stat(subDir)
	require.True(t, os.IsNotExist(subStatErr), "subagents dir should be gone")
}

// TestSessions_DeleteUnknownPrefix verifies a "no match" result for unknown prefix.
func TestSessions_DeleteUnknownPrefix(t *testing.T) {
	dir := t.TempDir()
	res, err := deleteSession(dir, "nope")
	require.NoError(t, err)
	require.Contains(t, res.Text, "no match")
}

// TestSessions_DeleteAmbiguousPrefix verifies an "ambiguous" result when multiple
// files share the prefix, and that neither is deleted.
func TestSessions_DeleteAmbiguousPrefix(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "abcXXX.jsonl"), []byte(`{}`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "abcYYY.jsonl"), []byte(`{}`), 0o644))

	res, err := deleteSession(dir, "--yes abc")
	require.NoError(t, err)
	require.Contains(t, res.Text, "ambiguous")
	// Both files must still exist.
	_, err1 := os.Stat(filepath.Join(dir, "abcXXX.jsonl"))
	require.NoError(t, err1)
	_, err2 := os.Stat(filepath.Join(dir, "abcYYY.jsonl"))
	require.NoError(t, err2)
}

// TestSessions_DeleteMissingPrefix verifies the usage message when no prefix is given.
func TestSessions_DeleteMissingPrefix(t *testing.T) {
	dir := t.TempDir()
	// "--yes" with no additional prefix token.
	res, err := deleteSession(dir, "--yes")
	require.NoError(t, err)
	require.Contains(t, res.Text, "usage")
}

// splitNonEmpty splits by newline, returning only non-empty lines.
func splitNonEmpty(s string) []string {
	var out []string
	for _, line := range bytes.Split([]byte(s), []byte("\n")) {
		if len(bytes.TrimSpace(line)) > 0 {
			out = append(out, string(line))
		}
	}
	return out
}

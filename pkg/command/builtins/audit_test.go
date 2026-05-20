package builtins

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/internal/session"
)

// auditTS is a fixed timestamp used in audit tests.
var auditTS = time.Date(2026, 3, 15, 10, 30, 0, 0, time.UTC)

func TestAudit_Empty(t *testing.T) {
	dir := t.TempDir()
	res, err := listAudit(dir, 50)
	require.NoError(t, err)
	require.Equal(t, "(no audit events yet)", res.Text)
}

func TestAudit_ListsToolCalls(t *testing.T) {
	dir := t.TempDir()
	records := []session.Record{
		{
			Kind:      session.KindToolUseRequest,
			Timestamp: auditTS,
			ToolUseRequest: &session.ToolUseRequest{
				ToolUseID: "tu-1",
				ToolName:  "Bash",
				ToolInput: map[string]any{"command": "ls -la"},
			},
		},
		{
			Kind:      session.KindToolUseRequest,
			Timestamp: auditTS.Add(time.Minute),
			ToolUseRequest: &session.ToolUseRequest{
				ToolUseID: "tu-2",
				ToolName:  "Read",
				ToolInput: map[string]any{"file_path": "/tmp/foo.go"},
			},
		},
	}
	writeJSONL(t, dir, "sess-abc123", records)

	res, err := listAudit(dir, 50)
	require.NoError(t, err)
	require.Contains(t, res.Text, "tool:Bash")
	require.Contains(t, res.Text, "tool:Read")
	require.Contains(t, res.Text, "sess-abc")
}

func TestAudit_FilterByTool(t *testing.T) {
	dir := t.TempDir()
	records := []session.Record{
		{
			Kind:      session.KindToolUseRequest,
			Timestamp: auditTS,
			ToolUseRequest: &session.ToolUseRequest{
				ToolUseID: "tu-1",
				ToolName:  "Bash",
				ToolInput: map[string]any{"command": "echo hi"},
			},
		},
		{
			Kind:      session.KindToolUseRequest,
			Timestamp: auditTS.Add(time.Minute),
			ToolUseRequest: &session.ToolUseRequest{
				ToolUseID: "tu-2",
				ToolName:  "Read",
				ToolInput: map[string]any{"file_path": "/tmp/bar.go"},
			},
		},
	}
	writeJSONL(t, dir, "sess-filter", records)

	res, err := byToolAudit(dir, "Bash")
	require.NoError(t, err)
	require.Contains(t, res.Text, "tool:Bash")
	require.NotContains(t, res.Text, "tool:Read")
}

func TestAudit_FilterErrors(t *testing.T) {
	dir := t.TempDir()
	records := []session.Record{
		{
			Kind:      session.KindToolResult,
			Timestamp: auditTS,
			ToolResult: &session.ToolResult{
				ToolUseID: "tu-1",
				Text:      "permission denied",
				IsError:   true,
			},
		},
		{
			Kind:      session.KindToolResult,
			Timestamp: auditTS.Add(time.Minute),
			ToolResult: &session.ToolResult{
				ToolUseID: "tu-2",
				Text:      "ok output",
				IsError:   false,
			},
		},
	}
	writeJSONL(t, dir, "sess-errors", records)

	res, err := errorsAudit(dir)
	require.NoError(t, err)
	require.Contains(t, res.Text, "permission denied")
	require.NotContains(t, res.Text, "ok output")
}

func TestAudit_Search(t *testing.T) {
	dir := t.TempDir()
	records := []session.Record{
		{
			Kind:      session.KindToolUseRequest,
			Timestamp: auditTS,
			ToolUseRequest: &session.ToolUseRequest{
				ToolUseID: "tu-1",
				ToolName:  "Bash",
				ToolInput: map[string]any{"command": "git status"},
			},
		},
		{
			Kind:      session.KindToolUseRequest,
			Timestamp: auditTS.Add(time.Minute),
			ToolUseRequest: &session.ToolUseRequest{
				ToolUseID: "tu-2",
				ToolName:  "Read",
				ToolInput: map[string]any{"file_path": "/tmp/main.go"},
			},
		},
	}
	writeJSONL(t, dir, "sess-search", records)

	res, err := searchAudit(dir, "git")
	require.NoError(t, err)
	require.Contains(t, res.Text, "git status")
	require.NotContains(t, res.Text, "main.go")
}

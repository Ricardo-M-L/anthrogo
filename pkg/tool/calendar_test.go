package tool

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCalendar_BasicICS(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "event.ics")

	res, err := CalendarEvent{}.Call(context.Background(), map[string]any{
		"title":    "Team Standup",
		"start":    "2026-05-22T09:00:00+00:00",
		"end":      "2026-05-22T09:30:00+00:00",
		"out_path": outPath,
	}, nil)
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Contains(t, res.Text, outPath)

	data, err := os.ReadFile(outPath)
	require.NoError(t, err)
	content := string(data)

	require.Contains(t, content, "BEGIN:VCALENDAR")
	require.Contains(t, content, "BEGIN:VEVENT")
	require.Contains(t, content, "SUMMARY:Team Standup")
	require.Contains(t, content, "DTSTART:20260522T090000Z")
	require.Contains(t, content, "DTEND:20260522T093000Z")
	require.Contains(t, content, "END:VEVENT")
	require.Contains(t, content, "END:VCALENDAR")
	require.Contains(t, content, "VERSION:2.0")
	require.Contains(t, content, "PRODID:-//anthrogo//EN")
}

func TestCalendar_EscapesSpecialChars(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "escaped.ics")

	res, err := CalendarEvent{}.Call(context.Background(), map[string]any{
		"title":       "Meeting, Q2; Planning\nSession",
		"start":       "2026-06-01T10:00:00+00:00",
		"end":         "2026-06-01T11:00:00+00:00",
		"description": "Bring docs,notes; items\\backslash\nnewline",
		"out_path":    outPath,
	}, nil)
	require.NoError(t, err)
	require.False(t, res.IsError)

	data, err := os.ReadFile(outPath)
	require.NoError(t, err)
	content := string(data)

	// commas, semicolons, newlines, backslashes should be escaped
	require.Contains(t, content, `Meeting\, Q2\; Planning\nSession`)
	require.Contains(t, content, `Bring docs\,notes\; items\\backslash\nnewline`)
}

func TestCalendar_BadTime(t *testing.T) {
	res, err := CalendarEvent{}.Call(context.Background(), map[string]any{
		"title": "Bad Event",
		"start": "not-a-time",
		"end":   "2026-05-22T09:30:00+00:00",
	}, nil)
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, strings.ToLower(res.Text), "bad time format")
}

func TestCalendar_EndBeforeStart(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "bad.ics")

	res, err := CalendarEvent{}.Call(context.Background(), map[string]any{
		"title":    "Backwards",
		"start":    "2026-05-22T10:00:00+00:00",
		"end":      "2026-05-22T09:00:00+00:00",
		"out_path": outPath,
	}, nil)
	require.NoError(t, err)
	require.True(t, res.IsError)
	require.Contains(t, res.Text, "end must be after start")
}

func TestCalendar_DefaultPath(t *testing.T) {
	res, err := CalendarEvent{}.Call(context.Background(), map[string]any{
		"title": "My Event",
		"start": "2026-05-22T14:00:00+00:00",
		"end":   "2026-05-22T15:00:00+00:00",
	}, nil)
	require.NoError(t, err)
	require.False(t, res.IsError)

	// Extract path from result message "wrote <path> (N bytes)"
	parts := strings.Fields(res.Text)
	require.Greater(t, len(parts), 1)
	writtenPath := parts[1]

	// Should be under TempDir and have slugified name
	require.True(t, strings.HasPrefix(writtenPath, os.TempDir()), "expected path under TempDir, got: "+writtenPath)
	require.True(t, strings.HasSuffix(writtenPath, ".ics"))
	require.Contains(t, writtenPath, "my-event")

	// File must exist
	_, err = os.Stat(writtenPath)
	require.NoError(t, err)

	// Cleanup
	_ = os.Remove(writtenPath)
}

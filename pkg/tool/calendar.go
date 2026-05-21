package tool

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
)

// CalendarEvent creates an .ics (iCalendar) file and optionally adds it to macOS Calendar.app.
type CalendarEvent struct {
	DefaultPermission
}

func (CalendarEvent) Name() string { return "CalendarEvent" }
func (CalendarEvent) Description(context.Context) string {
	return "Create a calendar event as an .ics file. On macOS, optionally adds to Calendar.app via osascript."
}
func (CalendarEvent) UserFacingName(input map[string]any) string {
	if title, _ := input["title"].(string); title != "" {
		return "CalendarEvent: " + title
	}
	return "CalendarEvent"
}
func (CalendarEvent) IsReadOnly() bool        { return false }
func (CalendarEvent) IsConcurrencySafe() bool { return true }

func (CalendarEvent) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"title": map[string]any{"type": "string"},
			"start": map[string]any{
				"type":        "string",
				"description": "RFC3339 e.g. 2026-05-22T15:00:00+08:00",
			},
			"end": map[string]any{
				"type":        "string",
				"description": "RFC3339",
			},
			"description": map[string]any{"type": "string"},
			"location":    map[string]any{"type": "string"},
			"out_path": map[string]any{
				"type":        "string",
				"description": "Absolute path for the .ics file. Default: $TMPDIR/<slugged-title>.ics",
			},
			"add_to_calendar_app": map[string]any{
				"type":        "boolean",
				"description": "macOS only. If true, opens the .ics in Calendar.app via `open`.",
			},
		},
		"required": []string{"title", "start", "end"},
	}
}

func (CalendarEvent) Call(_ context.Context, input map[string]any, _ *Context) (Result, error) {
	title, _ := input["title"].(string)
	if title == "" {
		return errResult("calendarevent: title is required"), nil
	}
	startStr, _ := input["start"].(string)
	if startStr == "" {
		return errResult("calendarevent: start is required"), nil
	}
	endStr, _ := input["end"].(string)
	if endStr == "" {
		return errResult("calendarevent: end is required"), nil
	}

	startT, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		return errResult("calendarevent: bad time format for start: " + err.Error()), nil
	}
	endT, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		return errResult("calendarevent: bad time format for end: " + err.Error()), nil
	}
	if !endT.After(startT) {
		return errResult("calendarevent: end must be after start"), nil
	}

	description, _ := input["description"].(string)
	location, _ := input["location"].(string)

	outPath, _ := input["out_path"].(string)
	if outPath == "" {
		outPath = filepath.Join(os.TempDir(), icsSlugify(title)+".ics")
	}

	now := time.Now().UTC()
	uid := uuid.New().String()

	var sb strings.Builder
	sb.WriteString("BEGIN:VCALENDAR\r\n")
	sb.WriteString("VERSION:2.0\r\n")
	sb.WriteString("PRODID:-//anthrogo//EN\r\n")
	sb.WriteString("CALSCALE:GREGORIAN\r\n")
	sb.WriteString("BEGIN:VEVENT\r\n")
	fmt.Fprintf(&sb, "UID:%s\r\n", uid)
	fmt.Fprintf(&sb, "DTSTAMP:%s\r\n", now.Format("20060102T150405Z"))
	fmt.Fprintf(&sb, "DTSTART:%s\r\n", startT.UTC().Format("20060102T150405Z"))
	fmt.Fprintf(&sb, "DTEND:%s\r\n", endT.UTC().Format("20060102T150405Z"))
	fmt.Fprintf(&sb, "SUMMARY:%s\r\n", icsEscape(title))
	if description != "" {
		fmt.Fprintf(&sb, "DESCRIPTION:%s\r\n", icsEscape(description))
	}
	if location != "" {
		fmt.Fprintf(&sb, "LOCATION:%s\r\n", icsEscape(location))
	}
	sb.WriteString("END:VEVENT\r\n")
	sb.WriteString("END:VCALENDAR\r\n")

	content := []byte(sb.String())
	if err := os.WriteFile(outPath, content, 0o644); err != nil {
		return errResult("calendarevent: write file: " + err.Error()), nil
	}

	openedNote := ""
	addToApp, _ := input["add_to_calendar_app"].(bool)
	if addToApp && runtime.GOOS == "darwin" {
		if err := exec.Command("open", outPath).Run(); err == nil {
			openedNote = " (opened in Calendar.app)"
		}
	}

	msg := fmt.Sprintf("wrote %s (%d bytes)%s", outPath, len(content), openedNote)
	return Result{Type: ResultText, Text: msg, ForLLM: msg}, nil
}

// icsEscape applies iCalendar text escaping.
func icsEscape(s string) string {
	var sb strings.Builder
	for _, r := range s {
		switch r {
		case '\\':
			sb.WriteString(`\\`)
		case ',':
			sb.WriteString(`\,`)
		case ';':
			sb.WriteString(`\;`)
		case '\n':
			sb.WriteString(`\n`)
		default:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// icsSlugify converts a title to a filesystem-safe slug.
func icsSlugify(s string) string {
	var sb strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			sb.WriteRune(r)
		} else {
			sb.WriteRune('-')
		}
	}
	slug := strings.Trim(sb.String(), "-")
	if slug == "" {
		return "event"
	}
	return slug
}

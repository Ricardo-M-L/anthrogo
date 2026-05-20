package builtins

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func tempHistoryFile(t *testing.T, lines ...string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "input_history")
	if len(lines) > 0 {
		content := strings.Join(lines, "\n") + "\n"
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatalf("write history: %v", err)
		}
	}
	return p
}

func TestHistoryEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "input_history")
	h := History{Path: path}
	res, err := h.Run(context.Background(), "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Text != "(no input history yet)" {
		t.Errorf("expected empty message, got: %q", res.Text)
	}
}

func TestHistoryList(t *testing.T) {
	p := tempHistoryFile(t, "first prompt", "second prompt", "third prompt")
	h := History{Path: p}

	// Default list (limit 20)
	res, err := h.Run(context.Background(), "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res.Text, "first prompt") {
		t.Errorf("expected 'first prompt' in output, got: %q", res.Text)
	}
	if !strings.Contains(res.Text, "third prompt") {
		t.Errorf("expected 'third prompt' in output, got: %q", res.Text)
	}

	// list with explicit limit
	res, err = h.Run(context.Background(), "list 2", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(res.Text, "first prompt") {
		t.Errorf("with limit=2, 'first prompt' should be excluded")
	}
	if !strings.Contains(res.Text, "second prompt") {
		t.Errorf("with limit=2, 'second prompt' should appear")
	}
	if !strings.Contains(res.Text, "third prompt") {
		t.Errorf("with limit=2, 'third prompt' should appear")
	}
}

func TestHistorySearch(t *testing.T) {
	p := tempHistoryFile(t, "hello world", "foo bar", "hello again")
	h := History{Path: p}

	res, err := h.Run(context.Background(), "search hello", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res.Text, "hello world") {
		t.Errorf("expected 'hello world' in search results")
	}
	if !strings.Contains(res.Text, "hello again") {
		t.Errorf("expected 'hello again' in search results")
	}
	if strings.Contains(res.Text, "foo bar") {
		t.Errorf("'foo bar' should not appear in search for 'hello'")
	}
}

func TestHistorySearchCaseInsensitive(t *testing.T) {
	p := tempHistoryFile(t, "Hello World", "lower case")
	h := History{Path: p}

	res, err := h.Run(context.Background(), "search hello", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res.Text, "Hello World") {
		t.Errorf("search should be case-insensitive")
	}
}

func TestHistorySearchNoMatches(t *testing.T) {
	p := tempHistoryFile(t, "foo", "bar")
	h := History{Path: p}

	res, err := h.Run(context.Background(), "search zzz", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Text != "(no matches)" {
		t.Errorf("expected no matches, got: %q", res.Text)
	}
}

func TestHistoryClear(t *testing.T) {
	p := tempHistoryFile(t, "something")
	h := History{Path: p}

	res, err := h.Run(context.Background(), "clear", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Text != "input history cleared" {
		t.Errorf("unexpected clear message: %q", res.Text)
	}
	// File should be gone.
	if _, statErr := os.Stat(p); !os.IsNotExist(statErr) {
		t.Errorf("history file should be removed after clear")
	}
}

func TestHistoryClearNonExistent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nope")
	h := History{Path: path}
	res, err := h.Run(context.Background(), "clear", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Text != "input history cleared" {
		t.Errorf("clearing non-existent file should succeed, got: %q", res.Text)
	}
}

func TestHistoryUnknownSubcommand(t *testing.T) {
	dir := t.TempDir()
	h := History{Path: filepath.Join(dir, "h")}
	res, err := h.Run(context.Background(), "bogus", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(res.Text, "usage:") {
		t.Errorf("expected usage message, got: %q", res.Text)
	}
}

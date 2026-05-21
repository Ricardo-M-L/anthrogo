package main

import (
	"testing"
)

func TestWebCmd_FlagsParseCleanly(t *testing.T) {
	cmd := newWebCmd()

	// Verify all expected flags are registered.
	flags := []string{
		"addr", "token", "cors-origin", "sessions-dir",
		"model", "provider", "no-browser",
	}
	for _, name := range flags {
		f := cmd.Flags().Lookup(name)
		if f == nil {
			t.Errorf("expected flag --%s to be registered", name)
		}
	}

	// Parse a set of flags to ensure no panics / errors.
	if err := cmd.ParseFlags([]string{
		"--addr", "127.0.0.1:9999",
		"--no-browser",
		"--model", "claude-opus-4-7",
	}); err != nil {
		t.Fatalf("ParseFlags returned error: %v", err)
	}

	addrFlag := cmd.Flags().Lookup("addr")
	if addrFlag == nil || addrFlag.Value.String() != "127.0.0.1:9999" {
		t.Errorf("addr flag value mismatch")
	}
	noBrowser := cmd.Flags().Lookup("no-browser")
	if noBrowser == nil || noBrowser.Value.String() != "true" {
		t.Errorf("no-browser flag should be true")
	}
}

func TestWebCmd_ResolveWebAddr_UsesFlag(t *testing.T) {
	addr, err := resolveWebAddr("127.0.0.1:19999")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addr != "127.0.0.1:19999" {
		t.Errorf("expected passthrough of flag addr, got %q", addr)
	}
}

func TestWebCmd_ResolveWebAddr_AutoDetect(t *testing.T) {
	addr, err := resolveWebAddr("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addr == "" {
		t.Fatal("expected non-empty addr")
	}
}

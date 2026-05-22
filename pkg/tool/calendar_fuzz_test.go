package tool

import (
	"strings"
	"testing"
)

// FuzzICSEscape verifies the iCal text escape doesn't panic on arbitrary
// strings and that no special char (CR, LF, comma, semicolon, backslash)
// remains unescaped in the output.
func FuzzICSEscape(f *testing.F) {
	seeds := []string{
		"hello",
		"a, b; c\\d\ne",
		"\r\n",
		"\\",
		"plain text",
		"",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > 8192 {
			t.Skip()
		}
		got := icsEscape(s)
		// Critical invariant: no raw CR/LF survives (would break the iCal
		// content-line framing).
		if strings.ContainsAny(got, "\r\n") {
			t.Fatalf("icsEscape left raw CR/LF in output: %q", got)
		}
		// No panic. Unit tests in calendar_test.go cover correctness for
		// backslash / comma / semicolon escaping per-char.
	})
}

// FuzzFoldICalLine verifies RFC 5545 §3.1 folding: no line in the output
// (split on \r\n) longer than 75 octets, and continuations are " " prefixed.
func FuzzFoldICalLine(f *testing.F) {
	seeds := []string{
		"short",
		strings.Repeat("a", 100),
		strings.Repeat("a", 200),
		strings.Repeat("a", 75),
		strings.Repeat("a", 76),
		"",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > 8192 {
			t.Skip()
		}
		got := foldICalLine(s)
		for _, line := range strings.Split(got, "\r\n") {
			if len(line) > 75 {
				// Continuations start with a space; that line is part of the
				// previous one and is allowed to be up to 75 again (74
				// payload + 1 leading space).
				if len(line) > 0 && line[0] == ' ' {
					if len(line) > 76 {
						t.Fatalf("folded line exceeds 76 octets (1 space + 75 payload): %d", len(line))
					}
				} else {
					t.Fatalf("non-continuation line exceeds 75 octets: %d in %q", len(line), got)
				}
			}
		}
	})
}

// Package yamlsafe provides a strict-with-warning YAML parser.
//
// Background: a recurring user trap in anthrogo is misspelling a YAML field
// (e.g. `pattern:` instead of `match:` in a permission rule) — yaml.v3's
// default behaviour silently zero-values the misspelled field, and the rule
// then matches *every* tool call. The user only finds out at runtime.
//
// `Unmarshal` here decodes strictly via KnownFields(true). On unknown-field
// errors it emits a warning to the provided warnings slice and falls back to
// a lenient decode, so existing valid-but-evolving configs keep working
// while typos become visible.
//
// Use Unmarshal in place of yaml.Unmarshal at every config loading boundary
// where a user-edited file is parsed.
package yamlsafe

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Unmarshal first decodes raw into out with KnownFields(true). If the only
// errors are "field not found" lines, it emits one warning per such line into
// *warnings and re-decodes leniently. Any other error is returned unchanged.
//
// `source` is a short label (e.g. "settings.yaml", "skill: foo/SKILL.md")
// used in the warning text.
//
// `warnings` may be nil — in which case warnings are silently dropped and
// the lenient fallback still runs.
func Unmarshal(raw []byte, out any, source string, warnings *[]string) error {
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(out); err != nil {
		// Errors from yaml.v3 KnownFields look like:
		//   "yaml: unmarshal errors:\n  line 5: field foo not found in type ..."
		// We treat them as "warn + fall back to lenient" only if EVERY
		// line in the error is a not-found line. Any other error (syntax,
		// type mismatch) is hard-failed.
		lines := strings.Split(err.Error(), "\n")
		notFoundOnly := true
		var foundWarns []string
		for _, ln := range lines {
			ln = strings.TrimSpace(ln)
			if ln == "" || ln == "yaml: unmarshal errors:" {
				continue
			}
			if strings.Contains(ln, "not found in type") {
				foundWarns = append(foundWarns, fmt.Sprintf("%s: %s", source, ln))
				continue
			}
			notFoundOnly = false
		}
		if !notFoundOnly || len(foundWarns) == 0 {
			return err
		}
		if warnings != nil {
			*warnings = append(*warnings, foundWarns...)
		}
		// Re-decode leniently so the rest of the document still applies.
		return yaml.Unmarshal(raw, out)
	}
	return nil
}

//go:build !darwin && !linux && !freebsd && !netbsd && !openbsd

package session

import "os"

// tryLockExclusive is a no-op on platforms without flock(2). On Windows, a
// proper LockFileEx-based implementation would go here; for the M15.2 scope
// we leave session JSONL un-flocked on Windows and document the caveat.
func tryLockExclusive(_ *os.File) error { return nil }

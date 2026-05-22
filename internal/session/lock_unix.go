//go:build darwin || linux || freebsd || netbsd || openbsd

package session

import (
	"fmt"
	"os"
	"syscall"
)

// tryLockExclusive places a non-blocking advisory exclusive lock (LOCK_EX |
// LOCK_NB) on the session JSONL fd. Returns an error if another process
// already holds the lock — meaning two anthrogo instances (e.g., TUI + serve)
// are pointed at the same session file. The caller closes the fd on error.
//
// flock(2) locks are advisory and per-file-table-entry: each os.OpenFile
// allocates a new entry, so a parent + subagent in the SAME process do NOT
// inherit the lock through fork.
func tryLockExclusive(f *os.File) error {
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		if err == syscall.EWOULDBLOCK {
			return fmt.Errorf("session JSONL already locked by another anthrogo process: %s", f.Name())
		}
		return fmt.Errorf("flock %s: %w", f.Name(), err)
	}
	return nil
}

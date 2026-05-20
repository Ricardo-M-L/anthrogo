package tui

import (
	"sync"
	"testing"
)

// TestApp_AppendServerLog_ConcurrentSafe verifies that AppendServerLog
// can be called from many goroutines (the MCP / hooks LogSink path)
// without -race fires. Regression test for the M3 review fix
// (logSinkRef + tea.Program.Send).
func TestApp_AppendServerLog_ConcurrentSafe(t *testing.T) {
	app := New(Options{})
	// Do not Run(); a.program is nil so AppendServerLog takes the
	// fallback path (direct chat.appendServerLog). The lines slice still
	// gets mutated from many goroutines — the test exists to catch that.
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			app.AppendServerLog("srv", "msg")
		}()
	}
	wg.Wait()
}

// TestApp_AppendHookLog_ConcurrentSafe is the same for the hook log rail.
func TestApp_AppendHookLog_ConcurrentSafe(t *testing.T) {
	app := New(Options{})
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			app.AppendHookLog("PreToolUse", "hook msg")
		}()
	}
	wg.Wait()
}

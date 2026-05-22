package tool

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
)

const browserMaxBytes = 200_000

// Browser implements headless-Chrome automation via chromedp.
// One shared instance is reused across calls (lazy-initialized); callers must
// serialise through it (IsConcurrencySafe returns false).
//
// The tool is NOT in the default alwaysAllow list because navigation and
// click actions can have network side-effects. The permission gate will Ask
// by default.
type Browser struct {
	DefaultPermission

	mu           sync.Mutex
	allocCtx     context.Context
	allocCancel  context.CancelFunc
	chromeCtx    context.Context
	chromeCancel context.CancelFunc
	userDataDir  string
}

func (b *Browser) Name() string { return "BrowserAction" }
func (b *Browser) Description(_ context.Context) string {
	return "Headless browser actions: navigate (get), click an element (click), or extract page text (text). Returns extracted content or status."
}
func (b *Browser) UserFacingName(input map[string]any) string {
	mode, _ := input["mode"].(string)
	switch mode {
	case "get":
		if u, ok := input["url"].(string); ok {
			return "BrowserAction get " + u
		}
	case "click":
		if s, ok := input["selector"].(string); ok {
			return "BrowserAction click " + s
		}
	case "text":
		if s, ok := input["selector"].(string); ok {
			return "BrowserAction text " + s
		}
		return "BrowserAction text"
	case "screenshot":
		if p, ok := input["out_path"].(string); ok {
			return "BrowserAction screenshot → " + p
		}
		return "BrowserAction screenshot"
	}
	return "BrowserAction"
}
func (b *Browser) IsReadOnly() bool        { return false }
func (b *Browser) IsConcurrencySafe() bool { return false }

func (b *Browser) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"mode": map[string]any{
				"type":        "string",
				"enum":        []string{"get", "click", "text", "screenshot"},
				"description": "Action: get=navigate to URL, click=click selector, text=extract page text, screenshot=PNG path",
			},
			"url": map[string]any{
				"type":        "string",
				"description": "URL for 'get'.",
			},
			"selector": map[string]any{
				"type":        "string",
				"description": "CSS selector for 'click' / 'text' / 'screenshot' (optional for text).",
			},
			"timeout_ms": map[string]any{
				"type":        "integer",
				"description": "Optional. Default 15000.",
			},
			"out_path": map[string]any{
				"type":        "string",
				"description": "Absolute output path for 'screenshot'.",
			},
		},
		"required": []string{"mode"},
	}
}

// Call dispatches the requested browser action.
func (b *Browser) Call(parent context.Context, input map[string]any, _ *Context) (Result, error) {
	mode, _ := input["mode"].(string)
	if mode == "" {
		return errResult("mode is required"), nil
	}

	// Validate mode-specific required fields before touching Chrome so that
	// input-validation tests can run without a real browser.
	url, _ := input["url"].(string)
	selector, _ := input["selector"].(string)
	outPath, _ := input["out_path"].(string)

	switch mode {
	case "get":
		if url == "" {
			return errResult("url is required for mode=get"), nil
		}
	case "click":
		if selector == "" {
			return errResult("selector is required for mode=click"), nil
		}
	case "text":
		// selector is optional; defaults to "body"
	case "screenshot":
		if outPath == "" {
			return errResult("out_path is required for mode=screenshot"), nil
		}
		if !filepath.IsAbs(outPath) {
			return errResult("out_path must be absolute"), nil
		}
	default:
		return errResult(fmt.Sprintf("unknown mode %q; must be get|click|text|screenshot", mode)), nil
	}

	timeoutMS := intField(input, "timeout_ms", 15_000)

	b.mu.Lock()
	cctx, err := b.ensureChrome()
	b.mu.Unlock()
	if err != nil {
		return errResult(fmt.Sprintf("browser init failed: %v", err)), nil
	}

	// Build a per-call context: inherits chrome context (for the CDP connection)
	// and respects the per-call timeout. Also propagates caller cancellation.
	runCtx, runCancel := context.WithTimeout(cctx, time.Duration(timeoutMS)*time.Millisecond)
	defer runCancel()
	// Propagate parent cancellation into runCtx.
	go func() {
		select {
		case <-parent.Done():
			runCancel()
		case <-runCtx.Done():
		}
	}()

	switch mode {
	case "get":
		if err := chromedp.Run(runCtx, chromedp.Navigate(url)); err != nil {
			return errResult(fmt.Sprintf("navigate error: %v", err)), nil
		}
		return Result{Type: ResultText, Text: fmt.Sprintf("navigated to %s", url), ForLLM: fmt.Sprintf("navigated to %s", url)}, nil

	case "click":
		if err := chromedp.Run(runCtx, chromedp.Click(selector, chromedp.NodeVisible)); err != nil {
			return errResult(fmt.Sprintf("click error: %v", err)), nil
		}
		msg := "clicked " + selector
		return Result{Type: ResultText, Text: msg, ForLLM: msg}, nil

	case "text":
		sel := selector
		if sel == "" {
			sel = "body"
		}
		var text string
		if err := chromedp.Run(runCtx, chromedp.Text(sel, &text, chromedp.NodeVisible)); err != nil {
			return errResult(fmt.Sprintf("text error: %v", err)), nil
		}
		truncated := false
		if len(text) > browserMaxBytes {
			text = text[:browserMaxBytes]
			truncated = true
		}
		if truncated {
			text += fmt.Sprintf("\n\n[truncated at %d bytes]", browserMaxBytes)
		}
		return Result{Type: ResultText, Text: text, ForLLM: text}, nil

	case "screenshot":
		var buf []byte
		var task chromedp.Action
		if selector == "" {
			task = chromedp.FullScreenshot(&buf, 90)
		} else {
			task = chromedp.Screenshot(selector, &buf, chromedp.NodeVisible)
		}
		if err := chromedp.Run(runCtx, task); err != nil {
			return errResult(fmt.Sprintf("screenshot error: %v", err)), nil
		}
		if err := os.WriteFile(outPath, buf, 0o644); err != nil {
			return errResult(fmt.Sprintf("write screenshot: %v", err)), nil
		}
		msg := fmt.Sprintf("saved screenshot (%d bytes) to %s", len(buf), outPath)
		return Result{Type: ResultText, Text: msg, ForLLM: msg}, nil
	}
	// unreachable
	return errResult("internal error: unhandled mode"), nil
}

// ensureChrome lazily creates the chromedp allocator and browser context.
// Must be called with b.mu held.
func (b *Browser) ensureChrome() (context.Context, error) {
	if b.chromeCtx != nil {
		return b.chromeCtx, nil
	}

	// Create a dedicated temp dir so each run gets a clean profile.
	userDataDir, err := os.MkdirTemp("", "anthrogo-chrome-*")
	if err != nil {
		return nil, fmt.Errorf("create chrome tmpdir: %w", err)
	}

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Headless,
		chromedp.UserDataDir(userDataDir),
		chromedp.Flag("disable-gpu", true),
	)
	// Only disable the Chrome sandbox when running as root (e.g. inside a container).
	// On macOS or a normal Linux user account, the sandbox provides meaningful protection.
	if os.Getuid() == 0 {
		opts = append(opts, chromedp.NoSandbox)
	}
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	chromeCtx, chromeCancel := chromedp.NewContext(allocCtx)

	b.allocCtx = allocCtx
	b.allocCancel = allocCancel
	b.chromeCtx = chromeCtx
	b.chromeCancel = chromeCancel
	b.userDataDir = userDataDir
	return chromeCtx, nil
}

// Close shuts down the headless Chrome instance if it was started.
// Called by main on shutdown.
func (b *Browser) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.chromeCancel != nil {
		b.chromeCancel()
		b.chromeCancel = nil
	}
	if b.allocCancel != nil {
		b.allocCancel()
		b.allocCancel = nil
	}
	b.chromeCtx = nil
	b.allocCtx = nil
	if b.userDataDir != "" {
		_ = os.RemoveAll(b.userDataDir)
		b.userDataDir = ""
	}
	return nil
}

package tool

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/ledongthuc/pdf"
)

const pdfMaxBytes = 200_000

// PDFRead extracts plain text from a local PDF file.
type PDFRead struct{ DefaultPermission }

func (PDFRead) Name() string                      { return "PDFRead" }
func (PDFRead) Description(context.Context) string { return pdfReadDescription }
func (PDFRead) IsReadOnly() bool                  { return true }
func (PDFRead) IsConcurrencySafe() bool           { return true }
func (PDFRead) UserFacingName(input map[string]any) string {
	if p, ok := input["file_path"].(string); ok {
		return "PDFRead " + p
	}
	return "PDFRead"
}

func (PDFRead) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"file_path": map[string]any{
				"type":        "string",
				"description": "Absolute path to the PDF file.",
			},
			"pages": map[string]any{
				"type":        "string",
				"description": `Optional page range: "1-5", "3", or "10-end". Default: all pages.`,
			},
		},
		"required": []string{"file_path"},
	}
}

func (PDFRead) Call(ctx context.Context, input map[string]any, _ *Context) (Result, error) {
	path, ok := input["file_path"].(string)
	if !ok || path == "" {
		return errResult("file_path is required"), nil
	}
	if r, isErr := errIfNotAbs("file_path", path); isErr {
		return r, nil
	}

	f, r, err := pdf.Open(path)
	if err != nil {
		return errResult(fmt.Sprintf("cannot open PDF: %v", err)), nil
	}
	defer f.Close()

	numPages := r.NumPage()
	if numPages == 0 {
		return errResult("pdf has no pages"), nil
	}

	startPage, endPage, parseErr := parsePDFPageRange(input["pages"], numPages)
	if parseErr != nil {
		return errResult(parseErr.Error()), nil
	}
	if startPage < 1 || endPage > numPages || startPage > endPage {
		return errResult(fmt.Sprintf("page range %d-%d is out of bounds (PDF has %d page(s))", startPage, endPage, numPages)), nil
	}

	fonts := make(map[string]*pdf.Font)
	var sb strings.Builder
	for i := startPage; i <= endPage; i++ {
		p := r.Page(i)
		for _, name := range p.Fonts() {
			if _, ok := fonts[name]; !ok {
				f2 := p.Font(name)
				fonts[name] = &f2
			}
		}
		text, err := p.GetPlainText(fonts)
		if err != nil {
			return errResult(fmt.Sprintf("error reading page %d: %v", i, err)), nil
		}
		sb.WriteString(text)
		if sb.Len() >= pdfMaxBytes {
			break
		}
	}

	out := truncatePDFText(sb.String(), pdfMaxBytes)
	return Result{Type: ResultText, Text: out, ForLLM: out}, nil
}

// truncatePDFText caps s at max bytes, appending a truncation marker when cut.
// Extracted as a testable helper.
func truncatePDFText(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + fmt.Sprintf("\n\n[truncated at %d bytes]", max)
}

// parsePDFPageRange parses the "pages" input value, returning (start, end) 1-indexed.
// Empty / nil → all pages. "N" → page N only. "N-M" → N to M. "N-end" → N to total.
func parsePDFPageRange(pages any, total int) (int, int, error) {
	raw := ""
	if pages != nil {
		raw, _ = pages.(string)
	}
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" || raw == "all" {
		return 1, total, nil
	}
	if idx := strings.Index(raw, "-"); idx >= 0 {
		startStr := strings.TrimSpace(raw[:idx])
		endStr := strings.TrimSpace(raw[idx+1:])
		start, err := strconv.Atoi(startStr)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid page range %q: bad start", pages)
		}
		var end int
		if endStr == "end" {
			end = total
		} else {
			end, err = strconv.Atoi(endStr)
			if err != nil {
				return 0, 0, fmt.Errorf("invalid page range %q: bad end", pages)
			}
		}
		return start, end, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid pages value %q", pages)
	}
	return n, n, nil
}


const pdfReadDescription = `Extract text from a local PDF file. Returns plain text, optionally limited to a page range (e.g. "1-5", "3", "10-end"). Output is capped at 200 KB.`

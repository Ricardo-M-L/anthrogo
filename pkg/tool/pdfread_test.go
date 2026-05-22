package tool

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

// samplePDFPath returns the absolute path to the test PDF fixture.
func samplePDFPath(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller failed")
	return filepath.Join(filepath.Dir(thisFile), "testdata", "sample.pdf")
}

func TestPDFRead_AllPages(t *testing.T) {
	p := samplePDFPath(t)
	got, err := PDFRead{}.Call(context.Background(), map[string]any{"file_path": p}, &Context{})
	require.NoError(t, err)
	require.False(t, got.IsError, "unexpected error result: %s", got.Text)
	require.NotEmpty(t, got.Text)
	// The sample PDF contains "heading" and "content".
	require.Contains(t, got.Text, "heading")
}

func TestPDFRead_PageRange(t *testing.T) {
	p := samplePDFPath(t)
	// Range "1-1" on a 1-page PDF should return the same text.
	got, err := PDFRead{}.Call(context.Background(), map[string]any{
		"file_path": p,
		"pages":     "1-1",
	}, &Context{})
	require.NoError(t, err)
	require.False(t, got.IsError)
	require.Contains(t, got.Text, "heading")
}

func TestPDFRead_PageN(t *testing.T) {
	p := samplePDFPath(t)
	got, err := PDFRead{}.Call(context.Background(), map[string]any{
		"file_path": p,
		"pages":     "1",
	}, &Context{})
	require.NoError(t, err)
	require.False(t, got.IsError)
	require.Contains(t, got.Text, "content")
}

func TestPDFRead_PageRangeOutOfBounds(t *testing.T) {
	p := samplePDFPath(t)
	// The sample PDF has 1 page; requesting page 99 should produce IsError.
	got, err := PDFRead{}.Call(context.Background(), map[string]any{
		"file_path": p,
		"pages":     "99",
	}, &Context{})
	require.NoError(t, err)
	require.True(t, got.IsError, "expected error for out-of-bounds page")
}

func TestPDFRead_FileNotFound(t *testing.T) {
	got, err := PDFRead{}.Call(context.Background(), map[string]any{
		"file_path": "/nonexistent/does_not_exist.pdf",
	}, &Context{})
	require.NoError(t, err)
	require.True(t, got.IsError)
}

func TestPDFRead_RelativePath_Rejected(t *testing.T) {
	got, err := PDFRead{}.Call(context.Background(), map[string]any{
		"file_path": "relative/path.pdf",
	}, &Context{})
	require.NoError(t, err)
	require.True(t, got.IsError)
	require.Contains(t, got.Text, "absolute")
}

// TestPDFRead_PageRange_BeyondAvailable_IsError verifies that requesting pages
// beyond the available page count returns IsError.
func TestPDFRead_PageRange_BeyondAvailable_IsError(t *testing.T) {
	p := samplePDFPath(t)
	got, err := PDFRead{}.Call(context.Background(), map[string]any{
		"file_path": p,
		"pages":     "5-10",
	}, &Context{})
	require.NoError(t, err)
	require.True(t, got.IsError, "expected error for pages=5-10 on 1-page PDF")
}

// TestPDFRead_PageRange_NEndForm verifies that pages="1-end" on a 1-page PDF
// returns the same text as pages="1".
func TestPDFRead_PageRange_NEndForm(t *testing.T) {
	p := samplePDFPath(t)
	gotAll, err := PDFRead{}.Call(context.Background(), map[string]any{
		"file_path": p,
		"pages":     "1",
	}, &Context{})
	require.NoError(t, err)
	require.False(t, gotAll.IsError)

	gotEnd, err := PDFRead{}.Call(context.Background(), map[string]any{
		"file_path": p,
		"pages":     "1-end",
	}, &Context{})
	require.NoError(t, err)
	require.False(t, gotEnd.IsError)
	require.Equal(t, gotAll.Text, gotEnd.Text, "pages='1-end' should equal pages='1' on a 1-page PDF")
}

// TestTruncatePDFText_NoTruncation verifies that short strings are returned unchanged.
func TestTruncatePDFText_NoTruncation(t *testing.T) {
	s := "hello world"
	got := truncatePDFText(s, 100)
	require.Equal(t, s, got)
}

// TestTruncatePDFText_TruncatesAndAddsMarker verifies that long strings are cut
// and the truncation marker is appended.
func TestTruncatePDFText_TruncatesAndAddsMarker(t *testing.T) {
	s := "abcdefghij" // 10 bytes
	got := truncatePDFText(s, 5)
	require.Equal(t, "abcde\n\n[truncated at 5 bytes]", got)
	require.True(t, len(got) > 5, "marker appended after cut")
}

// TestTruncatePDFText_ExactBoundary verifies a string of exactly max bytes is
// returned unchanged (no marker).
func TestTruncatePDFText_ExactBoundary(t *testing.T) {
	s := "12345"
	got := truncatePDFText(s, 5)
	require.Equal(t, s, got)
}

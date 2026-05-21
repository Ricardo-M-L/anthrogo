package tool

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/xuri/excelize/v2"
)

// makeSampleXlsx creates a temp .xlsx file with two sheets and returns its path.
//
//	Sheet1: A1=Name, B1=Score, A2=Alice, B2=95, A3=Bob, B3=82
//	Sheet2: A1=Color, B1=Hex, A2=Red, B2=#FF0000
func makeSampleXlsx(t *testing.T) string {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()

	// Sheet1 (already exists as default)
	require.NoError(t, f.SetCellValue("Sheet1", "A1", "Name"))
	require.NoError(t, f.SetCellValue("Sheet1", "B1", "Score"))
	require.NoError(t, f.SetCellValue("Sheet1", "A2", "Alice"))
	require.NoError(t, f.SetCellValue("Sheet1", "B2", 95))
	require.NoError(t, f.SetCellValue("Sheet1", "A3", "Bob"))
	require.NoError(t, f.SetCellValue("Sheet1", "B3", 82))

	// Sheet2
	_, err := f.NewSheet("Sheet2")
	require.NoError(t, err)
	require.NoError(t, f.SetCellValue("Sheet2", "A1", "Color"))
	require.NoError(t, f.SetCellValue("Sheet2", "B1", "Hex"))
	require.NoError(t, f.SetCellValue("Sheet2", "A2", "Red"))
	require.NoError(t, f.SetCellValue("Sheet2", "B2", "#FF0000"))

	dir := t.TempDir()
	out := filepath.Join(dir, "sample.xlsx")
	require.NoError(t, f.SaveAs(out))
	return out
}

func TestXlsxRead_DefaultSheet(t *testing.T) {
	p := makeSampleXlsx(t)
	got, err := XlsxRead{}.Call(context.Background(), map[string]any{"file_path": p}, &Context{})
	require.NoError(t, err)
	require.False(t, got.IsError, "unexpected error: %s", got.Text)
	require.Contains(t, got.Text, "Name")
	require.Contains(t, got.Text, "Alice")
	require.Contains(t, got.Text, "Bob")
}

func TestXlsxRead_NamedSheet(t *testing.T) {
	p := makeSampleXlsx(t)
	got, err := XlsxRead{}.Call(context.Background(), map[string]any{
		"file_path": p,
		"sheet":     "Sheet2",
	}, &Context{})
	require.NoError(t, err)
	require.False(t, got.IsError)
	require.Contains(t, got.Text, "Color")
	require.Contains(t, got.Text, "FF0000")
}

func TestXlsxRead_Range(t *testing.T) {
	p := makeSampleXlsx(t)
	got, err := XlsxRead{}.Call(context.Background(), map[string]any{
		"file_path": p,
		"sheet":     "Sheet1",
		"range":     "A1:B2",
	}, &Context{})
	require.NoError(t, err)
	require.False(t, got.IsError)
	require.Contains(t, got.Text, "Name")
	require.Contains(t, got.Text, "Alice")
	// Row 3 (Bob) should NOT be present since range is A1:B2
	require.False(t, strings.Contains(got.Text, "Bob"), "Bob should not appear in A1:B2 range")
}

func TestXlsxRead_BadSheet(t *testing.T) {
	p := makeSampleXlsx(t)
	// excelize.GetRows returns an error for a non-existent sheet name.
	// XlsxRead should forward that as an IsError result.
	got, err := XlsxRead{}.Call(context.Background(), map[string]any{
		"file_path": p,
		"sheet":     "NoSuchSheet",
	}, &Context{})
	require.NoError(t, err)
	require.True(t, got.IsError, "expected IsError for non-existent sheet; got: %q", got.Text)
	require.Contains(t, got.Text, "sheet", "error message should mention 'sheet'")
}

func TestXlsxRead_FileNotFound(t *testing.T) {
	got, err := XlsxRead{}.Call(context.Background(), map[string]any{
		"file_path": "/nonexistent/missing.xlsx",
	}, &Context{})
	require.NoError(t, err)
	require.True(t, got.IsError)
}

func TestXlsxRead_RelativePath_Rejected(t *testing.T) {
	got, err := XlsxRead{}.Call(context.Background(), map[string]any{
		"file_path": "relative/file.xlsx",
	}, &Context{})
	require.NoError(t, err)
	require.True(t, got.IsError)
	require.Contains(t, got.Text, "absolute")
}

// TestXlsxRead_Range_InvertedFails verifies that an inverted range (end before
// start) returns IsError rather than panicking or returning empty rows.
func TestXlsxRead_Range_InvertedFails(t *testing.T) {
	p := makeSampleXlsx(t)
	got, err := XlsxRead{}.Call(context.Background(), map[string]any{
		"file_path": p,
		"sheet":     "Sheet1",
		"range":     "B2:A1",
	}, &Context{})
	require.NoError(t, err)
	require.True(t, got.IsError, "expected IsError for inverted range B2:A1; got: %q", got.Text)
}

func TestXlsxRead_TSVFormat(t *testing.T) {
	p := makeSampleXlsx(t)
	got, err := XlsxRead{}.Call(context.Background(), map[string]any{"file_path": p}, &Context{})
	require.NoError(t, err)
	require.False(t, got.IsError)
	lines := strings.Split(strings.TrimSpace(got.Text), "\n")
	require.GreaterOrEqual(t, len(lines), 3)
	// Header row should be tab-separated
	require.Contains(t, lines[0], "\t")
}

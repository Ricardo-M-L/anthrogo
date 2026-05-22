package tool

import (
	"context"
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
)

const xlsxMaxBytes = 200_000

// XlsxRead reads a sheet from a local .xlsx file and returns cells as TSV.
type XlsxRead struct{ DefaultPermission }

func (XlsxRead) Name() string                      { return "XlsxRead" }
func (XlsxRead) Description(context.Context) string { return xlsxReadDescription }
func (XlsxRead) IsReadOnly() bool                  { return true }
func (XlsxRead) IsConcurrencySafe() bool           { return true }
func (XlsxRead) UserFacingName(input map[string]any) string {
	if p, ok := input["file_path"].(string); ok {
		return "XlsxRead " + p
	}
	return "XlsxRead"
}

func (XlsxRead) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"file_path": map[string]any{
				"type":        "string",
				"description": "Absolute path to the .xlsx file.",
			},
			"sheet": map[string]any{
				"type":        "string",
				"description": "Sheet name. Default: first sheet.",
			},
			"range": map[string]any{
				"type":        "string",
				"description": `Optional A1 range like "A1:D10". Default: all rows.`,
			},
		},
		"required": []string{"file_path"},
	}
}

func (XlsxRead) Call(ctx context.Context, input map[string]any, _ *Context) (Result, error) {
	path, ok := input["file_path"].(string)
	if !ok || path == "" {
		return errResult("file_path is required"), nil
	}
	if r, isErr := errIfNotAbs("file_path", path); isErr {
		return r, nil
	}

	f, err := excelize.OpenFile(path)
	if err != nil {
		return errResult(fmt.Sprintf("cannot open xlsx: %v", err)), nil
	}
	defer f.Close()

	sheetName, _ := input["sheet"].(string)
	sheetName = strings.TrimSpace(sheetName)
	if sheetName == "" {
		sheets := f.GetSheetList()
		if len(sheets) == 0 {
			return errResult("xlsx has no sheets"), nil
		}
		sheetName = sheets[0]
	}

	rangeStr, _ := input["range"].(string)
	rangeStr = strings.TrimSpace(rangeStr)

	var rows [][]string
	if rangeStr == "" {
		rows, err = f.GetRows(sheetName)
		if err != nil {
			return errResult(fmt.Sprintf("cannot read sheet %q: %v", sheetName, err)), nil
		}
	} else {
		rows, err = xlsxReadRange(f, sheetName, rangeStr)
		if err != nil {
			return errResult(err.Error()), nil
		}
	}

	var sb strings.Builder
	truncated := false
	for _, row := range rows {
		sb.WriteString(strings.Join(row, "\t"))
		sb.WriteByte('\n')
		if sb.Len() >= xlsxMaxBytes {
			truncated = true
			break
		}
	}
	out := sb.String()
	if len(out) > xlsxMaxBytes {
		out = out[:xlsxMaxBytes]
		truncated = true
	}
	if truncated {
		out += fmt.Sprintf("\n[truncated at %d bytes]", xlsxMaxBytes)
	}
	return Result{Type: ResultText, Text: out, ForLLM: out}, nil
}

// xlsxReadRange iterates cells within an A1:B2-style range.
func xlsxReadRange(f *excelize.File, sheet, cellRange string) ([][]string, error) {
	parts := strings.SplitN(cellRange, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid range %q: expected format like A1:D10", cellRange)
	}
	startCol, startRow, err := excelize.CellNameToCoordinates(strings.TrimSpace(parts[0]))
	if err != nil {
		return nil, fmt.Errorf("invalid range start %q: %v", parts[0], err)
	}
	endCol, endRow, err := excelize.CellNameToCoordinates(strings.TrimSpace(parts[1]))
	if err != nil {
		return nil, fmt.Errorf("invalid range end %q: %v", parts[1], err)
	}
	if startRow > endRow || startCol > endCol {
		return nil, fmt.Errorf("range start must be before end in %q", cellRange)
	}

	result := make([][]string, 0, endRow-startRow+1)
	for row := startRow; row <= endRow; row++ {
		rowData := make([]string, 0, endCol-startCol+1)
		for col := startCol; col <= endCol; col++ {
			cellName, err := excelize.CoordinatesToCellName(col, row)
			if err != nil {
				rowData = append(rowData, "")
				continue
			}
			val, _ := f.GetCellValue(sheet, cellName)
			rowData = append(rowData, val)
		}
		result = append(result, rowData)
	}
	return result, nil
}

const xlsxReadDescription = `Read a sheet of a local .xlsx file. Returns cells as tab-separated values (TSV). Specify sheet name or defaults to the first sheet. Optional A1 range restricts output. Capped at 200 KB.`

package spreadsheet_test

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"

	"goskeleton/internal/spreadsheet"
)

func writeWorkbook(t *testing.T, sheets map[string][][]any) *bytes.Reader {
	t.Helper()

	workbook := excelize.NewFile()
	first := true
	for name, rows := range sheets {
		var sheet string
		if first {
			original := workbook.GetSheetName(0)
			if err := workbook.SetSheetName(original, name); err != nil {
				t.Fatalf("rename sheet: %v", err)
			}
			sheet = name
			first = false
		} else {
			created, err := workbook.NewSheet(name)
			if err != nil {
				t.Fatalf("new sheet: %v", err)
			}
			sheet = workbook.GetSheetName(created)
		}

		for rowIndex, row := range rows {
			for columnIndex, value := range row {
				cell, err := excelize.CoordinatesToCellName(columnIndex+1, rowIndex+1)
				if err != nil {
					t.Fatalf("cell name: %v", err)
				}
				if err := workbook.SetCellValue(sheet, cell, value); err != nil {
					t.Fatalf("set cell: %v", err)
				}
			}
		}
	}

	buffer, err := workbook.WriteToBuffer()
	if err != nil {
		t.Fatalf("write workbook: %v", err)
	}

	return bytes.NewReader(buffer.Bytes())
}

func TestParseCSV(t *testing.T) {
	t.Parallel()

	workbook, err := spreadsheet.Parse("transactions.csv", strings.NewReader(
		"date,type,amount,description,account,category\n2026-10-01,expense,12.50,Coffee,Cash,Food\n",
	))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(workbook.Sheets) != 1 {
		t.Fatalf("expected 1 sheet, got %d", len(workbook.Sheets))
	}
	table, err := workbook.Pick("")
	if err != nil {
		t.Fatalf("pick sheet: %v", err)
	}
	if table.Rows[1][0] != "2026-10-01" || table.Rows[1][2] != "12.50" {
		t.Fatalf("unexpected row: %+v", table.Rows[1])
	}
}

func TestParseXLSXNormalisesDates(t *testing.T) {
	t.Parallel()

	body := writeWorkbook(t, map[string][][]any{
		"Sheet1": {
			{"date", "type", "amount", "description", "account", "category"},
			{time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), "expense", 12.5, "Coffee", "Cash", "Food"},
		},
	})

	workbook, err := spreadsheet.Parse("transactions.xlsx", body)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	table, err := workbook.Pick("")
	if err != nil {
		t.Fatalf("pick sheet: %v", err)
	}
	if table.Rows[1][0] != "2026-10-01" {
		t.Fatalf("expected normalised date, got %q", table.Rows[1][0])
	}
	if table.Rows[1][2] != "12.5" {
		t.Fatalf("expected numeric amount, got %q", table.Rows[1][2])
	}
}

func TestParseXLSXKeepsEveryPopulatedSheet(t *testing.T) {
	t.Parallel()

	body := writeWorkbook(t, map[string][][]any{
		"October": {
			{"date", "amount"},
			{"2026-10-01", 10},
		},
		"November": {
			{"date", "amount"},
			{"2026-11-01", 20},
		},
	})

	workbook, err := spreadsheet.Parse("book.xlsx", body)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	names := workbook.Names()
	if len(names) != 2 {
		t.Fatalf("expected 2 sheets, got %v", names)
	}

	selected, err := workbook.Pick("november")
	if err != nil {
		t.Fatalf("expected case-insensitive sheet lookup, got %v", err)
	}
	if selected.Name != "November" || selected.Rows[1][1] != "20" {
		t.Fatalf("unexpected selected sheet: %+v", selected)
	}

	if _, err := workbook.Pick("missing"); !errors.Is(err, spreadsheet.ErrSheetNotFound) {
		t.Fatalf("expected ErrSheetNotFound, got %v", err)
	}
}

func TestTableToCSV(t *testing.T) {
	t.Parallel()

	workbook, err := spreadsheet.Parse("rows.csv", strings.NewReader("a,b\n1,2\n"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	table, err := workbook.Pick("")
	if err != nil {
		t.Fatalf("pick sheet: %v", err)
	}

	encoded, err := table.ToCSV()
	if err != nil {
		t.Fatalf("encode csv: %v", err)
	}
	if encoded != "a,b\n1,2\n" {
		t.Fatalf("unexpected csv %q", encoded)
	}
}

func TestParseRejectsTooManyColumns(t *testing.T) {
	t.Parallel()

	header := strings.Repeat("c,", spreadsheet.MaxColumns) + "c"
	_, err := spreadsheet.Parse("wide.csv", strings.NewReader(header+"\n"))
	if !errors.Is(err, spreadsheet.ErrTooManyColumns) {
		t.Fatalf("expected ErrTooManyColumns, got %v", err)
	}
}

func TestParseRejectsTooManySheets(t *testing.T) {
	t.Parallel()

	workbook := excelize.NewFile()
	for i := 0; i < spreadsheet.MaxSheets; i++ {
		if _, err := workbook.NewSheet(fmt.Sprintf("S%d", i)); err != nil {
			t.Fatalf("new sheet: %v", err)
		}
	}

	buffer, err := workbook.WriteToBuffer()
	if err != nil {
		t.Fatalf("write workbook: %v", err)
	}

	if _, err := spreadsheet.Parse("many.xlsx", bytes.NewReader(buffer.Bytes())); !errors.Is(err, spreadsheet.ErrTooManySheets) {
		t.Fatalf("expected ErrTooManySheets, got %v", err)
	}
}

func TestParseRejectsUnsupportedFormat(t *testing.T) {
	t.Parallel()

	_, err := spreadsheet.Parse("notes.txt", strings.NewReader("hello"))
	if !errors.Is(err, spreadsheet.ErrUnsupportedFormat) {
		t.Fatalf("expected ErrUnsupportedFormat, got %v", err)
	}
}

func TestParseRejectsEmptyFile(t *testing.T) {
	t.Parallel()

	_, err := spreadsheet.Parse("empty.csv", strings.NewReader(""))
	if !errors.Is(err, spreadsheet.ErrEmptyWorkbook) {
		t.Fatalf("expected ErrEmptyWorkbook, got %v", err)
	}
}

func TestParseTrimsTrailingEmptyColumns(t *testing.T) {
	t.Parallel()

	workbook, err := spreadsheet.Parse("rows.csv", strings.NewReader("a,b,c\n1,,,\n"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	table, err := workbook.Pick("")
	if err != nil {
		t.Fatalf("pick sheet: %v", err)
	}
	if len(table.Rows[1]) != 1 || table.Rows[1][0] != "1" {
		t.Fatalf("expected trailing empties trimmed, got %+v", table.Rows[1])
	}
}

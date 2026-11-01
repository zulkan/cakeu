package spreadsheet

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/shakinm/xlsReader/xls"
	"github.com/shakinm/xlsReader/xls/structure"
	"github.com/xuri/excelize/v2"
)

var (
	ErrUnsupportedFormat = errors.New("unsupported file format")
	ErrEmptyWorkbook     = errors.New("the file has no data")
	ErrTooManyRows       = errors.New("the file has too many rows")
	ErrTooManySheets     = errors.New("the file has too many sheets")
	ErrTooManyColumns    = errors.New("the file has too many columns")
	ErrSheetNotFound     = errors.New("sheet not found")
)

// MaxRows, MaxColumns and MaxSheets bound how much of a workbook is read.
const (
	MaxRows    = 5001
	MaxColumns = 50
	MaxSheets  = 50
)

// maxInspectedCells caps the per-cell type lookups so a large sheet stays fast.
const maxInspectedCells = 50000

// Table is one sheet of an uploaded file.
type Table struct {
	Name string
	Rows [][]string
}

// Workbook holds every populated sheet of an uploaded file.
type Workbook struct {
	Sheets []Table
}

func (w *Workbook) Names() []string {
	names := make([]string, 0, len(w.Sheets))
	for _, sheet := range w.Sheets {
		names = append(names, sheet.Name)
	}
	return names
}

// Pick returns the named sheet, or the first populated sheet when name is empty.
func (w *Workbook) Pick(name string) (*Table, error) {
	if len(w.Sheets) == 0 {
		return nil, ErrEmptyWorkbook
	}

	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return &w.Sheets[0], nil
	}

	for i := range w.Sheets {
		if strings.EqualFold(w.Sheets[i].Name, trimmed) {
			return &w.Sheets[i], nil
		}
	}

	return nil, fmt.Errorf("%w: %q", ErrSheetNotFound, trimmed)
}

func (t *Table) ToCSV() (string, error) {
	var builder strings.Builder
	writer := csv.NewWriter(&builder)

	for _, row := range t.Rows {
		if err := writer.Write(row); err != nil {
			return "", fmt.Errorf("encode csv: %w", err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", fmt.Errorf("encode csv: %w", err)
	}

	return builder.String(), nil
}

var supportedExtensions = map[string]bool{
	".csv":  true,
	".xlsx": true,
	".xls":  true,
}

func Parse(filename string, r io.Reader) (*Workbook, error) {
	extension := strings.ToLower(filepath.Ext(filename))
	if !supportedExtensions[extension] {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedFormat, extension)
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read upload: %w", err)
	}
	if len(data) == 0 {
		return nil, ErrEmptyWorkbook
	}

	switch extension {
	case ".csv":
		return parseCSV(data)
	case ".xlsx":
		return parseXLSX(data)
	default:
		return parseXLS(data)
	}
}

func parseCSV(data []byte) (*Workbook, error) {
	reader := csv.NewReader(bytes.NewReader(data))
	reader.FieldsPerRecord = -1

	rows := make([][]string, 0, 32)
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse csv: %w", err)
		}
		if len(rows) >= MaxRows {
			return nil, ErrTooManyRows
		}

		trimmed, trimErr := trimRow(record)
		if trimErr != nil {
			return nil, trimErr
		}
		rows = append(rows, trimmed)
	}

	return &Workbook{Sheets: []Table{{Name: "csv", Rows: rows}}}, nil
}

func parseXLSX(data []byte) (*Workbook, error) {
	workbook, err := excelize.OpenReader(bytes.NewReader(data), excelize.Options{
		UnzipSizeLimit:    64 << 20,
		UnzipXMLSizeLimit: 16 << 20,
	})
	if err != nil {
		return nil, fmt.Errorf("open xlsx: %w", err)
	}
	defer func() {
		_ = workbook.Close()
	}()

	use1904 := false
	if props, propsErr := workbook.GetWorkbookProps(); propsErr == nil && props.Date1904 != nil {
		use1904 = *props.Date1904
	}

	sheetNames := workbook.GetSheetList()
	if len(sheetNames) > MaxSheets {
		return nil, ErrTooManySheets
	}

	sheets := make([]Table, 0, len(sheetNames))
	for _, name := range sheetNames {
		rows, rowsErr := workbook.GetRows(name, excelize.Options{RawCellValue: true})
		if rowsErr != nil {
			return nil, fmt.Errorf("read xlsx rows: %w", rowsErr)
		}
		if len(rows) > MaxRows {
			return nil, ErrTooManyRows
		}
		if len(rows) == 0 {
			continue
		}

		normalizeXLSXDates(workbook, name, rows, use1904)
		for i, row := range rows {
			trimmed, trimErr := trimRow(row)
			if trimErr != nil {
				return nil, trimErr
			}
			rows[i] = trimmed
		}

		sheets = append(sheets, Table{Name: name, Rows: rows})
	}

	if len(sheets) == 0 {
		return nil, ErrEmptyWorkbook
	}

	return &Workbook{Sheets: sheets}, nil
}

func normalizeXLSXDates(workbook *excelize.File, sheet string, rows [][]string, use1904 bool) {
	inspected := 0
	dateStyles := make(map[int]bool, 8)

	for i, row := range rows {
		for j, value := range row {
			if value == "" || inspected >= maxInspectedCells {
				continue
			}
			inspected++

			axis, err := excelize.CoordinatesToCellName(j+1, i+1)
			if err != nil {
				continue
			}
			if !cellIsDate(workbook, sheet, axis, dateStyles) {
				continue
			}

			serial, err := strconv.ParseFloat(value, 64)
			if err != nil {
				continue
			}
			converted, err := excelize.ExcelDateToTime(serial, use1904)
			if err != nil {
				continue
			}
			rows[i][j] = converted.Format("2006-01-02")
		}
	}
}

func cellIsDate(workbook *excelize.File, sheet, axis string, cache map[int]bool) bool {
	styleID, err := workbook.GetCellStyle(sheet, axis)
	if err != nil {
		return false
	}

	if isDate, ok := cache[styleID]; ok {
		return isDate
	}

	isDate := false
	if style, styleErr := workbook.GetStyle(styleID); styleErr == nil {
		isDate = isBuiltInDateFormat(style.NumFmt) ||
			(style.CustomNumFmt != nil && isDateFormat(*style.CustomNumFmt))
	}

	cache[styleID] = isDate
	return isDate
}

func isBuiltInDateFormat(numFmt int) bool {
	switch {
	case numFmt >= 14 && numFmt <= 22:
		return true
	case numFmt >= 27 && numFmt <= 36:
		return true
	case numFmt >= 45 && numFmt <= 47:
		return true
	case numFmt >= 50 && numFmt <= 58:
		return true
	default:
		return false
	}
}

func parseXLS(data []byte) (*Workbook, error) {
	workbook, err := xls.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("open xls: %w", err)
	}

	if workbook.GetNumberSheets() > MaxSheets {
		return nil, ErrTooManySheets
	}

	sheets := make([]Table, 0, workbook.GetNumberSheets())
	for index := 0; index < workbook.GetNumberSheets(); index++ {
		sheet, sheetErr := workbook.GetSheet(index)
		if sheetErr != nil || sheet == nil {
			continue
		}

		rawRows := sheet.GetRows()
		if len(rawRows) > MaxRows {
			return nil, ErrTooManyRows
		}

		rows := make([][]string, 0, len(rawRows))
		for _, rawRow := range rawRows {
			cells := rawRow.GetCols()
			values := make([]string, 0, len(cells))
			for _, cell := range cells {
				values = append(values, xlsCellValue(&workbook, cell))
			}
			trimmed, trimErr := trimRow(values)
			if trimErr != nil {
				return nil, trimErr
			}
			rows = append(rows, trimmed)
		}

		if len(rows) == 0 {
			continue
		}

		sheets = append(sheets, Table{Name: sheet.GetName(), Rows: rows})
	}

	if len(sheets) == 0 {
		return nil, ErrEmptyWorkbook
	}

	return &Workbook{Sheets: sheets}, nil
}

func xlsCellValue(workbook *xls.Workbook, cell structure.CellData) string {
	if cell.GetType() != "numeric" {
		return strings.TrimSpace(cell.GetString())
	}

	serial := cell.GetFloat64()
	if serial > 0 && isDateFormat(xlsFormatString(workbook, cell)) {
		return excelSerialToDate(serial).Format("2006-01-02")
	}

	return strconv.FormatFloat(serial, 'f', -1, 64)
}

func xlsFormatString(workbook *xls.Workbook, cell structure.CellData) string {
	defer func() {
		if recovered := recover(); recovered != nil {
			slog.Warn("xls format lookup failed", "panic", recovered)
		}
	}()

	index := cell.GetXFIndex()
	if index <= 0 {
		return ""
	}

	format := workbook.GetFormatByIndex(index)

	return format.String()
}

func isDateFormat(format string) bool {
	cleaned := strings.ToLower(format)
	cleaned = stripQuoted(cleaned)
	cleaned = stripBracketed(cleaned)

	return strings.ContainsAny(cleaned, "yd") || strings.Contains(cleaned, "m/") || strings.Contains(cleaned, "/m")
}

func stripQuoted(value string) string {
	for {
		start := strings.Index(value, `"`)
		if start < 0 {
			return value
		}
		end := strings.Index(value[start+1:], `"`)
		if end < 0 {
			return value[:start]
		}
		value = value[:start] + value[start+end+2:]
	}
}

func stripBracketed(value string) string {
	for {
		start := strings.Index(value, "[")
		if start < 0 {
			return value
		}
		end := strings.Index(value[start:], "]")
		if end < 0 {
			return value
		}
		value = value[:start] + value[start+end+1:]
	}
}

func excelSerialToDate(serial float64) time.Time {
	days := int(serial)
	fraction := serial - float64(days)
	date := time.Date(1899, 12, 30, 0, 0, 0, 0, time.UTC).AddDate(0, 0, days)

	return date.Add(time.Duration(fraction * float64(24*time.Hour)))
}

func trimRow(row []string) ([]string, error) {
	if len(row) > MaxColumns {
		return nil, fmt.Errorf("%w: found %d columns, maximum is %d", ErrTooManyColumns, len(row), MaxColumns)
	}

	trimmed := make([]string, len(row))
	for i, value := range row {
		trimmed[i] = strings.TrimSpace(value)
	}

	for len(trimmed) > 0 && trimmed[len(trimmed)-1] == "" {
		trimmed = trimmed[:len(trimmed)-1]
	}

	return trimmed, nil
}

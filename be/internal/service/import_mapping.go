package service

import (
	"encoding/csv"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"goskeleton/internal/model"
	"goskeleton/internal/spreadsheet"
)

var ErrInvalidMapping = errors.New("invalid import mapping")

const (
	MaxMappingBytes = 32 << 10

	maxMappingSkipRows    = 200
	maxMappingValueMap    = 200
	maxMappingStringLen   = 120
	maxMappingNotesLen    = 500
	canonicalImportHeader = "date,type,amount,description,account,category"
)

var (
	validDateFormats = map[string]bool{
		"": true, "iso": true, "dmy": true, "mdy": true,
		"d_month_y_id": true, "d_month_y_en": true, "d_mon_y_id": true, "excel_serial": true,
	}
	validAmountFormats = map[string]bool{
		"": true, "plain": true, "id_grouped": true, "en_grouped": true, "currency_id": true, "currency_en": true,
	}
	validAmountModes = map[string]bool{"": true, "as_is": true, "abs": true, "negate": true}
	monthNames       = map[string]time.Month{
		"januari": time.January, "january": time.January, "jan": time.January,
		"februari": time.February, "february": time.February, "feb": time.February,
		"maret": time.March, "march": time.March, "mar": time.March,
		"april": time.April, "apr": time.April,
		"mei": time.May, "may": time.May,
		"juni": time.June, "june": time.June, "jun": time.June,
		"juli": time.July, "july": time.July, "jul": time.July,
		"agustus": time.August, "august": time.August, "agu": time.August, "aug": time.August,
		"september": time.September, "sept": time.September, "sep": time.September,
		"oktober": time.October, "october": time.October, "okt": time.October, "oct": time.October,
		"november": time.November, "nov": time.November,
		"desember": time.December, "december": time.December, "des": time.December, "dec": time.December,
	}
)

type resolvedMapping struct {
	plan       *model.ImportMapping
	dataStart  int
	dataEnd    int
	maxColumns int
}

// ValidateMapping checks everything that can be checked without user data.
func ValidateMapping(plan *model.ImportMapping, table *spreadsheet.Table) error {
	_, err := resolveMapping(plan, table)
	return err
}

func resolveMapping(plan *model.ImportMapping, table *spreadsheet.Table) (*resolvedMapping, error) {
	if plan == nil {
		return nil, fmt.Errorf("%w: mapping is required", ErrInvalidMapping)
	}
	if table == nil || len(table.Rows) == 0 {
		return nil, fmt.Errorf("%w: the sheet has no rows", ErrInvalidMapping)
	}

	if plan.Sheet != "" && !strings.EqualFold(strings.TrimSpace(plan.Sheet), strings.TrimSpace(table.Name)) {
		return nil, fmt.Errorf("%w: mapping is for sheet %q, not %q", ErrInvalidMapping, plan.Sheet, table.Name)
	}

	maxColumns := 0
	for _, row := range table.Rows {
		if len(row) > maxColumns {
			maxColumns = len(row)
		}
	}
	if maxColumns == 0 {
		return nil, fmt.Errorf("%w: the sheet has no columns", ErrInvalidMapping)
	}

	if plan.HeaderRow < 0 || plan.HeaderRow > len(table.Rows) {
		return nil, fmt.Errorf("%w: header_row %d is out of range", ErrInvalidMapping, plan.HeaderRow)
	}

	dataStart := plan.DataStart
	if dataStart == 0 {
		dataStart = plan.HeaderRow + 1
	}
	if dataStart < 1 {
		return nil, fmt.Errorf("%w: data_start %d is out of range", ErrInvalidMapping, plan.DataStart)
	}
	dataEnd := plan.DataEnd
	if dataEnd == 0 {
		dataEnd = len(table.Rows)
	}
	if dataEnd < 1 || dataStart > dataEnd || dataEnd > len(table.Rows) {
		return nil, fmt.Errorf("%w: data range %d-%d is invalid", ErrInvalidMapping, dataStart, dataEnd)
	}

	if len(plan.SkipRows) > maxMappingSkipRows {
		return nil, fmt.Errorf("%w: too many skip_rows", ErrInvalidMapping)
	}
	for _, row := range plan.SkipRows {
		if row < 1 || row > len(table.Rows) {
			return nil, fmt.Errorf("%w: skip row %d is out of range", ErrInvalidMapping, row)
		}
	}

	specs := []struct {
		name     string
		rule     model.ImportFieldRule
		required bool
	}{
		{"date", plan.Fields.Date, true},
		{"amount", plan.Fields.Amount, true},
		{"description", plan.Fields.Description, false},
		{"account", plan.Fields.Account, true},
		{"category", plan.Fields.Category, true},
	}
	for _, spec := range specs {
		if err := validateFieldRule(spec.name, spec.rule, spec.required, maxColumns); err != nil {
			return nil, err
		}
	}

	if err := validateTypeRule(plan.Type, maxColumns); err != nil {
		return nil, err
	}

	if utf8.RuneCountInString(plan.Notes) > maxMappingNotesLen {
		return nil, fmt.Errorf("%w: notes are too long", ErrInvalidMapping)
	}
	if plan.Confidence < 0 || plan.Confidence > 1 {
		return nil, fmt.Errorf("%w: confidence must be between 0 and 1", ErrInvalidMapping)
	}

	return &resolvedMapping{plan: plan, dataStart: dataStart, dataEnd: dataEnd, maxColumns: maxColumns}, nil
}

func validateFieldRule(name string, rule model.ImportFieldRule, required bool, maxColumns int) error {
	if rule.Column < 0 || rule.Column > maxColumns {
		return fmt.Errorf("%w: %s column %d is out of range", ErrInvalidMapping, name, rule.Column)
	}
	if name == "date" || name == "amount" {
		if rule.Column < 1 {
			return fmt.Errorf("%w: %s must map to a column", ErrInvalidMapping, name)
		}
		if strings.TrimSpace(rule.Constant) != "" {
			return fmt.Errorf("%w: %s must not use a constant", ErrInvalidMapping, name)
		}
		if len(rule.ValueMap) > 0 {
			return fmt.Errorf("%w: %s must not use value_map", ErrInvalidMapping, name)
		}
	} else if rule.Column == 0 && strings.TrimSpace(rule.Constant) == "" && required {
		return fmt.Errorf("%w: %s needs a column or a constant", ErrInvalidMapping, name)
	}
	if utf8.RuneCountInString(rule.Constant) > maxMappingStringLen {
		return fmt.Errorf("%w: %s constant is too long", ErrInvalidMapping, name)
	}
	if len(rule.DateFormat) > maxMappingStringLen || !validDateFormats[rule.DateFormat] {
		return fmt.Errorf("%w: unknown date_format %q", ErrInvalidMapping, rule.DateFormat)
	}
	if !validAmountFormats[rule.AmountFormat] {
		return fmt.Errorf("%w: unknown amount_format %q", ErrInvalidMapping, rule.AmountFormat)
	}
	if !validAmountModes[rule.AmountMode] {
		return fmt.Errorf("%w: unknown amount_mode %q", ErrInvalidMapping, rule.AmountMode)
	}
	if len(rule.ValueMap) > maxMappingValueMap {
		return fmt.Errorf("%w: %s value_map is too large", ErrInvalidMapping, name)
	}
	for source, target := range rule.ValueMap {
		if utf8.RuneCountInString(source) > maxMappingStringLen || utf8.RuneCountInString(target) > maxMappingStringLen || strings.TrimSpace(target) == "" {
			return fmt.Errorf("%w: %s value_map entry is invalid", ErrInvalidMapping, name)
		}
	}
	return nil
}

func validateTypeRule(rule model.ImportTypeRule, maxColumns int) error {
	if rule.Column < 0 || rule.Column > maxColumns {
		return fmt.Errorf("%w: type column %d is out of range", ErrInvalidMapping, rule.Column)
	}
	if rule.Column == 0 && rule.Constant != TypeIncome && rule.Constant != TypeExpense {
		return fmt.Errorf("%w: type constant must be income or expense", ErrInvalidMapping)
	}
	if len(rule.ValueMap) > maxMappingValueMap {
		return fmt.Errorf("%w: type value_map is too large", ErrInvalidMapping)
	}
	for source, target := range rule.ValueMap {
		if utf8.RuneCountInString(source) > maxMappingStringLen || utf8.RuneCountInString(target) > maxMappingStringLen || strings.TrimSpace(target) == "" {
			return fmt.Errorf("%w: type value_map entry is invalid", ErrInvalidMapping)
		}
		target = strings.ToLower(strings.TrimSpace(target))
		if target != TypeIncome && target != TypeExpense {
			return fmt.Errorf("%w: type value_map target must be income or expense", ErrInvalidMapping)
		}
	}
	return nil
}

// ApplyMapping turns a sheet into the canonical CSV plus the original row numbers.
// It returns the CSV text, the origin row for each data row, and the skipped count.
func ApplyMapping(plan *model.ImportMapping, table *spreadsheet.Table) (string, []int, int, error) {
	resolved, err := resolveMapping(plan, table)
	if err != nil {
		return "", nil, 0, err
	}

	skip := make(map[int]bool, len(plan.SkipRows))
	for _, row := range plan.SkipRows {
		skip[row] = true
	}

	carry := map[string]string{"date": "", "amount": "", "description": "", "account": "", "category": ""}
	var builder strings.Builder
	writer := csv.NewWriter(&builder)
	if err := writer.Write(strings.Split(canonicalImportHeader, ",")); err != nil {
		return "", nil, 0, fmt.Errorf("write header: %w", err)
	}

	origins := make([]int, 0, resolved.dataEnd-resolved.dataStart+1)
	skipped := 0
	descriptionMapped := plan.Fields.Description.Column > 0 || strings.TrimSpace(plan.Fields.Description.Constant) != ""
	for rowNumber := resolved.dataStart; rowNumber <= resolved.dataEnd; rowNumber++ {
		if skip[rowNumber] {
			skipped++
			continue
		}

		row := table.Rows[rowNumber-1]
		dateValue := fieldValue(row, plan.Fields.Date, carry, "date")
		amountValue := fieldValue(row, plan.Fields.Amount, carry, "amount")
		descriptionValue := fieldValue(row, plan.Fields.Description, carry, "description")
		accountValue := fieldValue(row, plan.Fields.Account, carry, "account")
		categoryValue := fieldValue(row, plan.Fields.Category, carry, "category")
		typeValue := typeValue(row, plan.Type)

		if plan.Fields.Date.Column > 0 && strings.TrimSpace(dateValue) == "" {
			if !descriptionMapped || strings.TrimSpace(descriptionValue) == "" {
				skipped++
				continue
			}
		}

		record := []string{
			normalizeDate(dateValue, plan.Fields.Date.DateFormat),
			typeValue,
			normalizeAmount(amountValue, plan.Fields.Amount.AmountFormat, plan.Fields.Amount.AmountMode),
			descriptionValue,
			accountValue,
			categoryValue,
		}
		if err := writer.Write(record); err != nil {
			return "", nil, 0, fmt.Errorf("write row %d: %w", rowNumber, err)
		}
		origins = append(origins, rowNumber)
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", nil, 0, fmt.Errorf("encode mapping: %w", err)
	}

	return builder.String(), origins, skipped, nil
}

// RenumberImportResult rewrites canonical CSV line numbers back to sheet rows.
func RenumberImportResult(result *model.ImportResult, origins []int) {
	if result == nil {
		return
	}
	remap := func(line int) int {
		index := line - 2
		if index < 0 || index >= len(origins) {
			return line
		}
		return origins[index]
	}
	for i := range result.Errors {
		result.Errors[i].Row = remap(result.Errors[i].Row)
	}
	for i := range result.Duplicates {
		result.Duplicates[i].Row = remap(result.Duplicates[i].Row)
	}
	for i := range result.Preview {
		result.Preview[i].Row = remap(result.Preview[i].Row)
	}
}

func fieldValue(row []string, rule model.ImportFieldRule, carry map[string]string, key string) string {
	if rule.Column <= 0 {
		return strings.TrimSpace(rule.Constant)
	}

	value := strings.TrimSpace(cellAt(row, rule.Column))
	if rule.CarryForward {
		if value == "" {
			value = carry[key]
		} else {
			carry[key] = value
		}
	}
	if mapped, ok := mapValue(value, rule.ValueMap); ok {
		return mapped
	}
	return value
}

func typeValue(row []string, rule model.ImportTypeRule) string {
	if rule.Column <= 0 {
		return strings.ToLower(strings.TrimSpace(rule.Constant))
	}
	value := strings.ToLower(strings.TrimSpace(cellAt(row, rule.Column)))
	if mapped, ok := mapValue(value, rule.ValueMap); ok {
		return strings.ToLower(mapped)
	}
	return value
}

func mapValue(value string, mapping map[string]string) (string, bool) {
	if len(mapping) == 0 {
		return "", false
	}
	if mapped, ok := mapping[value]; ok {
		return mapped, true
	}
	for source, mapped := range mapping {
		if strings.EqualFold(strings.TrimSpace(source), value) {
			return mapped, true
		}
	}
	return "", false
}

func cellAt(row []string, column int) string {
	if column < 1 || column > len(row) {
		return ""
	}
	return row[column-1]
}

func normalizeDate(raw, format string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	switch format {
	case "", "iso":
		return raw
	case "dmy":
		for _, layout := range []string{"02/01/2006", "2/1/2006", "02-01-2006", "2-1-2006"} {
			if parsed, err := time.Parse(layout, raw); err == nil {
				return parsed.Format("2006-01-02")
			}
		}
	case "mdy":
		for _, layout := range []string{"01/02/2006", "1/2/2006", "01-02-2006", "1-2-2006"} {
			if parsed, err := time.Parse(layout, raw); err == nil {
				return parsed.Format("2006-01-02")
			}
		}
	case "d_month_y_id", "d_month_y_en", "d_mon_y_id":
		if parsed, ok := parseMonthNameDate(raw); ok {
			return parsed.Format("2006-01-02")
		}
	case "excel_serial":
		if serial, err := strconv.ParseFloat(raw, 64); err == nil {
			return spreadsheet.SerialToDate(serial).Format("2006-01-02")
		}
	}
	return raw
}

func parseMonthNameDate(raw string) (time.Time, bool) {
	cleaned := strings.NewReplacer(",", " ", ".", " ").Replace(strings.ToLower(raw))
	fields := strings.Fields(cleaned)
	if len(fields) != 3 {
		return time.Time{}, false
	}
	day, err := strconv.Atoi(fields[0])
	if err != nil {
		return time.Time{}, false
	}
	month, ok := monthNames[fields[1]]
	if !ok {
		return time.Time{}, false
	}
	yearField := fields[2]
	if len(yearField) < 4 {
		return time.Time{}, false
	}
	year, err := strconv.Atoi(yearField)
	if err != nil {
		return time.Time{}, false
	}
	parsed := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
	if parsed.Day() != day || parsed.Month() != month || parsed.Year() != year {
		return time.Time{}, false
	}
	return parsed, true
}

func normalizeAmount(raw, format, mode string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}

	switch format {
	case "id_grouped":
		value = strings.ReplaceAll(value, ".", "")
		value = strings.ReplaceAll(value, ",", ".")
	case "en_grouped":
		value = strings.ReplaceAll(value, ",", "")
	case "currency_id":
		value = strings.NewReplacer("Rp", "", "IDR", "", " ", "").Replace(value)
		value = strings.ReplaceAll(value, ".", "")
		value = strings.ReplaceAll(value, ",", ".")
	case "currency_en":
		value = strings.NewReplacer("$", "", "USD", "", " ", "", ",", "").Replace(value)
	case "plain", "":
		if ambiguousPlainAmount(value) {
			return raw
		}
	}

	number, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
		return raw
	}

	switch mode {
	case "abs":
		number = math.Abs(number)
	case "negate":
		number = -number
	}
	return strconv.FormatFloat(number, 'f', -1, 64)
}

// ambiguousPlainAmount reports values whose separators could mean either
// decimal or thousands grouping. Plain amounts must be unambiguous; the
// caller keeps the raw text so the strict validator rejects it instead of
// silently importing a value that is off by 1000x.
func ambiguousPlainAmount(value string) bool {
	if strings.Contains(value, ",") {
		return true
	}
	trimmed := strings.TrimPrefix(strings.TrimPrefix(value, "+"), "-")
	dot := strings.IndexByte(trimmed, '.')
	if dot < 0 || dot != strings.LastIndexByte(trimmed, '.') {
		return false
	}
	integer, fraction := trimmed[:dot], trimmed[dot+1:]
	return len(integer) >= 1 && len(integer) <= 3 && len(fraction) == 3
}

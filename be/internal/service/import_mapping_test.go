package service_test

import (
	"errors"
	"strings"
	"testing"

	"goskeleton/internal/model"
	"goskeleton/internal/service"
	"goskeleton/internal/spreadsheet"
)

// januariTable mirrors the shape of the real "budgeting 2026.xlsx" sheet:
// rows 1-2 are a summary, rows 3-6 are blank, row 7 is the header,
// rows 8-12 contain pocket groups, a total row and dated expenses.
func januariTable() *spreadsheet.Table {
	return &spreadsheet.Table{
		Name: "JANUARI",
		Rows: [][]string{
			{"Income : ", "10000000"},      // 1
			{"sisa main pocket ", "12934"}, // 2
			{},                             // 3
			{},                             // 4
			{},                             // 5
			{},                             // 6
			{"POCKET", "UANG MASUK", "TOTAL", "EXPENSES", "TANGGAL", "KETERANGAN", "SISA"}, // 7
			{"Saving", "4000000", "5080003", "", "", "", ""},                               // 8
			{"", "", "", "1500000", "14 januari 2026", "main pocket", ""},                  // 9
			{"", "", "", "3700000", "", "", "1380003"},                                     // 10
			{"Listrik dan air", "500000", "607550", "", "", "", ""},                        // 11
			{"", "", "", "300000", "21 januari 2026", "listrik", ""},                       // 12
		},
	}
}

func januariMapping() *model.ImportMapping {
	return &model.ImportMapping{
		Sheet:     "JANUARI",
		HeaderRow: 7,
		DataStart: 8,
		DataEnd:   12,
		SkipRows:  []int{10},
		Fields: model.ImportFields{
			Date:        model.ImportFieldRule{Column: 5, DateFormat: "d_month_y_id"},
			Amount:      model.ImportFieldRule{Column: 4},
			Description: model.ImportFieldRule{Column: 6},
			Account:     model.ImportFieldRule{Constant: "Cash"},
			Category:    model.ImportFieldRule{Column: 1, CarryForward: true},
		},
		Type: model.ImportTypeRule{Constant: "expense"},
	}
}

func TestApplyMappingExtractsAndCarriesForward(t *testing.T) {
	csvText, origins, skipped, err := service.ApplyMapping(januariMapping(), januariTable())
	if err != nil {
		t.Fatalf("ApplyMapping: %v", err)
	}

	want := strings.Join([]string{
		"date,type,amount,description,account,category",
		"2026-01-14,expense,1500000,main pocket,Cash,Saving",
		"2026-01-21,expense,300000,listrik,Cash,Listrik dan air",
		"",
	}, "\n")
	if csvText != want {
		t.Fatalf("unexpected csv:\n%q\nwant:\n%q", csvText, want)
	}
	if len(origins) != 2 || origins[0] != 9 || origins[1] != 12 {
		t.Fatalf("unexpected origins: %v", origins)
	}
	if skipped != 3 {
		t.Fatalf("expected 3 skipped rows, got %d", skipped)
	}
}

func TestValidateMappingRejectsBadPlans(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*model.ImportMapping)
	}{
		{"nil", nil},
		{"sheet mismatch", func(m *model.ImportMapping) { m.Sheet = "FEBRUARI" }},
		{"header row out of range", func(m *model.ImportMapping) { m.HeaderRow = 99 }},
		{"data range inverted", func(m *model.ImportMapping) { m.DataStart = 12; m.DataEnd = 8 }},
		{"column out of range", func(m *model.ImportMapping) { m.Fields.Date.Column = 9 }},
		{"missing account", func(m *model.ImportMapping) { m.Fields.Account = model.ImportFieldRule{} }},
		{"bad date format", func(m *model.ImportMapping) { m.Fields.Date.DateFormat = "nope" }},
		{"bad amount mode", func(m *model.ImportMapping) { m.Fields.Amount.AmountMode = "nope" }},
		{"skip row out of range", func(m *model.ImportMapping) { m.SkipRows = []int{99} }},
		{"bad type constant", func(m *model.ImportMapping) { m.Type = model.ImportTypeRule{Constant: "transfer"} }},
		{"constant date invalid", func(m *model.ImportMapping) {
			m.Fields.Date = model.ImportFieldRule{Constant: "14/01/2026"}
		}},
		{"date constant is not allowed", func(m *model.ImportMapping) {
			m.Fields.Date = model.ImportFieldRule{Constant: "2026-01-14"}
		}},
		{"amount constant is not allowed", func(m *model.ImportMapping) {
			m.Fields.Amount = model.ImportFieldRule{Constant: "1500"}
		}},
		{"date value_map is not allowed", func(m *model.ImportMapping) {
			m.Fields.Date.ValueMap = map[string]string{"14/01/2026": "2026-01-14"}
		}},
		{"amount value_map is not allowed", func(m *model.ImportMapping) {
			m.Fields.Amount.ValueMap = map[string]string{"1.500": "1500"}
		}},
		{"negative data_start", func(m *model.ImportMapping) { m.DataStart = -1 }},
		{"type value_map target must be an enum", func(m *model.ImportMapping) {
			m.Type = model.ImportTypeRule{Column: 2, ValueMap: map[string]string{"in": "transfer"}}
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var plan *model.ImportMapping
			if tc.mutate != nil {
				plan = januariMapping()
				tc.mutate(plan)
			}
			err := service.ValidateMapping(plan, januariTable())
			if !errors.Is(err, service.ErrInvalidMapping) {
				t.Fatalf("expected ErrInvalidMapping, got %v", err)
			}
		})
	}
}

func TestNormalizeFormatsThroughApplyMapping(t *testing.T) {
	table := &spreadsheet.Table{
		Name: "Sheet1",
		Rows: [][]string{
			{"Date", "Amount", "Description", "Account", "Category"},
			{"14 januari 2026", "Rp 1.500.000", "listrik", "Cash", "Utilities"},
			{"21/01/2026", "1,500,000.50", "air", "Cash", "Utilities"},
			{"45292", "42.5", "serial", "Cash", "Utilities"},
		},
	}

	plan := &model.ImportMapping{
		Sheet: "Sheet1", HeaderRow: 1, DataStart: 2, DataEnd: 4,
		Fields: model.ImportFields{
			Date:        model.ImportFieldRule{Column: 1, DateFormat: "d_month_y_id"},
			Amount:      model.ImportFieldRule{Column: 2, AmountFormat: "currency_id"},
			Description: model.ImportFieldRule{Column: 3},
			Account:     model.ImportFieldRule{Column: 4},
			Category:    model.ImportFieldRule{Column: 5},
		},
		Type: model.ImportTypeRule{Constant: "expense"},
	}
	csvText, _, _, err := service.ApplyMapping(plan, table)
	if err != nil {
		t.Fatalf("ApplyMapping: %v", err)
	}
	if !strings.Contains(csvText, "2026-01-14,expense,1500000,listrik,Cash,Utilities") {
		t.Fatalf("Indonesian date/currency not normalized: %q", csvText)
	}

	plan.Fields.Date.DateFormat = "dmy"
	plan.Fields.Amount.AmountFormat = "en_grouped"
	csvText, _, _, err = service.ApplyMapping(plan, table)
	if err != nil {
		t.Fatalf("ApplyMapping: %v", err)
	}
	if !strings.Contains(csvText, "2026-01-21,expense,1500000.5,air,Cash,Utilities") {
		t.Fatalf("dmy/en grouped not normalized: %q", csvText)
	}

	plan.Fields.Date.DateFormat = "excel_serial"
	plan.Fields.Amount.AmountFormat = "plain"
	csvText, _, _, err = service.ApplyMapping(plan, table)
	if err != nil {
		t.Fatalf("ApplyMapping: %v", err)
	}
	if !strings.Contains(csvText, "2024-01-01,expense,42.5,serial,Cash,Utilities") {
		t.Fatalf("serial date not normalized: %q", csvText)
	}
}

func TestRenumberImportResult(t *testing.T) {
	result := &model.ImportResult{
		Errors:  []model.ImportRowError{{Row: 2, Message: "bad"}},
		Preview: []model.ImportPreviewRow{{Row: 2}},
	}
	service.RenumberImportResult(result, []int{9, 12})
	if result.Errors[0].Row != 9 || result.Preview[0].Row != 9 {
		t.Fatalf("expected row 9, got %+v", result)
	}
}

func TestNormalizeDateAndAmountEnums(t *testing.T) {
	tests := []struct {
		name       string
		date       string
		dateFormat string
		amount     string
		amountFmt  string
		mode       string
		wantDate   string
		wantAmount string
	}{
		{"mdy dashes", "1-2-2026", "mdy", "42.5", "plain", "", "2026-01-02", "42.5"},
		{"day month name id", "14 Jan 26", "d_mon_y_id", "1500", "plain", "", "14 Jan 26", "1500"},
		{"day month name id long", "31 Desember 2026", "d_mon_y_id", "1500", "plain", "", "2026-12-31", "1500"},
		{"id grouped", "2026-01-01", "iso", "1.234.567", "id_grouped", "", "2026-01-01", "1234567"},
		{"en grouped", "2026-01-01", "iso", "1,234.56", "en_grouped", "abs", "2026-01-01", "1234.56"},
		{"currency en", "2026-01-01", "iso", "$1,234.56", "currency_en", "negate", "2026-01-01", "-1234.56"},
		{"abs mode", "2026-01-01", "iso", "-42.5", "plain", "abs", "2026-01-01", "42.5"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			table := &spreadsheet.Table{
				Name: "Sheet1",
				Rows: [][]string{
					{"Date", "Amount"},
					{tc.date, tc.amount},
				},
			}
			plan := &model.ImportMapping{
				Sheet: "Sheet1", HeaderRow: 1, DataStart: 2, DataEnd: 2,
				Fields: model.ImportFields{
					Date:     model.ImportFieldRule{Column: 1, DateFormat: tc.dateFormat},
					Amount:   model.ImportFieldRule{Column: 2, AmountFormat: tc.amountFmt, AmountMode: tc.mode},
					Account:  model.ImportFieldRule{Column: 1},
					Category: model.ImportFieldRule{Column: 2},
				},
				Type: model.ImportTypeRule{Constant: "expense"},
			}

			csvText, _, _, err := service.ApplyMapping(plan, table)
			if err != nil {
				t.Fatalf("ApplyMapping: %v", err)
			}
			want := tc.wantDate + ",expense," + tc.wantAmount
			if !strings.Contains(csvText, want) {
				t.Fatalf("expected %q in %q", want, csvText)
			}
		})
	}
}

func TestApplyMappingPlainAmountDoesNotGuessGrouping(t *testing.T) {
	table := &spreadsheet.Table{
		Name: "Sheet1",
		Rows: [][]string{
			{"Date", "Amount"},
			{"2026-01-01", "12,345"},
			{"2026-01-02", "1.500"},
			{"2026-01-03", "42.5"},
		},
	}
	plan := &model.ImportMapping{
		Sheet: "Sheet1", HeaderRow: 1, DataStart: 2, DataEnd: 4,
		Fields: model.ImportFields{
			Date:     model.ImportFieldRule{Column: 1, DateFormat: "iso"},
			Amount:   model.ImportFieldRule{Column: 2, AmountFormat: "plain"},
			Account:  model.ImportFieldRule{Column: 1},
			Category: model.ImportFieldRule{Column: 2},
		},
		Type: model.ImportTypeRule{Constant: "expense"},
	}

	csvText, _, _, err := service.ApplyMapping(plan, table)
	if err != nil {
		t.Fatalf("ApplyMapping: %v", err)
	}
	if !strings.Contains(csvText, "2026-01-01,expense,\"12,345\"") {
		t.Fatalf("comma-grouped plain value must stay raw: %q", csvText)
	}
	if !strings.Contains(csvText, "2026-01-02,expense,1.500") {
		t.Fatalf("ambiguous plain value must stay raw: %q", csvText)
	}
	if !strings.Contains(csvText, "2026-01-03,expense,42.5") {
		t.Fatalf("plain decimal should still parse: %q", csvText)
	}
}

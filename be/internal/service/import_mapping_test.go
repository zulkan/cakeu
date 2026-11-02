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

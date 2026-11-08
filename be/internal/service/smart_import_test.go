package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"goskeleton/internal/llm"
	"goskeleton/internal/model"
	"goskeleton/internal/service"
	"goskeleton/internal/spreadsheet"
)

type stubImportLLM struct {
	replies []string
	err     error
	calls   int
}

func (s *stubImportLLM) Complete(ctx context.Context, messages []llm.Message, tools []llm.Tool) (*llm.Message, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	if len(s.replies) == 0 {
		return nil, errors.New("no scripted reply")
	}
	reply := s.replies[0]
	if len(s.replies) > 1 {
		s.replies = s.replies[1:]
	}
	return &llm.Message{Role: "assistant", Content: reply}, nil
}

func newImportServiceForTest(t *testing.T, client llm.Client) (service.ImportService, *fakeAccountRepo, *fakeCategoryRepo) {
	t.Helper()
	accountRepo := &fakeAccountRepo{accounts: []model.Account{
		{ID: uuid.New(), Name: "Cash"},
	}}
	categoryRepo := &fakeCategoryRepo{categories: []model.Category{
		{ID: uuid.New(), Name: "Saving", Type: "expense"},
		{ID: uuid.New(), Name: "Listrik dan air", Type: "expense"},
	}}
	transactions := service.NewTransactionService(&fakeTransactionRepo{}, accountRepo, categoryRepo)
	imports := service.NewImportService(client, transactions, accountRepo, categoryRepo,
		service.ImportSettings{Mode: service.ImportModeAI, SampleRows: 30}, nil)
	return imports, accountRepo, categoryRepo
}

func TestImportServicePreviewUsesAIMapping(t *testing.T) {
	reply := `{
		"sheet":"JANUARI","header_row":7,"data_start":8,"data_end":12,"skip_rows":[10],
		"fields":{
			"date":{"column":5,"date_format":"d_month_y_id"},
			"amount":{"column":4},
			"description":{"column":6},
			"account":{"constant":"Cash"},
			"category":{"column":1,"carry_forward":true}
		},
		"type":{"constant":"expense"}
	}`
	imports, _, _ := newImportServiceForTest(t, &stubImportLLM{replies: []string{reply}})

	result, err := imports.Preview(context.Background(), uuid.New(), januariTable(), []string{"JANUARI"},
		service.ImportOptions{Filename: "budgeting 2026.xlsx", Sheet: "JANUARI", Mode: service.ImportModeAI})
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if result.Mode != "ai" || result.Mapping == nil {
		t.Fatalf("expected ai mode and mapping, got %+v", result)
	}
	if result.Valid != 2 || result.Skipped != 3 || len(result.Errors) != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if result.Preview[0].Row != 9 {
		t.Fatalf("expected original row 9, got %d", result.Preview[0].Row)
	}
}

func TestImportServiceFallsBackToStrictWhenNotConfigured(t *testing.T) {
	table := &spreadsheet.Table{Name: "csv", Rows: [][]string{
		{"date", "type", "amount", "description", "account", "category"},
		{"2026-01-14", "expense", "1500000", "main pocket", "Cash", "Saving"},
	}}
	imports, _, _ := newImportServiceForTest(t, &stubImportLLM{err: llm.ErrNotConfigured})

	result, err := imports.Preview(context.Background(), uuid.New(), table, []string{"csv"},
		service.ImportOptions{Filename: "x.csv", Sheet: "csv", Mode: service.ImportModeAI})
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if result.Mode != "strict" || result.Mapping != nil {
		t.Fatalf("expected strict fallback, got %+v", result)
	}
	if result.Valid != 1 {
		t.Fatalf("expected 1 valid row, got %d", result.Valid)
	}
}

func TestImportServiceRepairsMalformedMappingOnce(t *testing.T) {
	reply := `{"sheet":"JANUARI","header_row":7,"data_start":8,"data_end":12,"skip_rows":[10],
		"fields":{"date":{"column":5,"date_format":"d_month_y_id"},"amount":{"column":4},
		"description":{"column":6},"account":{"constant":"Cash"},
		"category":{"column":1,"carry_forward":true}},"type":{"constant":"expense"}}`
	client := &stubImportLLM{replies: []string{"not json", reply}}
	imports, _, _ := newImportServiceForTest(t, client)

	if _, err := imports.Preview(context.Background(), uuid.New(), januariTable(), []string{"JANUARI"},
		service.ImportOptions{Filename: "budgeting 2026.xlsx", Sheet: "JANUARI", Mode: service.ImportModeAI}); err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if client.calls != 2 {
		t.Fatalf("expected one repair retry, got %d calls", client.calls)
	}
}

func TestImportServicePreviewCarriesForwardBlankDates(t *testing.T) {
	table := &spreadsheet.Table{Name: "SHEET", Rows: [][]string{
		{"Date", "Amount", "Description"},
		{"2026-01-14", "1000", "first"},
		{"", "2000", "second"},
		{"", "3000", "third"},
	}}
	reply := `{
		"sheet":"SHEET","header_row":1,"data_start":2,"data_end":4,
		"fields":{
			"date":{"column":1,"date_format":"iso"},
			"amount":{"column":2},
			"description":{"column":3},
			"account":{"constant":"Cash"},
			"category":{"constant":"Saving"}
		},
		"type":{"constant":"expense"}
	}`
	imports, _, _ := newImportServiceForTest(t, &stubImportLLM{replies: []string{reply}})

	result, err := imports.Preview(context.Background(), uuid.New(), table, []string{"SHEET"},
		service.ImportOptions{Filename: "book.xlsx", Sheet: "SHEET", Mode: service.ImportModeAI})
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if result.Valid != 3 || result.Invalid != 0 || len(result.Errors) != 0 {
		t.Fatalf("expected 3 valid rows and no errors, got %+v", result)
	}
	for _, row := range result.Preview {
		if row.Date != "2026-01-14" {
			t.Fatalf("expected carried date on row %d, got %s", row.Row, row.Date)
		}
	}
}

func TestImportServicePreviewReportsBlankDateWithoutPrevious(t *testing.T) {
	table := &spreadsheet.Table{Name: "SHEET", Rows: [][]string{
		{"Date", "Amount", "Description"},
		{"", "2000", "orphan"},
		{"2026-01-14", "1000", "first"},
	}}
	reply := `{
		"sheet":"SHEET","header_row":1,"data_start":2,"data_end":3,
		"fields":{
			"date":{"column":1,"date_format":"iso"},
			"amount":{"column":2},
			"description":{"column":3},
			"account":{"constant":"Cash"},
			"category":{"constant":"Saving"}
		},
		"type":{"constant":"expense"}
	}`
	imports, _, _ := newImportServiceForTest(t, &stubImportLLM{replies: []string{reply}})

	result, err := imports.Preview(context.Background(), uuid.New(), table, []string{"SHEET"},
		service.ImportOptions{Filename: "book.xlsx", Sheet: "SHEET", Mode: service.ImportModeAI})
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if result.Valid != 1 || result.Invalid != 1 || len(result.Errors) != 1 {
		t.Fatalf("expected 1 valid and 1 invalid row, got %+v", result)
	}
	if result.Errors[0].Row != 2 {
		t.Fatalf("expected the error on row 2, got row %d", result.Errors[0].Row)
	}
	if !strings.Contains(result.Errors[0].Message, "no previous date") {
		t.Fatalf("expected the reason in %q", result.Errors[0].Message)
	}
}

func TestImportServiceReturnsUnavailableAfterTwoBadReplies(t *testing.T) {
	imports, _, _ := newImportServiceForTest(t, &stubImportLLM{replies: []string{"no", "still no"}})

	_, err := imports.Preview(context.Background(), uuid.New(), januariTable(), []string{"JANUARI"},
		service.ImportOptions{Filename: "budgeting 2026.xlsx", Sheet: "JANUARI", Mode: service.ImportModeAI})
	if !errors.Is(err, service.ErrImportAIUnavailable) {
		t.Fatalf("expected ErrImportAIUnavailable, got %v", err)
	}
}

func TestImportServiceImportRequiresMappingInAIMode(t *testing.T) {
	imports, _, _ := newImportServiceForTest(t, &stubImportLLM{})

	_, err := imports.Import(context.Background(), uuid.New(), januariTable(), []string{"JANUARI"},
		service.ImportOptions{Filename: "budgeting 2026.xlsx", Sheet: "JANUARI", Mode: service.ImportModeAI})
	if !errors.Is(err, service.ErrInvalidMapping) {
		t.Fatalf("expected ErrInvalidMapping, got %v", err)
	}
}

func TestImportServiceCommitDoesNotCallAI(t *testing.T) {
	plan := januariMapping()
	client := &stubImportLLM{}
	imports, _, _ := newImportServiceForTest(t, client)

	result, err := imports.Import(context.Background(), uuid.New(), januariTable(), []string{"JANUARI"},
		service.ImportOptions{Filename: "budgeting 2026.xlsx", Sheet: "JANUARI", Mode: service.ImportModeAI, Mapping: plan})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if client.calls != 0 {
		t.Fatalf("commit must not call the AI, got %d calls", client.calls)
	}
	if result.Mode != "ai" || result.Valid != 2 || result.Skipped != 3 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestImportServiceRejectsSheetMismatch(t *testing.T) {
	plan := januariMapping()
	plan.Sheet = "FEBRUARI"
	imports, _, _ := newImportServiceForTest(t, &stubImportLLM{})

	_, err := imports.Import(context.Background(), uuid.New(), januariTable(), []string{"JANUARI"},
		service.ImportOptions{Filename: "x.xlsx", Sheet: "JANUARI", Mode: service.ImportModeAI, Mapping: plan})
	if !errors.Is(err, service.ErrInvalidMapping) {
		t.Fatalf("expected ErrInvalidMapping, got %v", err)
	}
}

func TestImportServiceCommitRejectsUnknownAccountConstant(t *testing.T) {
	plan := januariMapping()
	plan.Fields.Account = model.ImportFieldRule{Constant: "Missing account"}
	imports, _, _ := newImportServiceForTest(t, &stubImportLLM{})

	_, err := imports.Import(context.Background(), uuid.New(), januariTable(), []string{"JANUARI"},
		service.ImportOptions{Filename: "x.xlsx", Sheet: "JANUARI", Mode: service.ImportModeAI, Mapping: plan})
	if !errors.Is(err, service.ErrInvalidMapping) {
		t.Fatalf("expected ErrInvalidMapping, got %v", err)
	}
}

func TestImportServiceCommitRejectsWrongTypeCategoryConstant(t *testing.T) {
	plan := januariMapping()
	plan.Fields.Category = model.ImportFieldRule{Constant: "Saving"}
	plan.Type = model.ImportTypeRule{Constant: "income"}
	imports, _, _ := newImportServiceForTest(t, &stubImportLLM{})

	_, err := imports.Import(context.Background(), uuid.New(), januariTable(), []string{"JANUARI"},
		service.ImportOptions{Filename: "x.xlsx", Sheet: "JANUARI", Mode: service.ImportModeAI, Mapping: plan})
	if !errors.Is(err, service.ErrInvalidMapping) {
		t.Fatalf("expected ErrInvalidMapping, got %v", err)
	}
}

func TestImportServiceReturnsUnavailableOnTransportError(t *testing.T) {
	imports, _, _ := newImportServiceForTest(t, &stubImportLLM{err: errors.New("dial timeout")})

	_, err := imports.Preview(context.Background(), uuid.New(), januariTable(), []string{"JANUARI"},
		service.ImportOptions{Filename: "x.xlsx", Sheet: "JANUARI", Mode: service.ImportModeAI})
	if !errors.Is(err, service.ErrImportAIUnavailable) {
		t.Fatalf("expected ErrImportAIUnavailable, got %v", err)
	}
	if !strings.Contains(err.Error(), "dial timeout") {
		t.Fatalf("expected the cause to be preserved, got %v", err)
	}
}

func TestImportServiceReturnsResolvedDataRange(t *testing.T) {
	plan := januariMapping()
	plan.DataStart = 0
	plan.DataEnd = 0
	imports, _, _ := newImportServiceForTest(t, &stubImportLLM{})

	result, err := imports.Import(context.Background(), uuid.New(), januariTable(), []string{"JANUARI"},
		service.ImportOptions{Filename: "x.xlsx", Sheet: "JANUARI", Mode: service.ImportModeAI, Mapping: plan})
	if err != nil {
		t.Fatalf("Import: %v", err)
	}
	if result.Mapping == nil || result.Mapping.DataStart != 8 || result.Mapping.DataEnd != 12 {
		t.Fatalf("expected resolved data range 8-12, got %+v", result.Mapping)
	}
}

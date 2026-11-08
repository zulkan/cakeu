package service_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"goskeleton/internal/model"
	"goskeleton/internal/service"
)

func TestPreviewImportWrapsMissingColumns(t *testing.T) {
	transactions := service.NewTransactionService(&fakeTransactionRepo{}, &fakeAccountRepo{}, &fakeCategoryRepo{})

	_, err := transactions.PreviewImport(context.Background(), uuid.New(), strings.NewReader("pocket,total\nSaving,100\n"))

	if !errors.Is(err, service.ErrImportMissingColumns) {
		t.Fatalf("expected ErrImportMissingColumns, got %v", err)
	}
	if !strings.Contains(err.Error(), `missing column "date"`) {
		t.Fatalf("expected the column name in the message, got %q", err.Error())
	}
}

func newImportTestTransactions(userID uuid.UUID) service.TransactionService {
	return service.NewTransactionService(
		&fakeTransactionRepo{},
		&fakeAccountRepo{accounts: []model.Account{{ID: uuid.New(), UserID: userID, Name: "Cash"}}},
		&fakeCategoryRepo{categories: []model.Category{{ID: uuid.New(), UserID: userID, Name: "Saving", Type: "expense"}}},
	)
}

func TestPreviewImportCarriesForwardEmptyDates(t *testing.T) {
	userID := uuid.New()
	transactions := newImportTestTransactions(userID)

	csvText := strings.Join([]string{
		"date,type,amount,description,account,category",
		"2026-01-14,expense,1500000,main pocket,Cash,Saving",
		",expense,500000,main pocket,Cash,Saving",
		"2026-01-16,expense,25000,laundry,Cash,Saving",
		",expense,100000,air,Cash,Saving",
	}, "\n") + "\n"

	result, err := transactions.PreviewImport(context.Background(), userID, strings.NewReader(csvText))
	if err != nil {
		t.Fatalf("PreviewImport: %v", err)
	}
	if result.Valid != 4 || result.Invalid != 0 || len(result.Errors) != 0 {
		t.Fatalf("expected 4 valid rows and no errors, got %+v", result)
	}
	wantDates := []string{"2026-01-14", "2026-01-14", "2026-01-16", "2026-01-16"}
	for i, want := range wantDates {
		if got := result.Preview[i].Date; got != want {
			t.Fatalf("preview row %d: expected date %s, got %s", i, want, got)
		}
	}
}

func TestPreviewImportReportsEmptyDateWithoutPrevious(t *testing.T) {
	userID := uuid.New()
	transactions := newImportTestTransactions(userID)

	csvText := strings.Join([]string{
		"date,type,amount,description,account,category",
		",expense,10000,orphan,Cash,Saving",
		"2026-01-14,expense,1500000,main pocket,Cash,Saving",
	}, "\n") + "\n"

	result, err := transactions.PreviewImport(context.Background(), userID, strings.NewReader(csvText))
	if err != nil {
		t.Fatalf("PreviewImport: %v", err)
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

func TestPreviewImportKeepsCarryingDatesAndReportsAllErrors(t *testing.T) {
	userID := uuid.New()
	transactions := newImportTestTransactions(userID)

	csvText := strings.Join([]string{
		"date,type,amount,description,account,category",
		"2026-01-14,expense,1500000,main pocket,Cash,Saving",
		",expense,not-a-number,bad amount,Cash,Saving",
		",expense,500000,carried date,Cash,Saving",
		",expense,25000,unknown account,Missing,Saving",
		",expense,100000,still carried,Cash,Saving",
	}, "\n") + "\n"

	result, err := transactions.PreviewImport(context.Background(), userID, strings.NewReader(csvText))
	if err != nil {
		t.Fatalf("PreviewImport: %v", err)
	}
	if result.Valid != 3 || result.Invalid != 2 || len(result.Errors) != 2 {
		t.Fatalf("expected 3 valid and 2 invalid rows, got %+v", result)
	}
	if result.Errors[0].Row != 3 || result.Errors[1].Row != 5 {
		t.Fatalf("unexpected error rows: %+v", result.Errors)
	}
	for _, row := range result.Preview {
		if row.Date != "2026-01-14" {
			t.Fatalf("expected carried date on row %d, got %s", row.Row, row.Date)
		}
	}
}

func TestPreviewImportRejectsNonFiniteAndAmbiguousAmounts(t *testing.T) {
	userID := uuid.New()
	transactions := service.NewTransactionService(
		&fakeTransactionRepo{},
		&fakeAccountRepo{accounts: []model.Account{{ID: uuid.New(), Name: "Cash"}}},
		&fakeCategoryRepo{categories: []model.Category{{ID: uuid.New(), Name: "Saving", Type: "expense"}}},
	)

	csvText := strings.Join([]string{
		"date,type,amount,description,account,category",
		"2026-01-01,expense,NaN,not a number,Cash,Saving",
		"2026-01-02,expense,+Inf,infinity,Cash,Saving",
		"2026-01-03,expense,42.5,valid,Cash,Saving",
		"",
	}, "\n")

	result, err := transactions.PreviewImport(context.Background(), userID, strings.NewReader(csvText))
	if err != nil {
		t.Fatalf("PreviewImport: %v", err)
	}
	if result.Valid != 1 || result.Invalid != 2 {
		t.Fatalf("expected 1 valid and 2 invalid rows, got %+v", result)
	}
}

func newImportTestTransactionsWithRepo(userID uuid.UUID) (service.TransactionService, *fakeTransactionRepo) {
	repo := &fakeTransactionRepo{}
	transactions := service.NewTransactionService(
		repo,
		&fakeAccountRepo{accounts: []model.Account{
			{ID: uuid.New(), UserID: userID, Name: "Cash"},
			{ID: uuid.New(), UserID: userID, Name: "Savings"},
		}},
		&fakeCategoryRepo{categories: []model.Category{{ID: uuid.New(), UserID: userID, Name: "Saving", Type: "expense"}}},
	)
	return transactions, repo
}

func existingFingerprint(date string, amount float64, description, account string) model.TransactionFingerprint {
	parsed, err := time.Parse("2006-01-02", date)
	if err != nil {
		panic(err)
	}
	return model.TransactionFingerprint{
		Date:        parsed,
		Type:        "expense",
		Amount:      amount,
		Description: description,
		AccountName: account,
	}
}

func TestPreviewImportFlagsExistingDuplicate(t *testing.T) {
	userID := uuid.New()
	transactions, repo := newImportTestTransactionsWithRepo(userID)
	repo.fingerprints = []model.TransactionFingerprint{existingFingerprint("2026-01-14", 1500000, "main pocket", "Cash")}

	csvText := strings.Join([]string{
		"date,type,amount,description,account,category",
		"2026-01-14,expense,1500000,main pocket,Cash,Saving",
	}, "\n") + "\n"

	result, err := transactions.PreviewImport(context.Background(), userID, strings.NewReader(csvText))
	if err != nil {
		t.Fatalf("PreviewImport: %v", err)
	}
	if result.Valid != 1 || result.DuplicateCount != 1 || len(result.Duplicates) != 1 {
		t.Fatalf("expected 1 valid and 1 duplicate, got %+v", result)
	}
	if result.Duplicates[0].Row != 2 || result.Duplicates[0].Message != "duplicate of an existing transaction" {
		t.Fatalf("unexpected duplicate report: %+v", result.Duplicates[0])
	}
	if !result.Preview[0].Duplicate {
		t.Fatalf("expected the preview row to be flagged as a duplicate, got %+v", result.Preview[0])
	}
	if !repo.fingerprintFrom.Equal(time.Date(2026, 1, 14, 0, 0, 0, 0, time.UTC)) || !repo.fingerprintTo.Equal(repo.fingerprintFrom) {
		t.Fatalf("expected the fingerprint range to cover the candidate date, got %s..%s", repo.fingerprintFrom, repo.fingerprintTo)
	}
}

func TestPreviewImportKeepsFirstIdenticalRowInFile(t *testing.T) {
	userID := uuid.New()
	transactions, _ := newImportTestTransactionsWithRepo(userID)

	csvText := strings.Join([]string{
		"date,type,amount,description,account,category",
		"2026-01-14,expense,1500000,main pocket,Cash,Saving",
		"2026-01-14,expense,1500000,main pocket,Cash,Saving",
	}, "\n") + "\n"

	result, err := transactions.PreviewImport(context.Background(), userID, strings.NewReader(csvText))
	if err != nil {
		t.Fatalf("PreviewImport: %v", err)
	}
	if result.Valid != 2 || result.DuplicateCount != 1 || len(result.Duplicates) != 1 {
		t.Fatalf("expected 2 valid rows and 1 duplicate, got %+v", result)
	}
	if result.Duplicates[0].Row != 3 || result.Duplicates[0].Message != "duplicate row in file" {
		t.Fatalf("unexpected duplicate report: %+v", result.Duplicates[0])
	}
	if result.Preview[0].Duplicate {
		t.Fatalf("expected the first identical row to stay clean, got %+v", result.Preview[0])
	}
	if !result.Preview[1].Duplicate {
		t.Fatalf("expected the second identical row to be flagged, got %+v", result.Preview[1])
	}
}

func TestImportTransactionsSkipsExistingDuplicatesByDefault(t *testing.T) {
	userID := uuid.New()
	transactions, repo := newImportTestTransactionsWithRepo(userID)
	repo.fingerprints = []model.TransactionFingerprint{existingFingerprint("2026-01-14", 1500000, "main pocket", "Cash")}

	csvText := strings.Join([]string{
		"date,type,amount,description,account,category",
		"2026-01-14,expense,1500000,main pocket,Cash,Saving",
		"2026-01-15,expense,25000,laundry,Cash,Saving",
	}, "\n") + "\n"

	result, err := transactions.ImportTransactions(context.Background(), userID, strings.NewReader(csvText), false)
	if err != nil {
		t.Fatalf("ImportTransactions: %v", err)
	}
	if result.Valid != 2 || result.DuplicateCount != 1 {
		t.Fatalf("expected 2 valid rows and 1 duplicate, got %+v", result)
	}
	if result.Imported != 1 || len(repo.lastBatch) != 1 {
		t.Fatalf("expected only the new row to import, got imported=%d batch=%d", result.Imported, len(repo.lastBatch))
	}
	if repo.lastBatch[0].Description != "laundry" {
		t.Fatalf("expected the non-duplicate row in the batch, got %+v", repo.lastBatch[0])
	}
}

func TestImportTransactionsAllowDuplicatesImportsExistingDuplicate(t *testing.T) {
	userID := uuid.New()
	transactions, repo := newImportTestTransactionsWithRepo(userID)
	repo.fingerprints = []model.TransactionFingerprint{existingFingerprint("2026-01-14", 1500000, "main pocket", "Cash")}

	csvText := strings.Join([]string{
		"date,type,amount,description,account,category",
		"2026-01-14,expense,1500000,main pocket,Cash,Saving",
	}, "\n") + "\n"

	result, err := transactions.ImportTransactions(context.Background(), userID, strings.NewReader(csvText), true)
	if err != nil {
		t.Fatalf("ImportTransactions: %v", err)
	}
	if result.Imported != 1 || len(repo.lastBatch) != 1 {
		t.Fatalf("expected the duplicate to import, got imported=%d batch=%d", result.Imported, len(repo.lastBatch))
	}
}

func TestImportTransactionsAlwaysSkipsInFileDuplicates(t *testing.T) {
	userID := uuid.New()
	transactions, repo := newImportTestTransactionsWithRepo(userID)

	csvText := strings.Join([]string{
		"date,type,amount,description,account,category",
		"2026-01-14,expense,1500000,main pocket,Cash,Saving",
		"2026-01-14,expense,1500000,main pocket,Cash,Saving",
	}, "\n") + "\n"

	result, err := transactions.ImportTransactions(context.Background(), userID, strings.NewReader(csvText), true)
	if err != nil {
		t.Fatalf("ImportTransactions: %v", err)
	}
	if result.Imported != 1 || len(repo.lastBatch) != 1 {
		t.Fatalf("expected allow_duplicates to keep only the first in-file row, got imported=%d batch=%d", result.Imported, len(repo.lastBatch))
	}
}

func TestImportTransactionsDoesNotFlagDistinctRows(t *testing.T) {
	userID := uuid.New()
	transactions, repo := newImportTestTransactionsWithRepo(userID)
	repo.fingerprints = []model.TransactionFingerprint{existingFingerprint("2026-01-14", 100, "coffee", "Cash")}

	csvText := strings.Join([]string{
		"date,type,amount,description,account,category",
		"2026-01-15,expense,100,coffee,Cash,Saving",
		"2026-01-14,expense,101,coffee,Cash,Saving",
		"2026-01-14,expense,100,tea,Cash,Saving",
		"2026-01-14,expense,100,coffee,Savings,Saving",
	}, "\n") + "\n"

	result, err := transactions.ImportTransactions(context.Background(), userID, strings.NewReader(csvText), false)
	if err != nil {
		t.Fatalf("ImportTransactions: %v", err)
	}
	if result.DuplicateCount != 0 || len(result.Duplicates) != 0 {
		t.Fatalf("expected no duplicates, got %+v", result.Duplicates)
	}
	if result.Imported != 4 || len(repo.lastBatch) != 4 {
		t.Fatalf("expected all distinct rows to import, got imported=%d batch=%d", result.Imported, len(repo.lastBatch))
	}
}

func TestPreviewImportSerializesEmptyDuplicatesAsArray(t *testing.T) {
	userID := uuid.New()
	transactions, _ := newImportTestTransactionsWithRepo(userID)

	csvText := "date,type,amount,description,account,category\n2026-01-14,expense,1500000,main pocket,Cash,Saving\n"
	result, err := transactions.PreviewImport(context.Background(), userID, strings.NewReader(csvText))
	if err != nil {
		t.Fatalf("PreviewImport: %v", err)
	}
	if result.Duplicates == nil || len(result.Duplicates) != 0 {
		t.Fatalf("expected an initialized empty duplicate slice, got %#v", result.Duplicates)
	}

	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if !strings.Contains(string(encoded), `"duplicates":[]`) {
		t.Fatalf("expected duplicates to serialize as [], got %s", encoded)
	}
}

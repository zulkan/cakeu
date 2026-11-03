package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"

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

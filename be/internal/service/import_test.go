package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

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

package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"goskeleton/internal/model"
	"goskeleton/internal/service"
)

type fakeBudgetRepo struct {
	budgets []model.Budget
	spent   map[uuid.UUID]float64
	deleted int
}

func (f *fakeBudgetRepo) Upsert(ctx context.Context, userID, categoryID uuid.UUID, month time.Time, amount float64) (*model.Budget, error) {
	return &model.Budget{ID: uuid.New(), UserID: userID, CategoryID: categoryID, Month: month, Amount: amount}, nil
}

func (f *fakeBudgetRepo) Delete(ctx context.Context, userID, categoryID uuid.UUID, month time.Time) error {
	f.deleted++
	return nil
}

func (f *fakeBudgetRepo) ListByMonth(ctx context.Context, userID uuid.UUID, month time.Time) ([]model.Budget, error) {
	return f.budgets, nil
}

func (f *fakeBudgetRepo) SpentByCategory(ctx context.Context, userID uuid.UUID, month time.Time) (map[uuid.UUID]float64, error) {
	return f.spent, nil
}

func TestBudgetServiceListComputesSpentAndPercent(t *testing.T) {
	t.Parallel()

	categoryID := uuid.New()
	repo := &fakeBudgetRepo{
		budgets: []model.Budget{{CategoryID: categoryID, CategoryName: "Food", Amount: 200}},
		spent:   map[uuid.UUID]float64{categoryID: 50},
	}
	budgets := service.NewBudgetService(repo)

	statuses, err := budgets.List(context.Background(), uuid.New(), time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("expected 1 status, got %d", len(statuses))
	}

	status := statuses[0]
	if status.Spent != 50 || status.Remaining != 150 || status.Percent != 25 {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestBudgetServiceUpsertRejectsNonPositiveAmount(t *testing.T) {
	t.Parallel()

	budgets := service.NewBudgetService(&fakeBudgetRepo{})

	if _, err := budgets.Upsert(context.Background(), uuid.New(), uuid.New(), time.Now(), 0); !errors.Is(err, service.ErrInvalidBudgetAmount) {
		t.Fatalf("expected ErrInvalidBudgetAmount, got %v", err)
	}
}

func TestBudgetServiceRejectsZeroMonth(t *testing.T) {
	t.Parallel()

	budgets := service.NewBudgetService(&fakeBudgetRepo{})

	if _, err := budgets.List(context.Background(), uuid.New(), time.Time{}); !errors.Is(err, service.ErrInvalidMonth) {
		t.Fatalf("expected ErrInvalidMonth, got %v", err)
	}
}

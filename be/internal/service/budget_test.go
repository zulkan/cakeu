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
	budgets         []model.Budget
	spent           map[uuid.UUID]float64
	spentByCurrency map[string]map[uuid.UUID]float64
	spentCurrencies []string
	deleted         int
	deletedCurrency string
}

func (f *fakeBudgetRepo) Upsert(ctx context.Context, userID, categoryID uuid.UUID, month time.Time, currency string, amount float64) (*model.Budget, error) {
	return &model.Budget{ID: uuid.New(), UserID: userID, CategoryID: categoryID, Month: month, Currency: currency, Amount: amount}, nil
}

func (f *fakeBudgetRepo) Delete(ctx context.Context, userID, categoryID uuid.UUID, month time.Time, currency string) error {
	f.deleted++
	f.deletedCurrency = currency
	return nil
}

func (f *fakeBudgetRepo) ListByMonth(ctx context.Context, userID uuid.UUID, month time.Time) ([]model.Budget, error) {
	return f.budgets, nil
}

func (f *fakeBudgetRepo) SpentByCategory(ctx context.Context, userID uuid.UUID, month time.Time, currency string) (map[uuid.UUID]float64, error) {
	f.spentCurrencies = append(f.spentCurrencies, currency)
	if f.spentByCurrency != nil {
		return f.spentByCurrency[currency], nil
	}
	return f.spent, nil
}

func TestBudgetServiceListComputesSpentAndPercent(t *testing.T) {
	t.Parallel()

	categoryID := uuid.New()
	repo := &fakeBudgetRepo{
		budgets: []model.Budget{{CategoryID: categoryID, CategoryName: "Food", Currency: "USD", Amount: 200}},
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
	if status.Currency != "USD" {
		t.Fatalf("expected currency USD, got %q", status.Currency)
	}
}

func TestBudgetServiceListUsesSpendingPerCurrency(t *testing.T) {
	t.Parallel()

	usdCategory := uuid.New()
	idrCategory := uuid.New()
	repo := &fakeBudgetRepo{
		budgets: []model.Budget{
			{CategoryID: usdCategory, CategoryName: "Food", Currency: "USD", Amount: 100},
			{CategoryID: idrCategory, CategoryName: "Food", Currency: "IDR", Amount: 1_000_000},
		},
		spentByCurrency: map[string]map[uuid.UUID]float64{
			"USD": {usdCategory: 25},
			"IDR": {idrCategory: 500_000},
		},
	}
	budgets := service.NewBudgetService(repo)

	statuses, err := budgets.List(context.Background(), uuid.New(), time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(statuses))
	}

	if statuses[0].Spent != 25 || statuses[0].Percent != 25 {
		t.Fatalf("unexpected USD status: %+v", statuses[0])
	}
	if statuses[1].Spent != 500_000 || statuses[1].Percent != 50 {
		t.Fatalf("unexpected IDR status: %+v", statuses[1])
	}
}

func TestBudgetServiceUpsertRejectsNonPositiveAmount(t *testing.T) {
	t.Parallel()

	budgets := service.NewBudgetService(&fakeBudgetRepo{})

	if _, err := budgets.Upsert(context.Background(), uuid.New(), uuid.New(), time.Now(), "USD", 0); !errors.Is(err, service.ErrInvalidBudgetAmount) {
		t.Fatalf("expected ErrInvalidBudgetAmount, got %v", err)
	}
}

func TestBudgetServiceUpsertRequiresCurrency(t *testing.T) {
	t.Parallel()

	budgets := service.NewBudgetService(&fakeBudgetRepo{})

	if _, err := budgets.Upsert(context.Background(), uuid.New(), uuid.New(), time.Now(), "", 100); !errors.Is(err, service.ErrInvalidCurrency) {
		t.Fatalf("expected ErrInvalidCurrency, got %v", err)
	}
	if _, err := budgets.Upsert(context.Background(), uuid.New(), uuid.New(), time.Now(), "US", 100); !errors.Is(err, service.ErrInvalidCurrency) {
		t.Fatalf("expected ErrInvalidCurrency for short code, got %v", err)
	}
}

func TestBudgetServiceNormalisesCurrency(t *testing.T) {
	t.Parallel()

	repo := &fakeBudgetRepo{}
	budgets := service.NewBudgetService(repo)

	budget, err := budgets.Upsert(context.Background(), uuid.New(), uuid.New(), time.Now(), " usd ", 100)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if budget.Currency != "USD" {
		t.Fatalf("expected normalised currency USD, got %q", budget.Currency)
	}
}

func TestBudgetServiceDeleteRequiresCurrency(t *testing.T) {
	t.Parallel()

	repo := &fakeBudgetRepo{}
	budgets := service.NewBudgetService(repo)

	if err := budgets.Delete(context.Background(), uuid.New(), uuid.New(), time.Now(), ""); !errors.Is(err, service.ErrInvalidCurrency) {
		t.Fatalf("expected ErrInvalidCurrency, got %v", err)
	}
	if repo.deleted != 0 {
		t.Fatalf("expected no delete for invalid currency, got %d", repo.deleted)
	}

	if err := budgets.Delete(context.Background(), uuid.New(), uuid.New(), time.Now(), " idr "); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if repo.deletedCurrency != "IDR" {
		t.Fatalf("expected normalised IDR, got %q", repo.deletedCurrency)
	}
}

func TestBudgetServiceRejectsZeroMonth(t *testing.T) {
	t.Parallel()

	budgets := service.NewBudgetService(&fakeBudgetRepo{})

	if _, err := budgets.List(context.Background(), uuid.New(), time.Time{}); !errors.Is(err, service.ErrInvalidMonth) {
		t.Fatalf("expected ErrInvalidMonth, got %v", err)
	}
}

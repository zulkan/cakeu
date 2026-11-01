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

type fakeReportRepo struct {
	cashflow   []model.CashflowPoint
	categories []model.CategoryTotal
	merchants  []model.MerchantTotal
	lastLimit  int
}

func (f *fakeReportRepo) Cashflow(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]model.CashflowPoint, error) {
	return f.cashflow, nil
}

func (f *fakeReportRepo) CategoryTotals(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]model.CategoryTotal, error) {
	return f.categories, nil
}

func (f *fakeReportRepo) MerchantTotals(ctx context.Context, userID uuid.UUID, from, to time.Time, limit int) ([]model.MerchantTotal, error) {
	f.lastLimit = limit
	return f.merchants, nil
}

func TestReportServiceNormalisesMonthRange(t *testing.T) {
	t.Parallel()

	from, to, err := service.NormaliseRange(
		time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 12, 3, 0, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !from.Equal(time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected start %s", from)
	}
	if !to.Equal(time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected exclusive end at next month, got %s", to)
	}
}

func TestReportServiceRejectsInvalidRanges(t *testing.T) {
	t.Parallel()

	inverted := func() error {
		_, _, err := service.NormaliseRange(
			time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		)
		return err
	}
	if !errors.Is(inverted(), service.ErrInvalidRange) {
		t.Fatal("expected inverted range to be rejected")
	}

	tooLong := func() error {
		_, _, err := service.NormaliseRange(
			time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
			time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		)
		return err
	}
	if !errors.Is(tooLong(), service.ErrInvalidRange) {
		t.Fatal("expected ranges over 24 months to be rejected")
	}
}

func TestReportServiceComputesCategoryShare(t *testing.T) {
	t.Parallel()

	repo := &fakeReportRepo{categories: []model.CategoryTotal{
		{CategoryName: "Food", Type: "expense", Total: 75},
		{CategoryName: "Rent", Type: "expense", Total: 25},
	}}
	reports := service.NewReportService(repo)

	totals, err := reports.Categories(context.Background(), uuid.New(),
		time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if totals[0].Share != 75 || totals[1].Share != 25 {
		t.Fatalf("unexpected shares: %+v", totals)
	}
}

func TestReportServiceComputesNetAndClampsMerchantLimit(t *testing.T) {
	t.Parallel()

	repo := &fakeReportRepo{cashflow: []model.CashflowPoint{{Period: "2026-10", Income: 100, Expense: 40}}}
	reports := service.NewReportService(repo)

	points, err := reports.Cashflow(context.Background(), uuid.New(),
		time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if points[0].Net != 60 {
		t.Fatalf("expected net 60, got %v", points[0].Net)
	}

	if _, err := reports.Merchants(context.Background(), uuid.New(),
		time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC), 500,
	); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if repo.lastLimit != 20 {
		t.Fatalf("expected limit clamped to 20, got %d", repo.lastLimit)
	}
}

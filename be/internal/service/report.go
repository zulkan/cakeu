package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"goskeleton/internal/model"
)

var ErrInvalidRange = errors.New("invalid report range")

const maxReportMonths = 24

type ReportRepository interface {
	Cashflow(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]model.CashflowPoint, error)
	CategoryTotals(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]model.CategoryTotal, error)
	MerchantTotals(ctx context.Context, userID uuid.UUID, from, to time.Time, limit int) ([]model.MerchantTotal, error)
}

type ReportService interface {
	Cashflow(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]model.CashflowPoint, error)
	Categories(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]model.CategoryTotal, error)
	Merchants(ctx context.Context, userID uuid.UUID, from, to time.Time, limit int) ([]model.MerchantTotal, error)
}

type reportService struct {
	repo ReportRepository
}

func NewReportService(repo ReportRepository) *reportService {
	return &reportService{repo: repo}
}

func (s *reportService) Cashflow(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]model.CashflowPoint, error) {
	start, end, err := NormaliseRange(from, to)
	if err != nil {
		return nil, err
	}

	points, err := s.repo.Cashflow(ctx, userID, start, end)
	if err != nil {
		return nil, fmt.Errorf("cashflow report: %w", err)
	}

	for i := range points {
		points[i].Net = points[i].Income - points[i].Expense
	}

	return points, nil
}

func (s *reportService) Categories(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]model.CategoryTotal, error) {
	start, end, err := NormaliseRange(from, to)
	if err != nil {
		return nil, err
	}

	totals, err := s.repo.CategoryTotals(ctx, userID, start, end)
	if err != nil {
		return nil, fmt.Errorf("category report: %w", err)
	}

	// Share is computed within each (currency, type) group: a USD category is
	// never compared against an IDR category's total.
	type shareKey struct {
		currency string
		kind     string
	}
	byGroup := make(map[shareKey]float64, 2)
	for _, total := range totals {
		byGroup[shareKey{currency: total.Currency, kind: total.Type}] += total.Total
	}

	for i := range totals {
		key := shareKey{currency: totals[i].Currency, kind: totals[i].Type}
		if sum := byGroup[key]; sum > 0 {
			totals[i].Share = totals[i].Total / sum * 100
		}
	}

	return totals, nil
}

func (s *reportService) Merchants(ctx context.Context, userID uuid.UUID, from, to time.Time, limit int) ([]model.MerchantTotal, error) {
	start, end, err := NormaliseRange(from, to)
	if err != nil {
		return nil, err
	}

	if limit <= 0 {
		limit = 10
	}
	if limit > 20 {
		limit = 20
	}

	totals, err := s.repo.MerchantTotals(ctx, userID, start, end, limit)
	if err != nil {
		return nil, fmt.Errorf("merchant report: %w", err)
	}

	return totals, nil
}

// NormaliseRange expands [from, to] to whole UTC months and returns an
// exclusive end so database range filters include every day of the last month.
func NormaliseRange(from, to time.Time) (time.Time, time.Time, error) {
	if from.IsZero() || to.IsZero() {
		return time.Time{}, time.Time{}, ErrInvalidRange
	}

	start := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(to.Year(), to.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 1, 0)

	if !end.After(start) {
		return time.Time{}, time.Time{}, ErrInvalidRange
	}

	months := (end.Year()-start.Year())*12 + int(end.Month()-start.Month())
	if months > maxReportMonths {
		return time.Time{}, time.Time{}, ErrInvalidRange
	}

	return start, end, nil
}

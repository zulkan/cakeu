package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"goskeleton/internal/model"
)

var (
	ErrInvalidBudgetAmount = errors.New("budget amount must be greater than zero")
	ErrBudgetNotFound      = errors.New("budget not found")
	ErrInvalidMonth        = errors.New("invalid month")
)

type BudgetRepository interface {
	Upsert(ctx context.Context, userID, categoryID uuid.UUID, month time.Time, amount float64) (*model.Budget, error)
	Delete(ctx context.Context, userID, categoryID uuid.UUID, month time.Time) error
	ListByMonth(ctx context.Context, userID uuid.UUID, month time.Time) ([]model.Budget, error)
	SpentByCategory(ctx context.Context, userID uuid.UUID, month time.Time) (map[uuid.UUID]float64, error)
}

type BudgetService interface {
	List(ctx context.Context, userID uuid.UUID, month time.Time) ([]model.BudgetStatus, error)
	Upsert(ctx context.Context, userID, categoryID uuid.UUID, month time.Time, amount float64) (*model.Budget, error)
	Delete(ctx context.Context, userID, categoryID uuid.UUID, month time.Time) error
}

type budgetService struct {
	repo BudgetRepository
}

func NewBudgetService(repo BudgetRepository) *budgetService {
	return &budgetService{repo: repo}
}

func (s *budgetService) List(ctx context.Context, userID uuid.UUID, month time.Time) ([]model.BudgetStatus, error) {
	normalised, err := NormaliseMonth(month)
	if err != nil {
		return nil, err
	}

	budgets, err := s.repo.ListByMonth(ctx, userID, normalised)
	if err != nil {
		return nil, fmt.Errorf("list budgets: %w", err)
	}

	spent, err := s.repo.SpentByCategory(ctx, userID, normalised)
	if err != nil {
		return nil, fmt.Errorf("sum spending: %w", err)
	}

	statuses := make([]model.BudgetStatus, 0, len(budgets))
	for _, budget := range budgets {
		spentAmount := spent[budget.CategoryID]
		remaining := budget.Amount - spentAmount

		percent := 0.0
		if budget.Amount > 0 {
			percent = spentAmount / budget.Amount * 100
		}

		statuses = append(statuses, model.BudgetStatus{
			CategoryID:   budget.CategoryID,
			CategoryName: budget.CategoryName,
			Budget:       budget.Amount,
			Spent:        spentAmount,
			Remaining:    remaining,
			Percent:      percent,
		})
	}

	return statuses, nil
}

func (s *budgetService) Upsert(ctx context.Context, userID, categoryID uuid.UUID, month time.Time, amount float64) (*model.Budget, error) {
	if amount <= 0 {
		return nil, ErrInvalidBudgetAmount
	}

	normalised, err := NormaliseMonth(month)
	if err != nil {
		return nil, err
	}

	budget, err := s.repo.Upsert(ctx, userID, categoryID, normalised, amount)
	if err != nil {
		return nil, fmt.Errorf("upsert budget: %w", err)
	}

	return budget, nil
}

func (s *budgetService) Delete(ctx context.Context, userID, categoryID uuid.UUID, month time.Time) error {
	normalised, err := NormaliseMonth(month)
	if err != nil {
		return err
	}

	if err := s.repo.Delete(ctx, userID, categoryID, normalised); err != nil {
		return err
	}

	return nil
}

// NormaliseMonth collapses any instant to the first day of its UTC month.
func NormaliseMonth(month time.Time) (time.Time, error) {
	if month.IsZero() {
		return time.Time{}, ErrInvalidMonth
	}

	return time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, time.UTC), nil
}

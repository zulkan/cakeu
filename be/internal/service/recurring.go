package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"goskeleton/internal/model"
)

var (
	ErrRecurringNotFound      = errors.New("recurring rule not found")
	ErrInvalidFrequency       = errors.New("frequency must be daily, weekly or monthly")
	ErrInvalidRecurringAmount = errors.New("amount must be greater than zero")
	ErrInvalidInterval        = errors.New("interval must be between 1 and 12")
	ErrInvalidNextRun         = errors.New("invalid next run date")
)

const (
	maxCatchUpIterations = 366
	frequencyDaily       = "daily"
	frequencyWeekly      = "weekly"
	frequencyMonthly     = "monthly"
)

type RecurringRepository interface {
	Create(ctx context.Context, rule model.RecurringRule) (*model.RecurringRule, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]model.RecurringRule, error)
	GetByID(ctx context.Context, userID, id uuid.UUID) (*model.RecurringRule, error)
	Update(ctx context.Context, userID uuid.UUID, rule model.RecurringRule) (*model.RecurringRule, error)
	Delete(ctx context.Context, userID, id uuid.UUID) error
	ListDue(ctx context.Context, now time.Time, limit int) ([]model.RecurringRule, error)
	ClaimRun(ctx context.Context, ruleID uuid.UUID, periodStart time.Time) (bool, error)
	AttachRunTransaction(ctx context.Context, ruleID uuid.UUID, periodStart time.Time, transactionID uuid.UUID) error
	ReleaseRun(ctx context.Context, ruleID uuid.UUID, periodStart time.Time) error
	AdvanceNextRun(ctx context.Context, ruleID uuid.UUID, next time.Time) error
}

type RecurringService interface {
	Create(ctx context.Context, userID uuid.UUID, req model.CreateRecurringRequest) (*model.RecurringRule, error)
	List(ctx context.Context, userID uuid.UUID) ([]model.RecurringRule, error)
	Update(ctx context.Context, userID, id uuid.UUID, req model.UpdateRecurringRequest) (*model.RecurringRule, error)
	Delete(ctx context.Context, userID, id uuid.UUID) error
	RunDue(ctx context.Context, now time.Time) (int, error)
	RunNow(ctx context.Context, userID, id uuid.UUID) (int, error)
}

type recurringService struct {
	repo         RecurringRepository
	transactions TransactionService
}

func NewRecurringService(repo RecurringRepository, transactions TransactionService) *recurringService {
	return &recurringService{repo: repo, transactions: transactions}
}

func (s *recurringService) Create(ctx context.Context, userID uuid.UUID, req model.CreateRecurringRequest) (*model.RecurringRule, error) {
	if err := validateRecurring(req.Type, req.Amount, req.Frequency, req.Interval); err != nil {
		return nil, err
	}

	nextRun, err := parseNextRun(req.NextRun)
	if err != nil {
		return nil, err
	}

	rule := model.RecurringRule{
		UserID:      userID,
		AccountID:   req.AccountID,
		CategoryID:  req.CategoryID,
		Type:        req.Type,
		Amount:      req.Amount,
		Description: strings.TrimSpace(req.Description),
		Frequency:   req.Frequency,
		Interval:    req.Interval,
		NextRun:     nextRun,
		Active:      true,
	}

	created, err := s.repo.Create(ctx, rule)
	if err != nil {
		return nil, fmt.Errorf("create recurring rule: %w", err)
	}

	return created, nil
}

func (s *recurringService) List(ctx context.Context, userID uuid.UUID) ([]model.RecurringRule, error) {
	rules, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list recurring rules: %w", err)
	}

	return rules, nil
}

func (s *recurringService) Update(ctx context.Context, userID, id uuid.UUID, req model.UpdateRecurringRequest) (*model.RecurringRule, error) {
	rule, err := s.repo.GetByID(ctx, userID, id)
	if err != nil {
		return nil, err
	}

	if req.Amount != nil {
		if *req.Amount <= 0 {
			return nil, ErrInvalidRecurringAmount
		}
		rule.Amount = *req.Amount
	}
	if req.Description != nil {
		rule.Description = strings.TrimSpace(*req.Description)
	}
	if req.Frequency != nil {
		rule.Frequency = *req.Frequency
	}
	if req.Interval != nil {
		rule.Interval = *req.Interval
	}
	if req.Active != nil {
		rule.Active = *req.Active
	}
	if req.NextRun != nil {
		nextRun, parseErr := parseNextRun(*req.NextRun)
		if parseErr != nil {
			return nil, parseErr
		}
		rule.NextRun = nextRun
	}

	if err := validateRecurring(rule.Type, rule.Amount, rule.Frequency, rule.Interval); err != nil {
		return nil, err
	}

	updated, err := s.repo.Update(ctx, userID, *rule)
	if err != nil {
		return nil, fmt.Errorf("update recurring rule: %w", err)
	}

	return updated, nil
}

func (s *recurringService) Delete(ctx context.Context, userID, id uuid.UUID) error {
	return s.repo.Delete(ctx, userID, id)
}

func (s *recurringService) RunDue(ctx context.Context, now time.Time) (int, error) {
	rules, err := s.repo.ListDue(ctx, now, 200)
	if err != nil {
		return 0, fmt.Errorf("list due rules: %w", err)
	}

	total := 0
	var firstErr error
	for _, rule := range rules {
		posted, runErr := s.runRule(ctx, rule, now)
		total += posted
		if runErr != nil && firstErr == nil {
			firstErr = runErr
		}
	}

	return total, firstErr
}

func (s *recurringService) RunNow(ctx context.Context, userID, id uuid.UUID) (int, error) {
	rule, err := s.repo.GetByID(ctx, userID, id)
	if err != nil {
		return 0, err
	}

	return s.runRule(ctx, *rule, rule.NextRun)
}

func (s *recurringService) runRule(ctx context.Context, rule model.RecurringRule, now time.Time) (int, error) {
	posted := 0
	next := rule.NextRun

	for i := 0; i < maxCatchUpIterations && !next.After(now); i++ {
		period := next

		claimed, err := s.repo.ClaimRun(ctx, rule.ID, period)
		if err != nil {
			return posted, fmt.Errorf("claim recurring period: %w", err)
		}

		if claimed {
			transaction, createErr := s.transactions.CreateTransaction(ctx, rule.UserID, model.CreateTransactionRequest{
				AccountID:   rule.AccountID,
				CategoryID:  rule.CategoryID,
				Type:        rule.Type,
				Amount:      rule.Amount,
				Description: rule.Description,
				Date:        period,
			})
			if createErr != nil {
				_ = s.repo.ReleaseRun(ctx, rule.ID, period)
				return posted, fmt.Errorf("post recurring transaction: %w", createErr)
			}

			if err := s.repo.AttachRunTransaction(ctx, rule.ID, period, transaction.ID); err != nil {
				return posted, fmt.Errorf("record recurring run: %w", err)
			}
			posted++
		}

		next = NextRunDate(period, rule.Frequency, rule.Interval)
	}

	if err := s.repo.AdvanceNextRun(ctx, rule.ID, next); err != nil {
		return posted, fmt.Errorf("advance next run: %w", err)
	}

	return posted, nil
}

func validateRecurring(type_ string, amount float64, frequency string, interval int) error {
	if !isValidType(type_) {
		return ErrInvalidType
	}

	if amount <= 0 {
		return ErrInvalidRecurringAmount
	}

	switch frequency {
	case frequencyDaily, frequencyWeekly, frequencyMonthly:
	default:
		return ErrInvalidFrequency
	}

	if interval < 1 || interval > 12 {
		return ErrInvalidInterval
	}

	return nil
}

func parseNextRun(value string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		now := time.Now().UTC()
		return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC), nil
	}

	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, ErrInvalidNextRun
	}

	return parsed, nil
}

func NextRunDate(from time.Time, frequency string, interval int) time.Time {
	switch frequency {
	case frequencyDaily:
		return from.AddDate(0, 0, interval)
	case frequencyWeekly:
		return from.AddDate(0, 0, 7*interval)
	case frequencyMonthly:
		return AddMonthsClamped(from, interval)
	default:
		return from.AddDate(0, 0, interval)
	}
}

func AddMonthsClamped(from time.Time, months int) time.Time {
	year, month, day := from.Date()
	total := int(month) - 1 + months
	targetYear := year + total/12
	targetMonth := time.Month(total%12 + 1)
	lastDay := time.Date(targetYear, targetMonth+1, 0, 0, 0, 0, 0, time.UTC).Day()
	if day > lastDay {
		day = lastDay
	}

	return time.Date(targetYear, targetMonth, day, 0, 0, 0, 0, time.UTC)
}

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"goskeleton/internal/model"
)

func (s *ChatService) setBudget(ctx context.Context, userID uuid.UUID, rawArgs string) (string, string, *model.ChatPendingAction, error) {
	if s.budgets == nil {
		return "", "", nil, fmt.Errorf("budgets are unavailable")
	}

	var args struct {
		Category string  `json:"category"`
		Type     string  `json:"type"`
		Month    string  `json:"month"`
		Currency string  `json:"currency"`
		Amount   float64 `json:"amount"`
	}
	if err := decodeArgs(rawArgs, &args); err != nil {
		return "", "", nil, err
	}

	category, err := s.findCategory(ctx, userID, args.Category, strings.ToLower(args.Type))
	if err != nil {
		return "", "", nil, err
	}

	month, err := parseMonthArg(args.Month)
	if err != nil {
		return "", "", nil, err
	}

	budget, err := s.budgets.Upsert(ctx, userID, category.ID, month, args.Currency, args.Amount)
	if err != nil {
		return "", "", nil, err
	}

	encoded, err := json.Marshal(map[string]any{
		"category": budget.CategoryName,
		"month":    budget.Month.Format("2006-01"),
		"amount":   budget.Amount,
	})
	if err != nil {
		return "", "", nil, fmt.Errorf("encode budget: %w", err)
	}

	return string(encoded), fmt.Sprintf("Set the %s budget to %.2f for %s", budget.CategoryName, budget.Amount, budget.Month.Format("2006-01")), nil, nil
}

func (s *ChatService) listBudgets(ctx context.Context, userID uuid.UUID, rawArgs string) (string, string, *model.ChatPendingAction, error) {
	if s.budgets == nil {
		return "", "", nil, fmt.Errorf("budgets are unavailable")
	}

	var args struct {
		Month string `json:"month"`
	}
	if err := decodeArgs(rawArgs, &args); err != nil {
		return "", "", nil, err
	}

	month, err := parseMonthArg(args.Month)
	if err != nil {
		return "", "", nil, err
	}

	statuses, err := s.budgets.List(ctx, userID, month)
	if err != nil {
		return "", "", nil, err
	}

	encoded, err := json.Marshal(statuses)
	if err != nil {
		return "", "", nil, fmt.Errorf("encode budgets: %w", err)
	}

	return string(encoded), fmt.Sprintf("Read %d budgets for %s", len(statuses), month.Format("2006-01")), nil, nil
}

func (s *ChatService) createRecurring(ctx context.Context, userID uuid.UUID, rawArgs string) (string, string, *model.ChatPendingAction, error) {
	if s.recurring == nil {
		return "", "", nil, fmt.Errorf("recurring rules are unavailable")
	}

	var args struct {
		Account     string  `json:"account"`
		Category    string  `json:"category"`
		Type        string  `json:"type"`
		Amount      float64 `json:"amount"`
		Description string  `json:"description"`
		Frequency   string  `json:"frequency"`
		Interval    int     `json:"interval"`
		NextRun     string  `json:"next_run"`
	}
	if err := decodeArgs(rawArgs, &args); err != nil {
		return "", "", nil, err
	}

	account, err := s.findAccount(ctx, userID, args.Account)
	if err != nil {
		return "", "", nil, err
	}

	category, err := s.findCategory(ctx, userID, args.Category, strings.ToLower(args.Type))
	if err != nil {
		return "", "", nil, err
	}

	rule, err := s.recurring.Create(ctx, userID, model.CreateRecurringRequest{
		AccountID:   account.ID,
		CategoryID:  category.ID,
		Type:        strings.ToLower(args.Type),
		Amount:      args.Amount,
		Description: args.Description,
		Frequency:   args.Frequency,
		Interval:    args.Interval,
		NextRun:     args.NextRun,
	})
	if err != nil {
		return "", "", nil, err
	}

	encoded, err := json.Marshal(rule)
	if err != nil {
		return "", "", nil, fmt.Errorf("encode recurring rule: %w", err)
	}

	return string(encoded), fmt.Sprintf("Created a %s recurring %s of %.2f", rule.Frequency, rule.Type, rule.Amount), nil, nil
}

func (s *ChatService) listRecurring(ctx context.Context, userID uuid.UUID, rawArgs string) (string, string, *model.ChatPendingAction, error) {
	if s.recurring == nil {
		return "", "", nil, fmt.Errorf("recurring rules are unavailable")
	}

	if err := decodeArgs(rawArgs, &struct{}{}); err != nil {
		return "", "", nil, err
	}

	rules, err := s.recurring.List(ctx, userID)
	if err != nil {
		return "", "", nil, err
	}

	encoded, err := json.Marshal(rules)
	if err != nil {
		return "", "", nil, fmt.Errorf("encode recurring rules: %w", err)
	}

	return string(encoded), fmt.Sprintf("Listed %d recurring rules", len(rules)), nil, nil
}

func (s *ChatService) pauseRecurring(ctx context.Context, userID uuid.UUID, rawArgs string) (string, string, *model.ChatPendingAction, error) {
	if s.recurring == nil {
		return "", "", nil, fmt.Errorf("recurring rules are unavailable")
	}

	var args struct {
		ID     string `json:"id"`
		Active *bool  `json:"active"`
	}
	if err := decodeArgs(rawArgs, &args); err != nil {
		return "", "", nil, err
	}

	id, err := uuid.Parse(strings.TrimSpace(args.ID))
	if err != nil {
		return "", "", nil, fmt.Errorf("invalid recurring rule id")
	}

	active := false
	if args.Active != nil {
		active = *args.Active
	}

	rule, err := s.recurring.Update(ctx, userID, id, model.UpdateRecurringRequest{Active: &active})
	if err != nil {
		return "", "", nil, err
	}

	state := "paused"
	if rule.Active {
		state = "resumed"
	}

	return "", fmt.Sprintf("Recurring rule %s", state), nil, nil
}

func (s *ChatService) cashflowReport(ctx context.Context, userID uuid.UUID, rawArgs string) (string, string, *model.ChatPendingAction, error) {
	if s.reports == nil {
		return "", "", nil, fmt.Errorf("reports are unavailable")
	}

	from, to, err := parseReportRange(rawArgs)
	if err != nil {
		return "", "", nil, err
	}

	points, err := s.reports.Cashflow(ctx, userID, from, to)
	if err != nil {
		return "", "", nil, err
	}

	encoded, err := json.Marshal(points)
	if err != nil {
		return "", "", nil, fmt.Errorf("encode cashflow: %w", err)
	}

	return string(encoded), fmt.Sprintf("Read cashflow for %d months", len(points)), nil, nil
}

func (s *ChatService) categoryReport(ctx context.Context, userID uuid.UUID, rawArgs string) (string, string, *model.ChatPendingAction, error) {
	if s.reports == nil {
		return "", "", nil, fmt.Errorf("reports are unavailable")
	}

	from, to, err := parseReportRange(rawArgs)
	if err != nil {
		return "", "", nil, err
	}

	totals, err := s.reports.Categories(ctx, userID, from, to)
	if err != nil {
		return "", "", nil, err
	}

	encoded, err := json.Marshal(totals)
	if err != nil {
		return "", "", nil, fmt.Errorf("encode categories: %w", err)
	}

	return string(encoded), fmt.Sprintf("Read %d category totals", len(totals)), nil, nil
}

func parseMonthArg(value string) (time.Time, error) {
	if strings.TrimSpace(value) == "" {
		now := time.Now().UTC()
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC), nil
	}

	parsed, err := time.Parse("2006-01", value)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid month %q, expected YYYY-MM", value)
	}

	return parsed, nil
}

func parseReportRange(rawArgs string) (time.Time, time.Time, error) {
	var args struct {
		From string `json:"from"`
		To   string `json:"to"`
	}
	if err := decodeArgs(rawArgs, &args); err != nil {
		return time.Time{}, time.Time{}, err
	}

	from, err := time.Parse(chatDateLayout, args.From)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid from date %q", args.From)
	}

	to, err := time.Parse(chatDateLayout, args.To)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid to date %q", args.To)
	}

	return from, to, nil
}

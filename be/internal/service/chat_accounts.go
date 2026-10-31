package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"goskeleton/internal/model"
)

func (s *ChatService) findAccount(ctx context.Context, userID uuid.UUID, name string) (*model.Account, error) {
	accounts, err := s.accounts.ListAccounts(ctx, userID)
	if err != nil {
		return nil, err
	}

	wanted := strings.ToLower(strings.TrimSpace(name))
	for i := range accounts {
		if strings.ToLower(strings.TrimSpace(accounts[i].Name)) == wanted {
			return &accounts[i], nil
		}
	}

	return nil, fmt.Errorf("unknown account %q", name)
}

func (s *ChatService) findCategory(ctx context.Context, userID uuid.UUID, name, type_ string) (*model.Category, error) {
	categories, err := s.categories.ListCategories(ctx, userID)
	if err != nil {
		return nil, err
	}

	wanted := strings.ToLower(strings.TrimSpace(name))
	wantedType := strings.ToLower(strings.TrimSpace(type_))

	for i := range categories {
		if strings.ToLower(strings.TrimSpace(categories[i].Name)) != wanted {
			continue
		}
		if wantedType != "" && categories[i].Type != wantedType {
			continue
		}
		return &categories[i], nil
	}

	if wantedType == "" {
		return nil, fmt.Errorf("unknown category %q", name)
	}

	return nil, fmt.Errorf("unknown category %q for type %q", name, type_)
}

func (s *ChatService) createAccount(ctx context.Context, userID uuid.UUID, rawArgs string) (string, string, *model.ChatPendingAction, error) {
	var args struct {
		Name     string  `json:"name"`
		Balance  float64 `json:"balance"`
		Currency string  `json:"currency"`
	}
	if err := decodeArgs(rawArgs, &args); err != nil {
		return "", "", nil, err
	}

	account, err := s.accounts.CreateAccount(ctx, userID, model.CreateAccountRequest{
		Name:     args.Name,
		Balance:  args.Balance,
		Currency: args.Currency,
	})
	if err != nil {
		return "", "", nil, err
	}

	encoded, err := json.Marshal(map[string]any{
		"id":       account.ID.String(),
		"name":     account.Name,
		"balance":  account.Balance,
		"currency": account.Currency,
	})
	if err != nil {
		return "", "", nil, fmt.Errorf("encode account: %w", err)
	}

	return string(encoded), fmt.Sprintf("Created account %q", account.Name), nil, nil
}

func (s *ChatService) createCategory(ctx context.Context, userID uuid.UUID, rawArgs string) (string, string, *model.ChatPendingAction, error) {
	var args struct {
		Name  string `json:"name"`
		Type  string `json:"type"`
		Color string `json:"color"`
	}
	if err := decodeArgs(rawArgs, &args); err != nil {
		return "", "", nil, err
	}

	category, err := s.categories.CreateCategory(ctx, userID, model.CreateCategoryRequest{
		Name:  args.Name,
		Type:  args.Type,
		Color: args.Color,
	})
	if err != nil {
		return "", "", nil, err
	}

	encoded, err := json.Marshal(map[string]any{
		"id":    category.ID.String(),
		"name":  category.Name,
		"type":  category.Type,
		"color": category.Color,
	})
	if err != nil {
		return "", "", nil, fmt.Errorf("encode category: %w", err)
	}

	return string(encoded), fmt.Sprintf("Created category %q", category.Name), nil, nil
}

func (s *ChatService) updateAccount(ctx context.Context, userID uuid.UUID, rawArgs string) (string, string, *model.ChatPendingAction, error) {
	var args struct {
		Account  string   `json:"account"`
		Name     *string  `json:"name"`
		Balance  *float64 `json:"balance"`
		Currency *string  `json:"currency"`
	}
	if err := decodeArgs(rawArgs, &args); err != nil {
		return "", "", nil, err
	}

	target, err := s.findAccount(ctx, userID, args.Account)
	if err != nil {
		return "", "", nil, err
	}

	account, err := s.accounts.UpdateAccount(ctx, userID, target.ID, model.UpdateAccountRequest{
		Name:     args.Name,
		Balance:  args.Balance,
		Currency: args.Currency,
	})
	if err != nil {
		return "", "", nil, err
	}

	encoded, err := json.Marshal(map[string]any{
		"id":       account.ID.String(),
		"name":     account.Name,
		"balance":  account.Balance,
		"currency": account.Currency,
	})
	if err != nil {
		return "", "", nil, fmt.Errorf("encode account: %w", err)
	}

	return string(encoded), fmt.Sprintf("Updated account %q", account.Name), nil, nil
}

func (s *ChatService) requestDeleteAccount(ctx context.Context, userID uuid.UUID, rawArgs string) (string, string, *model.ChatPendingAction, error) {
	var args struct {
		Account string `json:"account"`
	}
	if err := decodeArgs(rawArgs, &args); err != nil {
		return "", "", nil, err
	}

	target, err := s.findAccount(ctx, userID, args.Account)
	if err != nil {
		return "", "", nil, err
	}

	summary := fmt.Sprintf("Delete account %q (balance %.2f %s)?", target.Name, target.Balance, target.Currency)
	pending := s.storePending(userID, pendingDeleteAccount, target.ID, summary)

	return "", summary, &pending, nil
}

func (s *ChatService) requestDeleteCategory(ctx context.Context, userID uuid.UUID, rawArgs string) (string, string, *model.ChatPendingAction, error) {
	var args struct {
		Category string `json:"category"`
		Type     string `json:"type"`
	}
	if err := decodeArgs(rawArgs, &args); err != nil {
		return "", "", nil, err
	}

	target, err := s.findCategory(ctx, userID, args.Category, args.Type)
	if err != nil {
		return "", "", nil, err
	}

	summary := fmt.Sprintf("Delete category %q (%s)?", target.Name, target.Type)
	pending := s.storePending(userID, pendingDeleteCategory, target.ID, summary)

	return "", summary, &pending, nil
}

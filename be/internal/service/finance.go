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
	ErrAccountNotFound     = errors.New("account not found")
	ErrCategoryNotFound    = errors.New("category not found")
	ErrTransactionNotFound = errors.New("transaction not found")
	ErrAccountNameExists   = errors.New("account name already exists")
	ErrCategoryNameExists  = errors.New("category name already exists")
	ErrInvalidAmount       = errors.New("amount must be positive")
	ErrInvalidType         = errors.New("type must be income or expense")
	ErrInvalidName         = errors.New("name is required")
	ErrInvalidCurrency     = errors.New("currency must be a 3-letter code")
)

const (
	// TypeIncome marks money flowing into an account.
	TypeIncome = "income"
	// TypeExpense marks money flowing out of an account.
	TypeExpense = "expense"

	// DefaultCurrency is used when a request omits a currency.
	DefaultCurrency = "USD"
	// DefaultCategoryColor is used when a request omits a category color.
	DefaultCategoryColor = "#FB923C"

	// DefaultPageSize is used when a listing request omits page_size.
	DefaultPageSize = 20
	// MaxPageSize caps the number of rows returned by a listing request.
	MaxPageSize = 100
)

type AccountRepository interface {
	Create(ctx context.Context, userID uuid.UUID, name string, balance float64, currency string) (*model.Account, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.Account, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]model.Account, error)
	Update(ctx context.Context, id uuid.UUID, name *string, balance *float64, currency *string) (*model.Account, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type CategoryRepository interface {
	Create(ctx context.Context, userID uuid.UUID, name string, type_ string, color string) (*model.Category, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.Category, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]model.Category, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type TransactionRepository interface {
	Create(ctx context.Context, tx model.Transaction) (*model.Transaction, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.Transaction, error)
	List(ctx context.Context, filters model.TransactionFilters) ([]model.Transaction, int, error)
	Delete(ctx context.Context, id uuid.UUID) error
	MonthlyTotals(ctx context.Context, userID uuid.UUID, month time.Time) (income float64, expense float64, err error)
}

type AccountService interface {
	CreateAccount(ctx context.Context, userID uuid.UUID, req model.CreateAccountRequest) (*model.Account, error)
	GetAccount(ctx context.Context, id uuid.UUID) (*model.Account, error)
	ListAccounts(ctx context.Context, userID uuid.UUID) ([]model.Account, error)
	UpdateAccount(ctx context.Context, id uuid.UUID, req model.UpdateAccountRequest) (*model.Account, error)
	DeleteAccount(ctx context.Context, id uuid.UUID) error
}

type CategoryService interface {
	CreateCategory(ctx context.Context, userID uuid.UUID, req model.CreateCategoryRequest) (*model.Category, error)
	GetCategory(ctx context.Context, id uuid.UUID) (*model.Category, error)
	ListCategories(ctx context.Context, userID uuid.UUID) ([]model.Category, error)
	DeleteCategory(ctx context.Context, id uuid.UUID) error
}

type TransactionService interface {
	CreateTransaction(ctx context.Context, userID uuid.UUID, req model.CreateTransactionRequest) (*model.Transaction, error)
	GetTransaction(ctx context.Context, id uuid.UUID) (*model.Transaction, error)
	ListTransactions(ctx context.Context, userID uuid.UUID, filters model.TransactionFilters) (*model.PaginatedTransactions, error)
	DeleteTransaction(ctx context.Context, id uuid.UUID) error
	GetDashboardSummary(ctx context.Context, userID uuid.UUID) (*model.DashboardSummary, error)
}

type accountService struct {
	repo AccountRepository
}

func NewAccountService(repo AccountRepository) AccountService {
	return &accountService{repo: repo}
}

func (s *accountService) CreateAccount(ctx context.Context, userID uuid.UUID, req model.CreateAccountRequest) (*model.Account, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, ErrInvalidName
	}

	currency := normalizeCurrency(req.Currency)
	if currency == "" {
		currency = DefaultCurrency
	}

	account, err := s.repo.Create(ctx, userID, name, req.Balance, currency)
	if err != nil {
		if errors.Is(err, ErrAccountNameExists) {
			return nil, ErrAccountNameExists
		}
		return nil, fmt.Errorf("create account: %w", err)
	}

	return account, nil
}

func (s *accountService) GetAccount(ctx context.Context, id uuid.UUID) (*model.Account, error) {
	account, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrAccountNotFound) {
			return nil, ErrAccountNotFound
		}
		return nil, fmt.Errorf("get account: %w", err)
	}

	return account, nil
}

func (s *accountService) ListAccounts(ctx context.Context, userID uuid.UUID) ([]model.Account, error) {
	accounts, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	if accounts == nil {
		accounts = []model.Account{}
	}

	return accounts, nil
}

func (s *accountService) UpdateAccount(ctx context.Context, id uuid.UUID, req model.UpdateAccountRequest) (*model.Account, error) {
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, ErrInvalidName
		}
		req.Name = &name
	}

	if req.Currency != nil {
		currency := normalizeCurrency(*req.Currency)
		if len(currency) != 3 {
			return nil, ErrInvalidCurrency
		}
		req.Currency = &currency
	}

	account, err := s.repo.Update(ctx, id, req.Name, req.Balance, req.Currency)
	if err != nil {
		if errors.Is(err, ErrAccountNotFound) {
			return nil, ErrAccountNotFound
		}
		if errors.Is(err, ErrAccountNameExists) {
			return nil, ErrAccountNameExists
		}
		return nil, fmt.Errorf("update account: %w", err)
	}

	return account, nil
}

func (s *accountService) DeleteAccount(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		if errors.Is(err, ErrAccountNotFound) {
			return ErrAccountNotFound
		}
		return fmt.Errorf("delete account: %w", err)
	}

	return nil
}

type categoryService struct {
	repo CategoryRepository
}

func NewCategoryService(repo CategoryRepository) CategoryService {
	return &categoryService{repo: repo}
}

func (s *categoryService) CreateCategory(ctx context.Context, userID uuid.UUID, req model.CreateCategoryRequest) (*model.Category, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, ErrInvalidName
	}

	categoryType := strings.ToLower(strings.TrimSpace(req.Type))
	if !isValidType(categoryType) {
		return nil, ErrInvalidType
	}

	color := strings.TrimSpace(req.Color)
	if color == "" {
		color = DefaultCategoryColor
	}

	category, err := s.repo.Create(ctx, userID, name, categoryType, color)
	if err != nil {
		if errors.Is(err, ErrCategoryNameExists) {
			return nil, ErrCategoryNameExists
		}
		return nil, fmt.Errorf("create category: %w", err)
	}

	return category, nil
}

func (s *categoryService) GetCategory(ctx context.Context, id uuid.UUID) (*model.Category, error) {
	category, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrCategoryNotFound) {
			return nil, ErrCategoryNotFound
		}
		return nil, fmt.Errorf("get category: %w", err)
	}

	return category, nil
}

func (s *categoryService) ListCategories(ctx context.Context, userID uuid.UUID) ([]model.Category, error) {
	categories, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	if categories == nil {
		categories = []model.Category{}
	}

	return categories, nil
}

func (s *categoryService) DeleteCategory(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		if errors.Is(err, ErrCategoryNotFound) {
			return ErrCategoryNotFound
		}
		return fmt.Errorf("delete category: %w", err)
	}

	return nil
}

type transactionService struct {
	repo       TransactionRepository
	accounts   AccountRepository
	categories CategoryRepository
}

func NewTransactionService(repo TransactionRepository, accounts AccountRepository, categories CategoryRepository) TransactionService {
	return &transactionService{
		repo:       repo,
		accounts:   accounts,
		categories: categories,
	}
}

func (s *transactionService) CreateTransaction(ctx context.Context, userID uuid.UUID, req model.CreateTransactionRequest) (*model.Transaction, error) {
	transactionType := strings.ToLower(strings.TrimSpace(req.Type))
	if !isValidType(transactionType) {
		return nil, ErrInvalidType
	}

	if req.Amount <= 0 {
		return nil, ErrInvalidAmount
	}

	account, err := s.accounts.GetByID(ctx, req.AccountID)
	if err != nil {
		if errors.Is(err, ErrAccountNotFound) {
			return nil, ErrAccountNotFound
		}
		return nil, fmt.Errorf("get account: %w", err)
	}
	if account.UserID != userID {
		return nil, ErrAccountNotFound
	}

	category, err := s.categories.GetByID(ctx, req.CategoryID)
	if err != nil {
		if errors.Is(err, ErrCategoryNotFound) {
			return nil, ErrCategoryNotFound
		}
		return nil, fmt.Errorf("get category: %w", err)
	}
	if category.UserID != userID {
		return nil, ErrCategoryNotFound
	}
	if category.Type != transactionType {
		return nil, ErrInvalidType
	}

	date := req.Date
	if date.IsZero() {
		date = time.Now().UTC()
	}

	transaction, err := s.repo.Create(ctx, model.Transaction{
		UserID:      userID,
		AccountID:   req.AccountID,
		CategoryID:  req.CategoryID,
		Type:        transactionType,
		Amount:      req.Amount,
		Description: strings.TrimSpace(req.Description),
		Date:        date,
	})
	if err != nil {
		return nil, fmt.Errorf("create transaction: %w", err)
	}

	return transaction, nil
}

func (s *transactionService) GetTransaction(ctx context.Context, id uuid.UUID) (*model.Transaction, error) {
	transaction, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrTransactionNotFound) {
			return nil, ErrTransactionNotFound
		}
		return nil, fmt.Errorf("get transaction: %w", err)
	}

	return transaction, nil
}

func (s *transactionService) ListTransactions(ctx context.Context, userID uuid.UUID, filters model.TransactionFilters) (*model.PaginatedTransactions, error) {
	filters.UserID = &userID

	if filters.Type != "" {
		filters.Type = strings.ToLower(strings.TrimSpace(filters.Type))
		if !isValidType(filters.Type) {
			return nil, ErrInvalidType
		}
	}

	if filters.Page < 1 {
		filters.Page = 1
	}
	if filters.PageSize < 1 {
		filters.PageSize = DefaultPageSize
	}
	if filters.PageSize > MaxPageSize {
		filters.PageSize = MaxPageSize
	}

	transactions, total, err := s.repo.List(ctx, filters)
	if err != nil {
		return nil, fmt.Errorf("list transactions: %w", err)
	}
	if transactions == nil {
		transactions = []model.Transaction{}
	}

	return &model.PaginatedTransactions{
		Data:     transactions,
		Total:    total,
		Page:     filters.Page,
		PageSize: filters.PageSize,
	}, nil
}

func (s *transactionService) DeleteTransaction(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		if errors.Is(err, ErrTransactionNotFound) {
			return ErrTransactionNotFound
		}
		return fmt.Errorf("delete transaction: %w", err)
	}

	return nil
}

func (s *transactionService) GetDashboardSummary(ctx context.Context, userID uuid.UUID) (*model.DashboardSummary, error) {
	accounts, err := s.accounts.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}

	var totalBalance float64
	for _, account := range accounts {
		totalBalance += account.Balance
	}

	income, expense, err := s.repo.MonthlyTotals(ctx, userID, time.Now().UTC())
	if err != nil {
		return nil, fmt.Errorf("monthly totals: %w", err)
	}

	recent, total, err := s.repo.List(ctx, model.TransactionFilters{
		UserID:   &userID,
		Page:     1,
		PageSize: 10,
	})
	if err != nil {
		return nil, fmt.Errorf("list recent transactions: %w", err)
	}
	if recent == nil {
		recent = []model.Transaction{}
	}

	return &model.DashboardSummary{
		TotalBalance:       totalBalance,
		MonthlyIncome:      income,
		MonthlyExpense:     expense,
		MonthlyNet:         income - expense,
		TransactionCount:   total,
		RecentTransactions: recent,
	}, nil
}

func isValidType(value string) bool {
	return value == TypeIncome || value == TypeExpense
}

func normalizeCurrency(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

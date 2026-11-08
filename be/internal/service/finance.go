package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"goskeleton/internal/model"
)

var (
	ErrAccountNotFound          = errors.New("account not found")
	ErrCategoryNotFound         = errors.New("category not found")
	ErrTransactionNotFound      = errors.New("transaction not found")
	ErrAccountNameExists        = errors.New("account name already exists")
	ErrCategoryNameExists       = errors.New("category name already exists")
	ErrCategoryInUse            = errors.New("category has transactions and cannot be deleted")
	ErrInvalidAmount            = errors.New("amount must be positive")
	ErrInvalidType              = errors.New("type must be income or expense")
	ErrInvalidName              = errors.New("name is required")
	ErrInvalidCurrency          = errors.New("currency must be a 3-letter code")
	ErrUseAdjustmentEndpoint    = errors.New("use the account adjustment endpoint to change a balance")
	ErrCurrencyLocked           = errors.New("currency is locked after the first entry")
	ErrSystemEntryImmutable     = errors.New("system entries cannot be modified")
	ErrInvalidBalance           = errors.New("balance must be a finite number")
	ErrSameAccount              = errors.New("cannot transfer to the same account")
	ErrTransferCurrencyMismatch = errors.New("transfer accounts must use the same currency")
	ErrTransferNotFound         = errors.New("transfer not found")
	ErrAccountHasTransfers      = errors.New("account has transfers and cannot be deleted")
	ErrInvalidDate              = errors.New("date must be in YYYY-MM-DD format")
)

const (
	// TypeIncome marks money flowing into an account.
	TypeIncome = "income"
	// TypeExpense marks money flowing out of an account.
	TypeExpense = "expense"

	// EntryKindNormal is a user, import, or recurring transaction.
	EntryKindNormal = "normal"
	// EntryKindOpening is the locked entry that seeds an account balance.
	EntryKindOpening = "opening"
	// EntryKindAdjustment is a locked correction to a real-world balance.
	EntryKindAdjustment = "adjustment"
	// EntryKindTransfer is one leg of a paired account-to-account movement.
	EntryKindTransfer = "transfer"

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
	Create(ctx context.Context, userID uuid.UUID, name string, openingBalance float64, currency string) (*model.Account, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.Account, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]model.Account, error)
	Update(ctx context.Context, userID uuid.UUID, id uuid.UUID, name *string, currency *string) (*model.Account, error)
	AdjustBalance(ctx context.Context, userID uuid.UUID, id uuid.UUID, targetBalance float64, description string) (*model.Account, error)
	Delete(ctx context.Context, userID uuid.UUID, id uuid.UUID) error
}

type CategoryRepository interface {
	Create(ctx context.Context, userID uuid.UUID, name string, type_ string, color string) (*model.Category, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.Category, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]model.Category, error)
	UpdateCategory(ctx context.Context, userID uuid.UUID, id uuid.UUID, name string, color string) (model.Category, error)
	CategoryHasTransactions(ctx context.Context, userID uuid.UUID, categoryID uuid.UUID) (bool, error)
	Delete(ctx context.Context, userID uuid.UUID, id uuid.UUID) error
}

type TransactionRepository interface {
	Create(ctx context.Context, tx model.Transaction) (*model.Transaction, error)
	GetByID(ctx context.Context, id uuid.UUID) (*model.Transaction, error)
	List(ctx context.Context, filters model.TransactionFilters) ([]model.Transaction, int, error)
	ForEachExportRow(ctx context.Context, filters model.TransactionFilters, emit func(model.TransactionExportRow) error) error
	CreateBatch(ctx context.Context, userID uuid.UUID, transactions []model.Transaction) error
	FingerprintCandidates(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]model.TransactionFingerprint, error)
	Update(ctx context.Context, userID uuid.UUID, transaction model.Transaction) (*model.Transaction, error)
	Delete(ctx context.Context, userID uuid.UUID, id uuid.UUID) error
	CreateTransfer(ctx context.Context, userID uuid.UUID, req model.CreateTransferRequest) (*model.TransferResult, error)
	DeleteTransfer(ctx context.Context, userID uuid.UUID, groupID uuid.UUID) error
	MonthlyTotalsByCurrency(ctx context.Context, userID uuid.UUID, month time.Time) ([]model.MonthlyCurrencyTotal, error)
}

type AccountService interface {
	CreateAccount(ctx context.Context, userID uuid.UUID, req model.CreateAccountRequest) (*model.Account, error)
	GetAccount(ctx context.Context, id uuid.UUID) (*model.Account, error)
	ListAccounts(ctx context.Context, userID uuid.UUID) ([]model.Account, error)
	UpdateAccount(ctx context.Context, userID uuid.UUID, id uuid.UUID, req model.UpdateAccountRequest) (*model.Account, error)
	AdjustAccountBalance(ctx context.Context, userID uuid.UUID, id uuid.UUID, req model.AdjustAccountRequest) (*model.Account, error)
	DeleteAccount(ctx context.Context, userID uuid.UUID, id uuid.UUID) error
}

type CategoryService interface {
	CreateCategory(ctx context.Context, userID uuid.UUID, req model.CreateCategoryRequest) (*model.Category, error)
	GetCategory(ctx context.Context, id uuid.UUID) (*model.Category, error)
	ListCategories(ctx context.Context, userID uuid.UUID) ([]model.Category, error)
	UpdateCategory(ctx context.Context, userID uuid.UUID, id uuid.UUID, req model.UpdateCategoryRequest) (*model.Category, error)
	DeleteCategory(ctx context.Context, userID uuid.UUID, id uuid.UUID) error
}

type TransactionService interface {
	CreateTransaction(ctx context.Context, userID uuid.UUID, req model.CreateTransactionRequest) (*model.Transaction, error)
	GetTransaction(ctx context.Context, id uuid.UUID) (*model.Transaction, error)
	ListTransactions(ctx context.Context, userID uuid.UUID, filters model.TransactionFilters) (*model.PaginatedTransactions, error)
	ExportTransactions(ctx context.Context, userID uuid.UUID, filters model.TransactionFilters, emit func(model.TransactionExportRow) error) error
	PreviewImport(ctx context.Context, userID uuid.UUID, r io.Reader) (*model.ImportResult, error)
	ImportTransactions(ctx context.Context, userID uuid.UUID, r io.Reader, allowDuplicates bool) (*model.ImportResult, error)
	UpdateTransaction(ctx context.Context, userID uuid.UUID, id uuid.UUID, req model.UpdateTransactionRequest) (*model.Transaction, error)
	DeleteTransaction(ctx context.Context, userID uuid.UUID, id uuid.UUID) error
	CreateTransfer(ctx context.Context, userID uuid.UUID, req model.CreateTransferRequest) (*model.TransferResult, error)
	DeleteTransfer(ctx context.Context, userID uuid.UUID, groupID uuid.UUID) error
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
	if math.IsNaN(req.OpeningBalance) || math.IsInf(req.OpeningBalance, 0) {
		return nil, ErrInvalidBalance
	}

	currency := normalizeCurrency(req.Currency)
	if currency == "" {
		currency = DefaultCurrency
	}

	account, err := s.repo.Create(ctx, userID, name, req.OpeningBalance, currency)
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

func (s *accountService) UpdateAccount(ctx context.Context, userID uuid.UUID, id uuid.UUID, req model.UpdateAccountRequest) (*model.Account, error) {
	if req.Balance != nil {
		return nil, ErrUseAdjustmentEndpoint
	}

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

	account, err := s.repo.Update(ctx, userID, id, req.Name, req.Currency)
	if err != nil {
		if errors.Is(err, ErrAccountNotFound) {
			return nil, ErrAccountNotFound
		}
		if errors.Is(err, ErrAccountNameExists) {
			return nil, ErrAccountNameExists
		}
		if errors.Is(err, ErrCurrencyLocked) {
			return nil, ErrCurrencyLocked
		}
		return nil, fmt.Errorf("update account: %w", err)
	}

	return account, nil
}

func (s *accountService) AdjustAccountBalance(ctx context.Context, userID uuid.UUID, id uuid.UUID, req model.AdjustAccountRequest) (*model.Account, error) {
	if math.IsNaN(req.TargetBalance) || math.IsInf(req.TargetBalance, 0) {
		return nil, ErrInvalidBalance
	}

	description := strings.TrimSpace(req.Description)
	if description == "" {
		description = "Balance adjustment"
	}

	account, err := s.repo.AdjustBalance(ctx, userID, id, req.TargetBalance, description)
	if err != nil {
		if errors.Is(err, ErrAccountNotFound) {
			return nil, ErrAccountNotFound
		}
		return nil, fmt.Errorf("adjust account balance: %w", err)
	}

	return account, nil
}

func (s *accountService) DeleteAccount(ctx context.Context, userID uuid.UUID, id uuid.UUID) error {
	if err := s.repo.Delete(ctx, userID, id); err != nil {
		if errors.Is(err, ErrAccountNotFound) {
			return ErrAccountNotFound
		}
		if errors.Is(err, ErrAccountHasTransfers) {
			return ErrAccountHasTransfers
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

func (s *categoryService) UpdateCategory(ctx context.Context, userID uuid.UUID, id uuid.UUID, req model.UpdateCategoryRequest) (*model.Category, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrCategoryNotFound) {
			return nil, ErrCategoryNotFound
		}
		return nil, fmt.Errorf("get category: %w", err)
	}
	if existing.UserID != userID {
		return nil, ErrCategoryNotFound
	}

	name := existing.Name
	if req.Name != nil {
		name = strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, ErrInvalidName
		}
	}

	color := existing.Color
	if req.Color != nil {
		color = strings.TrimSpace(*req.Color)
		if color == "" {
			color = DefaultCategoryColor
		}
	}

	updated, err := s.repo.UpdateCategory(ctx, userID, id, name, color)
	if err != nil {
		if errors.Is(err, ErrCategoryNotFound) {
			return nil, ErrCategoryNotFound
		}
		if errors.Is(err, ErrCategoryNameExists) {
			return nil, ErrCategoryNameExists
		}
		return nil, fmt.Errorf("update category: %w", err)
	}

	return &updated, nil
}

func (s *categoryService) DeleteCategory(ctx context.Context, userID uuid.UUID, id uuid.UUID) error {
	inUse, err := s.repo.CategoryHasTransactions(ctx, userID, id)
	if err != nil {
		return fmt.Errorf("check category transactions: %w", err)
	}
	if inUse {
		return ErrCategoryInUse
	}

	if err := s.repo.Delete(ctx, userID, id); err != nil {
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

var _ TransactionService = (*transactionService)(nil)

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
		Kind:        EntryKindNormal,
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

func (s *transactionService) ExportTransactions(ctx context.Context, userID uuid.UUID, filters model.TransactionFilters, emit func(model.TransactionExportRow) error) error {
	filters.UserID = &userID

	if filters.Type != "" {
		filters.Type = strings.ToLower(strings.TrimSpace(filters.Type))
		if !isValidType(filters.Type) {
			return ErrInvalidType
		}
	}

	if err := s.repo.ForEachExportRow(ctx, filters, emit); err != nil {
		return fmt.Errorf("export transactions: %w", err)
	}

	return nil
}

func (s *transactionService) UpdateTransaction(ctx context.Context, userID uuid.UUID, id uuid.UUID, req model.UpdateTransactionRequest) (*model.Transaction, error) {
	existing, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrTransactionNotFound) {
			return nil, ErrTransactionNotFound
		}
		return nil, fmt.Errorf("get transaction: %w", err)
	}
	if existing.UserID != userID {
		return nil, ErrTransactionNotFound
	}
	if existing.Kind != "" && existing.Kind != EntryKindNormal {
		return nil, ErrSystemEntryImmutable
	}

	updated := *existing

	if req.Type != nil {
		transactionType := strings.ToLower(strings.TrimSpace(*req.Type))
		if !isValidType(transactionType) {
			return nil, ErrInvalidType
		}
		updated.Type = transactionType
	}

	if req.AccountID != nil {
		account, err := s.accounts.GetByID(ctx, *req.AccountID)
		if err != nil {
			if errors.Is(err, ErrAccountNotFound) {
				return nil, ErrAccountNotFound
			}
			return nil, fmt.Errorf("get account: %w", err)
		}
		if account.UserID != userID {
			return nil, ErrAccountNotFound
		}
		updated.AccountID = account.ID
	}

	categoryID := updated.CategoryID
	if req.CategoryID != nil {
		categoryID = *req.CategoryID
	}
	category, err := s.categories.GetByID(ctx, categoryID)
	if err != nil {
		if errors.Is(err, ErrCategoryNotFound) {
			return nil, ErrCategoryNotFound
		}
		return nil, fmt.Errorf("get category: %w", err)
	}
	if category.UserID != userID {
		return nil, ErrCategoryNotFound
	}
	if category.Type != updated.Type {
		return nil, ErrInvalidType
	}
	updated.CategoryID = category.ID

	if req.Amount != nil {
		if *req.Amount <= 0 {
			return nil, ErrInvalidAmount
		}
		updated.Amount = *req.Amount
	}

	if req.Description != nil {
		updated.Description = strings.TrimSpace(*req.Description)
	}

	if req.Date != nil && !req.Date.IsZero() {
		updated.Date = *req.Date
	}

	result, err := s.repo.Update(ctx, userID, updated)
	if err != nil {
		if errors.Is(err, ErrTransactionNotFound) {
			return nil, ErrTransactionNotFound
		}
		if errors.Is(err, ErrSystemEntryImmutable) {
			return nil, ErrSystemEntryImmutable
		}
		return nil, fmt.Errorf("update transaction: %w", err)
	}

	return result, nil
}

func (s *transactionService) DeleteTransaction(ctx context.Context, userID uuid.UUID, id uuid.UUID) error {
	if err := s.repo.Delete(ctx, userID, id); err != nil {
		if errors.Is(err, ErrTransactionNotFound) {
			return ErrTransactionNotFound
		}
		if errors.Is(err, ErrSystemEntryImmutable) {
			return ErrSystemEntryImmutable
		}
		return fmt.Errorf("delete transaction: %w", err)
	}

	return nil
}

func (s *transactionService) CreateTransfer(ctx context.Context, userID uuid.UUID, req model.CreateTransferRequest) (*model.TransferResult, error) {
	if req.Amount <= 0 || math.IsNaN(req.Amount) || math.IsInf(req.Amount, 0) {
		return nil, ErrInvalidAmount
	}

	date, err := time.Parse("2006-01-02", strings.TrimSpace(req.Date))
	if err != nil {
		return nil, ErrInvalidDate
	}

	if req.FromAccountID == req.ToAccountID {
		return nil, ErrSameAccount
	}

	from, err := s.accounts.GetByID(ctx, req.FromAccountID)
	if err != nil {
		if errors.Is(err, ErrAccountNotFound) {
			return nil, ErrAccountNotFound
		}
		return nil, fmt.Errorf("get source account: %w", err)
	}
	if from.UserID != userID {
		return nil, ErrAccountNotFound
	}

	to, err := s.accounts.GetByID(ctx, req.ToAccountID)
	if err != nil {
		if errors.Is(err, ErrAccountNotFound) {
			return nil, ErrAccountNotFound
		}
		return nil, fmt.Errorf("get destination account: %w", err)
	}
	if to.UserID != userID {
		return nil, ErrAccountNotFound
	}

	if from.Currency != to.Currency {
		return nil, ErrTransferCurrencyMismatch
	}

	req.Date = date.Format("2006-01-02")
	transfer, err := s.repo.CreateTransfer(ctx, userID, req)
	if err != nil {
		if errors.Is(err, ErrAccountNotFound) {
			return nil, ErrAccountNotFound
		}
		if errors.Is(err, ErrSameAccount) {
			return nil, ErrSameAccount
		}
		if errors.Is(err, ErrTransferCurrencyMismatch) {
			return nil, ErrTransferCurrencyMismatch
		}
		return nil, fmt.Errorf("create transfer: %w", err)
	}

	return transfer, nil
}

func (s *transactionService) DeleteTransfer(ctx context.Context, userID uuid.UUID, groupID uuid.UUID) error {
	if err := s.repo.DeleteTransfer(ctx, userID, groupID); err != nil {
		if errors.Is(err, ErrTransferNotFound) {
			return ErrTransferNotFound
		}
		return fmt.Errorf("delete transfer: %w", err)
	}

	return nil
}

func (s *transactionService) GetDashboardSummary(ctx context.Context, userID uuid.UUID) (*model.DashboardSummary, error) {
	accounts, err := s.accounts.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}

	totals := make(map[string]float64, len(accounts))
	for _, account := range accounts {
		totals[account.Currency] += account.Balance
	}
	balances := make([]model.CurrencyTotal, 0, len(totals))
	for currency, total := range totals {
		balances = append(balances, model.CurrencyTotal{Currency: currency, Total: total})
	}
	sort.Slice(balances, func(i, j int) bool {
		return balances[i].Currency < balances[j].Currency
	})

	now := time.Now().UTC()
	monthly, err := s.repo.MonthlyTotalsByCurrency(ctx, userID, now)
	if err != nil {
		return nil, fmt.Errorf("monthly totals: %w", err)
	}
	for i := range monthly {
		monthly[i].Net = monthly[i].Income - monthly[i].Expense
	}
	if monthly == nil {
		monthly = []model.MonthlyCurrencyTotal{}
	}

	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	monthEnd := monthStart.AddDate(0, 1, 0).AddDate(0, 0, -1)
	_, total, err := s.repo.List(ctx, model.TransactionFilters{
		UserID:   &userID,
		Kind:     EntryKindNormal,
		DateFrom: &monthStart,
		DateTo:   &monthEnd,
		Page:     1,
		PageSize: 1,
	})
	if err != nil {
		return nil, fmt.Errorf("count transactions: %w", err)
	}

	recent, _, err := s.repo.List(ctx, model.TransactionFilters{
		UserID:   &userID,
		Kind:     EntryKindNormal,
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
		Balances:           balances,
		Monthly:            monthly,
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

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

type fakeAccountRepo struct {
	account    *model.Account
	accounts   []model.Account
	getErr     error
	listErr    error
	createErr  error
	updateErr  error
	deleteErr  error
	lastCreate struct {
		userID   uuid.UUID
		name     string
		balance  float64
		currency string
	}
	lastUpdate struct {
		id       uuid.UUID
		name     *string
		balance  *float64
		currency *string
	}
}

func (f *fakeAccountRepo) Create(ctx context.Context, userID uuid.UUID, name string, balance float64, currency string) (*model.Account, error) {
	f.lastCreate.userID = userID
	f.lastCreate.name = name
	f.lastCreate.balance = balance
	f.lastCreate.currency = currency
	if f.createErr != nil {
		return nil, f.createErr
	}
	return &model.Account{ID: uuid.New(), UserID: userID, Name: name, Balance: balance, Currency: currency}, nil
}

func (f *fakeAccountRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Account, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.account != nil {
		return f.account, nil
	}
	return nil, service.ErrAccountNotFound
}

func (f *fakeAccountRepo) ListByUser(ctx context.Context, userID uuid.UUID) ([]model.Account, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.accounts, nil
}

func (f *fakeAccountRepo) Update(ctx context.Context, id uuid.UUID, name *string, balance *float64, currency *string) (*model.Account, error) {
	f.lastUpdate.id = id
	f.lastUpdate.name = name
	f.lastUpdate.balance = balance
	f.lastUpdate.currency = currency
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	return &model.Account{ID: id, Name: "updated"}, nil
}

func (f *fakeAccountRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return f.deleteErr
}

type fakeCategoryRepo struct {
	category   *model.Category
	categories []model.Category
	getErr     error
	listErr    error
	createErr  error
	deleteErr  error
	lastCreate struct {
		userID uuid.UUID
		name   string
		typ    string
		color  string
	}
}

func (f *fakeCategoryRepo) Create(ctx context.Context, userID uuid.UUID, name string, type_ string, color string) (*model.Category, error) {
	f.lastCreate.userID = userID
	f.lastCreate.name = name
	f.lastCreate.typ = type_
	f.lastCreate.color = color
	if f.createErr != nil {
		return nil, f.createErr
	}
	return &model.Category{ID: uuid.New(), UserID: userID, Name: name, Type: type_, Color: color}, nil
}

func (f *fakeCategoryRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Category, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.category != nil {
		return f.category, nil
	}
	return nil, service.ErrCategoryNotFound
}

func (f *fakeCategoryRepo) ListByUser(ctx context.Context, userID uuid.UUID) ([]model.Category, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.categories, nil
}

func (f *fakeCategoryRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return f.deleteErr
}

type fakeTransactionRepo struct {
	created     *model.Transaction
	transaction *model.Transaction
	list        []model.Transaction
	total       int
	income      float64
	expense     float64
	createErr   error
	getErr      error
	listErr     error
	deleteErr   error
	monthlyErr  error
	lastFilters model.TransactionFilters
	lastCreated model.Transaction
}

func (f *fakeTransactionRepo) Create(ctx context.Context, tx model.Transaction) (*model.Transaction, error) {
	f.lastCreated = tx
	if f.createErr != nil {
		return nil, f.createErr
	}
	if f.created != nil {
		return f.created, nil
	}
	tx.ID = uuid.New()
	return &tx, nil
}

func (f *fakeTransactionRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Transaction, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.transaction != nil {
		return f.transaction, nil
	}
	return nil, service.ErrTransactionNotFound
}

func (f *fakeTransactionRepo) List(ctx context.Context, filters model.TransactionFilters) ([]model.Transaction, int, error) {
	f.lastFilters = filters
	if f.listErr != nil {
		return nil, 0, f.listErr
	}
	return f.list, f.total, nil
}

func (f *fakeTransactionRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return f.deleteErr
}

func (f *fakeTransactionRepo) MonthlyTotals(ctx context.Context, userID uuid.UUID, month time.Time) (float64, float64, error) {
	if f.monthlyErr != nil {
		return 0, 0, f.monthlyErr
	}
	return f.income, f.expense, nil
}

func TestAccountServiceCreateAccountDefaultsCurrency(t *testing.T) {
	t.Parallel()

	repo := &fakeAccountRepo{}
	accounts := service.NewAccountService(repo)
	userID := uuid.New()

	account, err := accounts.CreateAccount(context.Background(), userID, model.CreateAccountRequest{
		Name:    "  Checking  ",
		Balance: 100,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if repo.lastCreate.name != "Checking" {
		t.Fatalf("expected trimmed name %q, got %q", "Checking", repo.lastCreate.name)
	}
	if repo.lastCreate.currency != service.DefaultCurrency {
		t.Fatalf("expected default currency %q, got %q", service.DefaultCurrency, repo.lastCreate.currency)
	}
	if account.UserID != userID {
		t.Fatalf("expected user id %s, got %s", userID, account.UserID)
	}
}

func TestAccountServiceCreateAccountRejectsEmptyName(t *testing.T) {
	t.Parallel()

	accounts := service.NewAccountService(&fakeAccountRepo{})

	_, err := accounts.CreateAccount(context.Background(), uuid.New(), model.CreateAccountRequest{Name: "   "})
	if !errors.Is(err, service.ErrInvalidName) {
		t.Fatalf("expected ErrInvalidName, got %v", err)
	}
}

func TestAccountServiceCreateAccountReturnsNameExists(t *testing.T) {
	t.Parallel()

	accounts := service.NewAccountService(&fakeAccountRepo{createErr: service.ErrAccountNameExists})

	_, err := accounts.CreateAccount(context.Background(), uuid.New(), model.CreateAccountRequest{Name: "Checking"})
	if !errors.Is(err, service.ErrAccountNameExists) {
		t.Fatalf("expected ErrAccountNameExists, got %v", err)
	}
}

func TestAccountServiceUpdateAccountPartial(t *testing.T) {
	t.Parallel()

	repo := &fakeAccountRepo{}
	accounts := service.NewAccountService(repo)
	id := uuid.New()
	name := "  Savings  "

	_, err := accounts.UpdateAccount(context.Background(), id, model.UpdateAccountRequest{Name: &name})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if repo.lastUpdate.id != id {
		t.Fatalf("expected id %s, got %s", id, repo.lastUpdate.id)
	}
	if repo.lastUpdate.name == nil || *repo.lastUpdate.name != "Savings" {
		t.Fatalf("expected trimmed name %q, got %v", "Savings", repo.lastUpdate.name)
	}
	if repo.lastUpdate.balance != nil {
		t.Fatalf("expected balance to remain nil, got %v", *repo.lastUpdate.balance)
	}
	if repo.lastUpdate.currency != nil {
		t.Fatalf("expected currency to remain nil, got %v", *repo.lastUpdate.currency)
	}
}

func TestAccountServiceUpdateAccountRejectsEmptyName(t *testing.T) {
	t.Parallel()

	accounts := service.NewAccountService(&fakeAccountRepo{})
	empty := ""

	_, err := accounts.UpdateAccount(context.Background(), uuid.New(), model.UpdateAccountRequest{Name: &empty})
	if !errors.Is(err, service.ErrInvalidName) {
		t.Fatalf("expected ErrInvalidName, got %v", err)
	}
}

func TestAccountServiceUpdateAccountRejectsInvalidCurrency(t *testing.T) {
	t.Parallel()

	accounts := service.NewAccountService(&fakeAccountRepo{})
	currency := "US"

	_, err := accounts.UpdateAccount(context.Background(), uuid.New(), model.UpdateAccountRequest{Currency: &currency})
	if !errors.Is(err, service.ErrInvalidCurrency) {
		t.Fatalf("expected ErrInvalidCurrency, got %v", err)
	}
}

func TestAccountServiceDeleteAccountNotFound(t *testing.T) {
	t.Parallel()

	accounts := service.NewAccountService(&fakeAccountRepo{deleteErr: service.ErrAccountNotFound})

	err := accounts.DeleteAccount(context.Background(), uuid.New())
	if !errors.Is(err, service.ErrAccountNotFound) {
		t.Fatalf("expected ErrAccountNotFound, got %v", err)
	}
}

func TestCategoryServiceCreateCategoryDefaultsColor(t *testing.T) {
	t.Parallel()

	repo := &fakeCategoryRepo{}
	categories := service.NewCategoryService(repo)
	userID := uuid.New()

	category, err := categories.CreateCategory(context.Background(), userID, model.CreateCategoryRequest{
		Name: "Groceries",
		Type: "EXPENSE",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if repo.lastCreate.typ != service.TypeExpense {
		t.Fatalf("expected normalized type %q, got %q", service.TypeExpense, repo.lastCreate.typ)
	}
	if repo.lastCreate.color != service.DefaultCategoryColor {
		t.Fatalf("expected default color %q, got %q", service.DefaultCategoryColor, repo.lastCreate.color)
	}
	if category.UserID != userID {
		t.Fatalf("expected user id %s, got %s", userID, category.UserID)
	}
}

func TestCategoryServiceCreateCategoryRejectsInvalidType(t *testing.T) {
	t.Parallel()

	categories := service.NewCategoryService(&fakeCategoryRepo{})

	_, err := categories.CreateCategory(context.Background(), uuid.New(), model.CreateCategoryRequest{
		Name: "Transfer",
		Type: "transfer",
	})
	if !errors.Is(err, service.ErrInvalidType) {
		t.Fatalf("expected ErrInvalidType, got %v", err)
	}
}

func TestCategoryServiceCreateCategoryReturnsNameExists(t *testing.T) {
	t.Parallel()

	categories := service.NewCategoryService(&fakeCategoryRepo{createErr: service.ErrCategoryNameExists})

	_, err := categories.CreateCategory(context.Background(), uuid.New(), model.CreateCategoryRequest{
		Name: "Groceries",
		Type: service.TypeExpense,
	})
	if !errors.Is(err, service.ErrCategoryNameExists) {
		t.Fatalf("expected ErrCategoryNameExists, got %v", err)
	}
}

func TestTransactionServiceCreateTransactionIncome(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	accountID := uuid.New()
	categoryID := uuid.New()
	accountRepo := &fakeAccountRepo{account: &model.Account{ID: accountID, UserID: userID}}
	categoryRepo := &fakeCategoryRepo{category: &model.Category{ID: categoryID, UserID: userID, Type: service.TypeIncome}}
	transactionRepo := &fakeTransactionRepo{}
	transactions := service.NewTransactionService(transactionRepo, accountRepo, categoryRepo)

	transaction, err := transactions.CreateTransaction(context.Background(), userID, model.CreateTransactionRequest{
		AccountID:   accountID,
		CategoryID:  categoryID,
		Type:        "INCOME",
		Amount:      150.25,
		Description: "  Salary  ",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if transactionRepo.lastCreated.Type != service.TypeIncome {
		t.Fatalf("expected type %q, got %q", service.TypeIncome, transactionRepo.lastCreated.Type)
	}
	if transactionRepo.lastCreated.Description != "Salary" {
		t.Fatalf("expected trimmed description %q, got %q", "Salary", transactionRepo.lastCreated.Description)
	}
	if transactionRepo.lastCreated.Date.IsZero() {
		t.Fatal("expected date to default to now")
	}
	if transaction.UserID != userID {
		t.Fatalf("expected user id %s, got %s", userID, transaction.UserID)
	}
}

func TestTransactionServiceCreateTransactionExpense(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	accountID := uuid.New()
	categoryID := uuid.New()
	accountRepo := &fakeAccountRepo{account: &model.Account{ID: accountID, UserID: userID}}
	categoryRepo := &fakeCategoryRepo{category: &model.Category{ID: categoryID, UserID: userID, Type: service.TypeExpense}}
	transactionRepo := &fakeTransactionRepo{}
	transactions := service.NewTransactionService(transactionRepo, accountRepo, categoryRepo)

	_, err := transactions.CreateTransaction(context.Background(), userID, model.CreateTransactionRequest{
		AccountID:  accountID,
		CategoryID: categoryID,
		Type:       service.TypeExpense,
		Amount:     42.5,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if transactionRepo.lastCreated.Type != service.TypeExpense {
		t.Fatalf("expected type %q, got %q", service.TypeExpense, transactionRepo.lastCreated.Type)
	}
}

func TestTransactionServiceCreateTransactionRejectsInvalidAmount(t *testing.T) {
	t.Parallel()

	transactions := service.NewTransactionService(&fakeTransactionRepo{}, &fakeAccountRepo{}, &fakeCategoryRepo{})

	_, err := transactions.CreateTransaction(context.Background(), uuid.New(), model.CreateTransactionRequest{
		Type:   service.TypeExpense,
		Amount: 0,
	})
	if !errors.Is(err, service.ErrInvalidAmount) {
		t.Fatalf("expected ErrInvalidAmount, got %v", err)
	}
}

func TestTransactionServiceCreateTransactionRejectsInvalidType(t *testing.T) {
	t.Parallel()

	transactions := service.NewTransactionService(&fakeTransactionRepo{}, &fakeAccountRepo{}, &fakeCategoryRepo{})

	_, err := transactions.CreateTransaction(context.Background(), uuid.New(), model.CreateTransactionRequest{
		Type:   "transfer",
		Amount: 10,
	})
	if !errors.Is(err, service.ErrInvalidType) {
		t.Fatalf("expected ErrInvalidType, got %v", err)
	}
}

func TestTransactionServiceCreateTransactionRejectsForeignAccount(t *testing.T) {
	t.Parallel()

	accountID := uuid.New()
	accountRepo := &fakeAccountRepo{account: &model.Account{ID: accountID, UserID: uuid.New()}}
	transactions := service.NewTransactionService(&fakeTransactionRepo{}, accountRepo, &fakeCategoryRepo{})

	_, err := transactions.CreateTransaction(context.Background(), uuid.New(), model.CreateTransactionRequest{
		AccountID: accountID,
		Type:      service.TypeExpense,
		Amount:    10,
	})
	if !errors.Is(err, service.ErrAccountNotFound) {
		t.Fatalf("expected ErrAccountNotFound, got %v", err)
	}
}

func TestTransactionServiceCreateTransactionReturnsAccountNotFound(t *testing.T) {
	t.Parallel()

	transactions := service.NewTransactionService(&fakeTransactionRepo{}, &fakeAccountRepo{}, &fakeCategoryRepo{})

	_, err := transactions.CreateTransaction(context.Background(), uuid.New(), model.CreateTransactionRequest{
		AccountID: uuid.New(),
		Type:      service.TypeExpense,
		Amount:    10,
	})
	if !errors.Is(err, service.ErrAccountNotFound) {
		t.Fatalf("expected ErrAccountNotFound, got %v", err)
	}
}

func TestTransactionServiceCreateTransactionReturnsCategoryNotFound(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	accountID := uuid.New()
	accountRepo := &fakeAccountRepo{account: &model.Account{ID: accountID, UserID: userID}}
	transactions := service.NewTransactionService(&fakeTransactionRepo{}, accountRepo, &fakeCategoryRepo{})

	_, err := transactions.CreateTransaction(context.Background(), userID, model.CreateTransactionRequest{
		AccountID:  accountID,
		CategoryID: uuid.New(),
		Type:       service.TypeExpense,
		Amount:     10,
	})
	if !errors.Is(err, service.ErrCategoryNotFound) {
		t.Fatalf("expected ErrCategoryNotFound, got %v", err)
	}
}

func TestTransactionServiceCreateTransactionRejectsCategoryTypeMismatch(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	accountID := uuid.New()
	categoryID := uuid.New()
	accountRepo := &fakeAccountRepo{account: &model.Account{ID: accountID, UserID: userID}}
	categoryRepo := &fakeCategoryRepo{category: &model.Category{ID: categoryID, UserID: userID, Type: service.TypeExpense}}
	transactions := service.NewTransactionService(&fakeTransactionRepo{}, accountRepo, categoryRepo)

	_, err := transactions.CreateTransaction(context.Background(), userID, model.CreateTransactionRequest{
		AccountID:  accountID,
		CategoryID: categoryID,
		Type:       service.TypeIncome,
		Amount:     10,
	})
	if !errors.Is(err, service.ErrInvalidType) {
		t.Fatalf("expected ErrInvalidType, got %v", err)
	}
}

func TestTransactionServiceListTransactionsNormalizesPagination(t *testing.T) {
	t.Parallel()

	repo := &fakeTransactionRepo{list: []model.Transaction{{ID: uuid.New()}}, total: 1}
	transactions := service.NewTransactionService(repo, &fakeAccountRepo{}, &fakeCategoryRepo{})
	userID := uuid.New()

	result, err := transactions.ListTransactions(context.Background(), userID, model.TransactionFilters{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if repo.lastFilters.Page != 1 {
		t.Fatalf("expected page 1, got %d", repo.lastFilters.Page)
	}
	if repo.lastFilters.PageSize != service.DefaultPageSize {
		t.Fatalf("expected page size %d, got %d", service.DefaultPageSize, repo.lastFilters.PageSize)
	}
	if repo.lastFilters.UserID == nil || *repo.lastFilters.UserID != userID {
		t.Fatalf("expected user id %s in filters, got %v", userID, repo.lastFilters.UserID)
	}
	if result.Page != 1 || result.PageSize != service.DefaultPageSize {
		t.Fatalf("expected normalized pagination in result, got page %d size %d", result.Page, result.PageSize)
	}
}

func TestTransactionServiceListTransactionsCapsPageSize(t *testing.T) {
	t.Parallel()

	repo := &fakeTransactionRepo{}
	transactions := service.NewTransactionService(repo, &fakeAccountRepo{}, &fakeCategoryRepo{})

	_, err := transactions.ListTransactions(context.Background(), uuid.New(), model.TransactionFilters{Page: 2, PageSize: 500})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if repo.lastFilters.PageSize != service.MaxPageSize {
		t.Fatalf("expected page size %d, got %d", service.MaxPageSize, repo.lastFilters.PageSize)
	}
}

func TestTransactionServiceListTransactionsRejectsInvalidType(t *testing.T) {
	t.Parallel()

	transactions := service.NewTransactionService(&fakeTransactionRepo{}, &fakeAccountRepo{}, &fakeCategoryRepo{})

	_, err := transactions.ListTransactions(context.Background(), uuid.New(), model.TransactionFilters{Type: "transfer"})
	if !errors.Is(err, service.ErrInvalidType) {
		t.Fatalf("expected ErrInvalidType, got %v", err)
	}
}

func TestTransactionServiceGetDashboardSummary(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	accountRepo := &fakeAccountRepo{accounts: []model.Account{
		{ID: uuid.New(), UserID: userID, Balance: 100.50},
		{ID: uuid.New(), UserID: userID, Balance: 200},
	}}
	recent := []model.Transaction{{ID: uuid.New()}, {ID: uuid.New()}}
	transactionRepo := &fakeTransactionRepo{
		list:    recent,
		total:   42,
		income:  1000,
		expense: 250,
	}
	transactions := service.NewTransactionService(transactionRepo, accountRepo, &fakeCategoryRepo{})

	summary, err := transactions.GetDashboardSummary(context.Background(), userID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if summary.TotalBalance != 300.50 {
		t.Fatalf("expected total balance 300.50, got %v", summary.TotalBalance)
	}
	if summary.MonthlyIncome != 1000 {
		t.Fatalf("expected monthly income 1000, got %v", summary.MonthlyIncome)
	}
	if summary.MonthlyExpense != 250 {
		t.Fatalf("expected monthly expense 250, got %v", summary.MonthlyExpense)
	}
	if summary.MonthlyNet != 750 {
		t.Fatalf("expected monthly net 750, got %v", summary.MonthlyNet)
	}
	if summary.TransactionCount != 42 {
		t.Fatalf("expected transaction count 42, got %d", summary.TransactionCount)
	}
	if len(summary.RecentTransactions) != 2 {
		t.Fatalf("expected 2 recent transactions, got %d", len(summary.RecentTransactions))
	}
	if transactionRepo.lastFilters.PageSize != 10 {
		t.Fatalf("expected recent transactions page size 10, got %d", transactionRepo.lastFilters.PageSize)
	}
}

func TestTransactionServiceDeleteTransactionNotFound(t *testing.T) {
	t.Parallel()

	transactions := service.NewTransactionService(&fakeTransactionRepo{deleteErr: service.ErrTransactionNotFound}, &fakeAccountRepo{}, &fakeCategoryRepo{})

	err := transactions.DeleteTransaction(context.Background(), uuid.New())
	if !errors.Is(err, service.ErrTransactionNotFound) {
		t.Fatalf("expected ErrTransactionNotFound, got %v", err)
	}
}

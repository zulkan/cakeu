package service_test

import (
	"context"
	"errors"
	"math"
	"strings"
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
	adjustErr  error
	deleteErr  error
	deleted    []uuid.UUID
	lastCreate struct {
		userID         uuid.UUID
		name           string
		openingBalance float64
		currency       string
	}
	lastUpdate struct {
		id       uuid.UUID
		name     *string
		currency *string
	}
	lastAdjust model.AdjustAccountRequest
}

func (f *fakeAccountRepo) Create(ctx context.Context, userID uuid.UUID, name string, openingBalance float64, currency string) (*model.Account, error) {
	f.lastCreate.userID = userID
	f.lastCreate.name = name
	f.lastCreate.openingBalance = openingBalance
	f.lastCreate.currency = currency
	if f.createErr != nil {
		return nil, f.createErr
	}
	return &model.Account{
		ID:             uuid.New(),
		UserID:         userID,
		Name:           name,
		Balance:        openingBalance,
		OpeningBalance: openingBalance,
		Currency:       currency,
	}, nil
}

func (f *fakeAccountRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Account, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.account != nil {
		return f.account, nil
	}
	for _, account := range f.accounts {
		if account.ID == id {
			return &account, nil
		}
	}
	return nil, service.ErrAccountNotFound
}

func (f *fakeAccountRepo) ListByUser(ctx context.Context, userID uuid.UUID) ([]model.Account, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.accounts, nil
}

func (f *fakeAccountRepo) Update(ctx context.Context, userID uuid.UUID, id uuid.UUID, name *string, currency *string) (*model.Account, error) {
	f.lastUpdate.id = id
	f.lastUpdate.name = name
	f.lastUpdate.currency = currency
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	return &model.Account{ID: id, Name: "updated"}, nil
}

func (f *fakeAccountRepo) AdjustBalance(ctx context.Context, userID uuid.UUID, id uuid.UUID, targetBalance float64, description string) (*model.Account, error) {
	f.lastAdjust = model.AdjustAccountRequest{TargetBalance: targetBalance, Description: description}
	if f.adjustErr != nil {
		return nil, f.adjustErr
	}
	return &model.Account{ID: id, UserID: userID, Balance: targetBalance}, nil
}

func (f *fakeAccountRepo) Delete(ctx context.Context, userID uuid.UUID, id uuid.UUID) error {
	f.deleted = append(f.deleted, id)
	return f.deleteErr
}

type fakeCategoryRepo struct {
	category   *model.Category
	categories []model.Category
	getErr     error
	listErr    error
	createErr  error
	updateErr  error
	inUse      bool
	inUseErr   error
	deleteErr  error
	deleted    []uuid.UUID
	lastCreate struct {
		userID uuid.UUID
		name   string
		typ    string
		color  string
	}
	lastUpdate struct {
		userID uuid.UUID
		id     uuid.UUID
		name   string
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

func (f *fakeCategoryRepo) UpdateCategory(ctx context.Context, userID uuid.UUID, id uuid.UUID, name string, color string) (model.Category, error) {
	f.lastUpdate.userID = userID
	f.lastUpdate.id = id
	f.lastUpdate.name = name
	f.lastUpdate.color = color
	if f.updateErr != nil {
		return model.Category{}, f.updateErr
	}
	return model.Category{ID: id, UserID: userID, Name: name, Type: service.TypeExpense, Color: color}, nil
}

func (f *fakeCategoryRepo) CategoryHasTransactions(ctx context.Context, userID uuid.UUID, categoryID uuid.UUID) (bool, error) {
	if f.inUseErr != nil {
		return false, f.inUseErr
	}
	return f.inUse, nil
}

func (f *fakeCategoryRepo) Delete(ctx context.Context, userID uuid.UUID, id uuid.UUID) error {
	f.deleted = append(f.deleted, id)
	return f.deleteErr
}

type fakeTransactionRepo struct {
	created           *model.Transaction
	transaction       *model.Transaction
	updated           *model.Transaction
	list              []model.Transaction
	total             int
	monthly           []model.MonthlyCurrencyTotal
	createErr         error
	getErr            error
	listErr           error
	updateErr         error
	deleteErr         error
	monthlyErr        error
	exportErr         error
	batchErr          error
	exportRows        []model.TransactionExportRow
	lastBatch         []model.Transaction
	lastUpdated       model.Transaction
	deleted           []uuid.UUID
	lastFilters       model.TransactionFilters
	lastCreated       model.Transaction
	transfer          *model.TransferResult
	transferErr       error
	lastTransfer      model.CreateTransferRequest
	lastTransferGroup uuid.UUID
	deleteTransferErr error
	fingerprints      []model.TransactionFingerprint
	fingerprintErr    error
	fingerprintFrom   time.Time
	fingerprintTo     time.Time
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

func (f *fakeTransactionRepo) ForEachExportRow(ctx context.Context, filters model.TransactionFilters, emit func(model.TransactionExportRow) error) error {
	f.lastFilters = filters
	if f.exportErr != nil {
		return f.exportErr
	}
	for _, row := range f.exportRows {
		if err := emit(row); err != nil {
			return err
		}
	}
	return nil
}

func (f *fakeTransactionRepo) CreateBatch(ctx context.Context, userID uuid.UUID, transactions []model.Transaction) error {
	f.lastBatch = transactions
	return f.batchErr
}

func (f *fakeTransactionRepo) FingerprintCandidates(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]model.TransactionFingerprint, error) {
	f.fingerprintFrom = from
	f.fingerprintTo = to
	if f.fingerprintErr != nil {
		return nil, f.fingerprintErr
	}
	return f.fingerprints, nil
}

func (f *fakeTransactionRepo) Update(ctx context.Context, userID uuid.UUID, transaction model.Transaction) (*model.Transaction, error) {
	f.lastUpdated = transaction
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	if f.updated != nil {
		return f.updated, nil
	}
	return &transaction, nil
}

func (f *fakeTransactionRepo) Delete(ctx context.Context, userID uuid.UUID, id uuid.UUID) error {
	f.deleted = append(f.deleted, id)
	return f.deleteErr
}

func (f *fakeTransactionRepo) CreateTransfer(ctx context.Context, userID uuid.UUID, req model.CreateTransferRequest) (*model.TransferResult, error) {
	f.lastTransfer = req
	if f.transferErr != nil {
		return nil, f.transferErr
	}
	if f.transfer != nil {
		return f.transfer, nil
	}
	return &model.TransferResult{TransferGroupID: uuid.New()}, nil
}

func (f *fakeTransactionRepo) DeleteTransfer(ctx context.Context, userID uuid.UUID, groupID uuid.UUID) error {
	f.lastTransferGroup = groupID
	return f.deleteTransferErr
}

func (f *fakeTransactionRepo) MonthlyTotalsByCurrency(ctx context.Context, userID uuid.UUID, month time.Time) ([]model.MonthlyCurrencyTotal, error) {
	if f.monthlyErr != nil {
		return nil, f.monthlyErr
	}
	return f.monthly, nil
}

func TestAccountServiceCreateAccountDefaultsCurrency(t *testing.T) {
	t.Parallel()

	repo := &fakeAccountRepo{}
	accounts := service.NewAccountService(repo)
	userID := uuid.New()

	account, err := accounts.CreateAccount(context.Background(), userID, model.CreateAccountRequest{
		Name:           "  Checking  ",
		OpeningBalance: 100,
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

func TestAccountServiceCreateAccountUsesOpeningBalance(t *testing.T) {
	t.Parallel()

	repo := &fakeAccountRepo{}
	accounts := service.NewAccountService(repo)

	account, err := accounts.CreateAccount(context.Background(), uuid.New(), model.CreateAccountRequest{
		Name:           "Checking",
		OpeningBalance: -50,
		Currency:       "usd",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if repo.lastCreate.openingBalance != -50 {
		t.Fatalf("expected opening balance -50, got %v", repo.lastCreate.openingBalance)
	}
	if repo.lastCreate.currency != "USD" {
		t.Fatalf("expected normalized currency USD, got %q", repo.lastCreate.currency)
	}
	if account.Balance != -50 {
		t.Fatalf("expected derived balance -50, got %v", account.Balance)
	}
}

func TestAccountServiceCreateAccountRejectsInvalidOpeningBalance(t *testing.T) {
	t.Parallel()

	accounts := service.NewAccountService(&fakeAccountRepo{})

	_, err := accounts.CreateAccount(context.Background(), uuid.New(), model.CreateAccountRequest{
		Name:           "Checking",
		OpeningBalance: math.NaN(),
	})
	if !errors.Is(err, service.ErrInvalidBalance) {
		t.Fatalf("expected ErrInvalidBalance, got %v", err)
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

	_, err := accounts.UpdateAccount(context.Background(), uuid.New(), id, model.UpdateAccountRequest{Name: &name})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if repo.lastUpdate.id != id {
		t.Fatalf("expected id %s, got %s", id, repo.lastUpdate.id)
	}
	if repo.lastUpdate.name == nil || *repo.lastUpdate.name != "Savings" {
		t.Fatalf("expected trimmed name %q, got %v", "Savings", repo.lastUpdate.name)
	}
	if repo.lastUpdate.currency != nil {
		t.Fatalf("expected currency to remain nil, got %v", *repo.lastUpdate.currency)
	}
}

func TestAccountServiceUpdateAccountRejectsEmptyName(t *testing.T) {
	t.Parallel()

	accounts := service.NewAccountService(&fakeAccountRepo{})
	empty := ""

	_, err := accounts.UpdateAccount(context.Background(), uuid.New(), uuid.New(), model.UpdateAccountRequest{Name: &empty})
	if !errors.Is(err, service.ErrInvalidName) {
		t.Fatalf("expected ErrInvalidName, got %v", err)
	}
}

func TestAccountServiceUpdateAccountRejectsInvalidCurrency(t *testing.T) {
	t.Parallel()

	accounts := service.NewAccountService(&fakeAccountRepo{})
	currency := "US"

	_, err := accounts.UpdateAccount(context.Background(), uuid.New(), uuid.New(), model.UpdateAccountRequest{Currency: &currency})
	if !errors.Is(err, service.ErrInvalidCurrency) {
		t.Fatalf("expected ErrInvalidCurrency, got %v", err)
	}
}

func TestAccountServiceUpdateAccountRejectsBalanceEdit(t *testing.T) {
	t.Parallel()

	accounts := service.NewAccountService(&fakeAccountRepo{})
	balance := 100.0

	_, err := accounts.UpdateAccount(context.Background(), uuid.New(), uuid.New(), model.UpdateAccountRequest{Balance: &balance})
	if !errors.Is(err, service.ErrUseAdjustmentEndpoint) {
		t.Fatalf("expected ErrUseAdjustmentEndpoint, got %v", err)
	}
}

func TestAccountServiceAdjustAccountBalanceRecordsCorrection(t *testing.T) {
	t.Parallel()

	repo := &fakeAccountRepo{}
	accounts := service.NewAccountService(repo)
	id := uuid.New()

	account, err := accounts.AdjustAccountBalance(context.Background(), uuid.New(), id, model.AdjustAccountRequest{
		TargetBalance: 950,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if repo.lastAdjust.TargetBalance != 950 {
		t.Fatalf("expected target 950, got %v", repo.lastAdjust.TargetBalance)
	}
	if repo.lastAdjust.Description != "Balance adjustment" {
		t.Fatalf("expected default description, got %q", repo.lastAdjust.Description)
	}
	if account.Balance != 950 {
		t.Fatalf("expected updated balance 950, got %v", account.Balance)
	}
}

func TestAccountServiceAdjustAccountBalanceRejectsInvalidTarget(t *testing.T) {
	t.Parallel()

	accounts := service.NewAccountService(&fakeAccountRepo{})

	_, err := accounts.AdjustAccountBalance(context.Background(), uuid.New(), uuid.New(), model.AdjustAccountRequest{
		TargetBalance: math.Inf(1),
	})
	if !errors.Is(err, service.ErrInvalidBalance) {
		t.Fatalf("expected ErrInvalidBalance, got %v", err)
	}
}

func TestAccountServiceDeleteAccountNotFound(t *testing.T) {
	t.Parallel()

	accounts := service.NewAccountService(&fakeAccountRepo{deleteErr: service.ErrAccountNotFound})

	err := accounts.DeleteAccount(context.Background(), uuid.New(), uuid.New())
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

func TestCategoryServiceUpdateCategoryRenames(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	categoryID := uuid.New()
	repo := &fakeCategoryRepo{
		category: &model.Category{ID: categoryID, UserID: userID, Name: "Old", Type: service.TypeExpense, Color: "#111111"},
	}
	categories := service.NewCategoryService(repo)

	name := "  Groceries  "
	updated, err := categories.UpdateCategory(context.Background(), userID, categoryID, model.UpdateCategoryRequest{Name: &name})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if repo.lastUpdate.name != "Groceries" {
		t.Fatalf("expected trimmed name %q, got %q", "Groceries", repo.lastUpdate.name)
	}
	if repo.lastUpdate.color != "#111111" {
		t.Fatalf("expected existing color to be preserved, got %q", repo.lastUpdate.color)
	}
	if repo.lastUpdate.userID != userID || repo.lastUpdate.id != categoryID {
		t.Fatalf("expected scoped update for user %s category %s", userID, categoryID)
	}
	if updated.Name != "Groceries" {
		t.Fatalf("expected updated category name, got %q", updated.Name)
	}
}

func TestCategoryServiceUpdateCategoryDefaultsEmptyColor(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	categoryID := uuid.New()
	repo := &fakeCategoryRepo{
		category: &model.Category{ID: categoryID, UserID: userID, Name: "Groceries", Type: service.TypeExpense, Color: "#111111"},
	}
	categories := service.NewCategoryService(repo)

	color := "  "
	_, err := categories.UpdateCategory(context.Background(), userID, categoryID, model.UpdateCategoryRequest{Color: &color})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if repo.lastUpdate.color != service.DefaultCategoryColor {
		t.Fatalf("expected default color %q, got %q", service.DefaultCategoryColor, repo.lastUpdate.color)
	}
}

func TestCategoryServiceUpdateCategoryRejectsEmptyName(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	categoryID := uuid.New()
	repo := &fakeCategoryRepo{
		category: &model.Category{ID: categoryID, UserID: userID, Name: "Groceries", Type: service.TypeExpense},
	}
	categories := service.NewCategoryService(repo)

	name := "   "
	_, err := categories.UpdateCategory(context.Background(), userID, categoryID, model.UpdateCategoryRequest{Name: &name})
	if !errors.Is(err, service.ErrInvalidName) {
		t.Fatalf("expected ErrInvalidName, got %v", err)
	}
}

func TestCategoryServiceUpdateCategoryNotFound(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	repo := &fakeCategoryRepo{category: &model.Category{ID: uuid.New(), UserID: uuid.New()}}
	categories := service.NewCategoryService(repo)

	name := "Groceries"
	_, err := categories.UpdateCategory(context.Background(), userID, uuid.New(), model.UpdateCategoryRequest{Name: &name})
	if !errors.Is(err, service.ErrCategoryNotFound) {
		t.Fatalf("expected ErrCategoryNotFound, got %v", err)
	}
}

func TestCategoryServiceUpdateCategoryReturnsNameExists(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	categoryID := uuid.New()
	repo := &fakeCategoryRepo{
		category:  &model.Category{ID: categoryID, UserID: userID, Name: "Groceries"},
		updateErr: service.ErrCategoryNameExists,
	}
	categories := service.NewCategoryService(repo)

	name := "Dining"
	_, err := categories.UpdateCategory(context.Background(), userID, categoryID, model.UpdateCategoryRequest{Name: &name})
	if !errors.Is(err, service.ErrCategoryNameExists) {
		t.Fatalf("expected ErrCategoryNameExists, got %v", err)
	}
}

func TestCategoryServiceDeleteCategoryRejectsInUse(t *testing.T) {
	t.Parallel()

	repo := &fakeCategoryRepo{inUse: true}
	categories := service.NewCategoryService(repo)

	err := categories.DeleteCategory(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, service.ErrCategoryInUse) {
		t.Fatalf("expected ErrCategoryInUse, got %v", err)
	}
	if len(repo.deleted) != 0 {
		t.Fatalf("expected no delete when the category is in use, got %v", repo.deleted)
	}
}

func TestCategoryServiceDeleteCategoryDeletesUnused(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	categoryID := uuid.New()
	repo := &fakeCategoryRepo{}
	categories := service.NewCategoryService(repo)

	if err := categories.DeleteCategory(context.Background(), userID, categoryID); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(repo.deleted) != 1 || repo.deleted[0] != categoryID {
		t.Fatalf("expected category %s to be deleted, got %v", categoryID, repo.deleted)
	}
}

func TestCategoryServiceDeleteCategoryNotFound(t *testing.T) {
	t.Parallel()

	repo := &fakeCategoryRepo{deleteErr: service.ErrCategoryNotFound}
	categories := service.NewCategoryService(repo)

	err := categories.DeleteCategory(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, service.ErrCategoryNotFound) {
		t.Fatalf("expected ErrCategoryNotFound, got %v", err)
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

func TestTransactionServiceUpdateTransactionMergesPartialFields(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	transactionID := uuid.New()
	oldAccountID := uuid.New()
	newAccountID := uuid.New()
	oldCategoryID := uuid.New()
	newCategoryID := uuid.New()
	date := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)

	existing := &model.Transaction{
		ID:          transactionID,
		UserID:      userID,
		AccountID:   oldAccountID,
		CategoryID:  oldCategoryID,
		Type:        service.TypeExpense,
		Amount:      20,
		Description: "Old",
		Date:        time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
	}
	accountRepo := &fakeAccountRepo{account: &model.Account{ID: newAccountID, UserID: userID}}
	categoryRepo := &fakeCategoryRepo{category: &model.Category{ID: newCategoryID, UserID: userID, Type: service.TypeIncome}}
	transactionRepo := &fakeTransactionRepo{transaction: existing}
	transactions := service.NewTransactionService(transactionRepo, accountRepo, categoryRepo)

	amount := 125.5
	description := "  Salary  "
	transactionType := service.TypeIncome

	updated, err := transactions.UpdateTransaction(context.Background(), userID, transactionID, model.UpdateTransactionRequest{
		AccountID:   &newAccountID,
		CategoryID:  &newCategoryID,
		Type:        &transactionType,
		Amount:      &amount,
		Description: &description,
		Date:        &date,
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	got := transactionRepo.lastUpdated
	if got.AccountID != newAccountID {
		t.Fatalf("expected account %s, got %s", newAccountID, got.AccountID)
	}
	if got.CategoryID != newCategoryID {
		t.Fatalf("expected category %s, got %s", newCategoryID, got.CategoryID)
	}
	if got.Type != service.TypeIncome {
		t.Fatalf("expected type %q, got %q", service.TypeIncome, got.Type)
	}
	if got.Amount != amount {
		t.Fatalf("expected amount %v, got %v", amount, got.Amount)
	}
	if got.Description != "Salary" {
		t.Fatalf("expected trimmed description %q, got %q", "Salary", got.Description)
	}
	if !got.Date.Equal(date) {
		t.Fatalf("expected date %v, got %v", date, got.Date)
	}
	if updated.ID != transactionID {
		t.Fatalf("expected returned transaction %s, got %s", transactionID, updated.ID)
	}
}

func TestTransactionServiceUpdateTransactionKeepsOmittedFields(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	transactionID := uuid.New()
	accountID := uuid.New()
	categoryID := uuid.New()
	existing := &model.Transaction{
		ID:          transactionID,
		UserID:      userID,
		AccountID:   accountID,
		CategoryID:  categoryID,
		Type:        service.TypeExpense,
		Amount:      20,
		Description: "Coffee",
		Date:        time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
	}
	categoryRepo := &fakeCategoryRepo{category: &model.Category{ID: categoryID, UserID: userID, Type: service.TypeExpense}}
	transactionRepo := &fakeTransactionRepo{transaction: existing}
	transactions := service.NewTransactionService(transactionRepo, &fakeAccountRepo{}, categoryRepo)

	amount := 30.0
	_, err := transactions.UpdateTransaction(context.Background(), userID, transactionID, model.UpdateTransactionRequest{Amount: &amount})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	got := transactionRepo.lastUpdated
	if got.AccountID != accountID || got.CategoryID != categoryID {
		t.Fatalf("expected account and category to stay unchanged, got %s / %s", got.AccountID, got.CategoryID)
	}
	if got.Type != service.TypeExpense {
		t.Fatalf("expected type %q, got %q", service.TypeExpense, got.Type)
	}
	if got.Description != "Coffee" {
		t.Fatalf("expected description %q, got %q", "Coffee", got.Description)
	}
	if got.Amount != 30 {
		t.Fatalf("expected amount 30, got %v", got.Amount)
	}
}

func TestTransactionServiceUpdateTransactionRejectsForeignTransaction(t *testing.T) {
	t.Parallel()

	transactionID := uuid.New()
	existing := &model.Transaction{ID: transactionID, UserID: uuid.New()}
	transactionRepo := &fakeTransactionRepo{transaction: existing}
	transactions := service.NewTransactionService(transactionRepo, &fakeAccountRepo{}, &fakeCategoryRepo{})

	_, err := transactions.UpdateTransaction(context.Background(), uuid.New(), transactionID, model.UpdateTransactionRequest{})
	if !errors.Is(err, service.ErrTransactionNotFound) {
		t.Fatalf("expected ErrTransactionNotFound, got %v", err)
	}
	if transactionRepo.lastUpdated.ID != uuid.Nil {
		t.Fatal("expected repo update not to run for a foreign transaction")
	}
}

func TestTransactionServiceUpdateTransactionReturnsNotFound(t *testing.T) {
	t.Parallel()

	transactions := service.NewTransactionService(&fakeTransactionRepo{}, &fakeAccountRepo{}, &fakeCategoryRepo{})

	_, err := transactions.UpdateTransaction(context.Background(), uuid.New(), uuid.New(), model.UpdateTransactionRequest{})
	if !errors.Is(err, service.ErrTransactionNotFound) {
		t.Fatalf("expected ErrTransactionNotFound, got %v", err)
	}
}

func TestTransactionServiceUpdateTransactionRejectsInvalidAmount(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	transactionID := uuid.New()
	categoryID := uuid.New()
	existing := &model.Transaction{
		ID:         transactionID,
		UserID:     userID,
		CategoryID: categoryID,
		Type:       service.TypeExpense,
		Amount:     20,
	}
	categoryRepo := &fakeCategoryRepo{category: &model.Category{ID: categoryID, UserID: userID, Type: service.TypeExpense}}
	transactions := service.NewTransactionService(&fakeTransactionRepo{transaction: existing}, &fakeAccountRepo{}, categoryRepo)

	amount := 0.0
	_, err := transactions.UpdateTransaction(context.Background(), userID, transactionID, model.UpdateTransactionRequest{Amount: &amount})
	if !errors.Is(err, service.ErrInvalidAmount) {
		t.Fatalf("expected ErrInvalidAmount, got %v", err)
	}
}

func TestTransactionServiceUpdateTransactionRejectsCategoryTypeMismatch(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	transactionID := uuid.New()
	categoryID := uuid.New()
	existing := &model.Transaction{
		ID:         transactionID,
		UserID:     userID,
		CategoryID: uuid.New(),
		Type:       service.TypeExpense,
		Amount:     20,
	}
	categoryRepo := &fakeCategoryRepo{category: &model.Category{ID: categoryID, UserID: userID, Type: service.TypeIncome}}
	transactions := service.NewTransactionService(&fakeTransactionRepo{transaction: existing}, &fakeAccountRepo{}, categoryRepo)

	_, err := transactions.UpdateTransaction(context.Background(), userID, transactionID, model.UpdateTransactionRequest{CategoryID: &categoryID})
	if !errors.Is(err, service.ErrInvalidType) {
		t.Fatalf("expected ErrInvalidType, got %v", err)
	}
}

func TestTransactionServiceUpdateTransactionRejectsForeignAccount(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	transactionID := uuid.New()
	accountID := uuid.New()
	existing := &model.Transaction{ID: transactionID, UserID: userID, Type: service.TypeExpense, Amount: 20}
	accountRepo := &fakeAccountRepo{account: &model.Account{ID: accountID, UserID: uuid.New()}}
	transactions := service.NewTransactionService(&fakeTransactionRepo{transaction: existing}, accountRepo, &fakeCategoryRepo{})

	_, err := transactions.UpdateTransaction(context.Background(), userID, transactionID, model.UpdateTransactionRequest{AccountID: &accountID})
	if !errors.Is(err, service.ErrAccountNotFound) {
		t.Fatalf("expected ErrAccountNotFound, got %v", err)
	}
}

func TestTransactionServiceUpdateTransactionRejectsSystemEntry(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	transactionID := uuid.New()
	existing := &model.Transaction{
		ID:     transactionID,
		UserID: userID,
		Kind:   service.EntryKindOpening,
	}
	transactions := service.NewTransactionService(
		&fakeTransactionRepo{transaction: existing},
		&fakeAccountRepo{},
		&fakeCategoryRepo{},
	)

	_, err := transactions.UpdateTransaction(context.Background(), userID, transactionID, model.UpdateTransactionRequest{})
	if !errors.Is(err, service.ErrSystemEntryImmutable) {
		t.Fatalf("expected ErrSystemEntryImmutable, got %v", err)
	}
}

func TestTransactionServiceDeleteTransactionRejectsSystemEntry(t *testing.T) {
	t.Parallel()

	transactions := service.NewTransactionService(
		&fakeTransactionRepo{deleteErr: service.ErrSystemEntryImmutable},
		&fakeAccountRepo{},
		&fakeCategoryRepo{},
	)

	err := transactions.DeleteTransaction(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, service.ErrSystemEntryImmutable) {
		t.Fatalf("expected ErrSystemEntryImmutable, got %v", err)
	}
}

func TestTransactionServiceExportTransactions(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	repo := &fakeTransactionRepo{
		exportRows: []model.TransactionExportRow{
			{
				Date:         time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
				Type:         "expense",
				Amount:       12.5,
				Description:  "Coffee",
				AccountName:  "Cash",
				CategoryName: "Food",
			},
		},
	}
	transactions := service.NewTransactionService(repo, &fakeAccountRepo{}, &fakeCategoryRepo{})

	var exported []model.TransactionExportRow
	err := transactions.ExportTransactions(context.Background(), userID, model.TransactionFilters{Type: " EXPENSE "}, func(row model.TransactionExportRow) error {
		exported = append(exported, row)
		return nil
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(exported) != 1 || exported[0].Description != "Coffee" {
		t.Fatalf("unexpected exported rows: %+v", exported)
	}
	if repo.lastFilters.UserID == nil || *repo.lastFilters.UserID != userID {
		t.Fatalf("expected export scoped to user %s", userID)
	}
	if repo.lastFilters.Type != "expense" {
		t.Fatalf("expected normalized type %q, got %q", "expense", repo.lastFilters.Type)
	}
}

func TestTransactionServiceExportTransactionsRejectsInvalidType(t *testing.T) {
	t.Parallel()

	transactions := service.NewTransactionService(&fakeTransactionRepo{}, &fakeAccountRepo{}, &fakeCategoryRepo{})

	err := transactions.ExportTransactions(context.Background(), uuid.New(), model.TransactionFilters{Type: "bogus"}, func(model.TransactionExportRow) error {
		return nil
	})
	if !errors.Is(err, service.ErrInvalidType) {
		t.Fatalf("expected ErrInvalidType, got %v", err)
	}
}

func TestTransactionServicePreviewImportValidatesRows(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	accountID := uuid.New()
	foodID := uuid.New()
	accountRepo := &fakeAccountRepo{accounts: []model.Account{{ID: accountID, UserID: userID, Name: "Cash"}}}
	categoryRepo := &fakeCategoryRepo{categories: []model.Category{{ID: foodID, UserID: userID, Name: "Food", Type: "expense"}}}
	transactions := service.NewTransactionService(&fakeTransactionRepo{}, accountRepo, categoryRepo)

	body := strings.Join([]string{
		"date,type,amount,description,account,category",
		"2026-10-01,expense,12.50,Coffee,Cash,Food",
		"2026-10-02,expense,5,Unknown account,Cash,Nope",
		"not-a-date,expense,5,Bad date,Cash,Food",
		"2026-10-03,expense,-2,Negative,Cash,Food",
		"2026-10-04,income,10,Salary,Cash,Food",
	}, "\n") + "\n"

	result, err := transactions.PreviewImport(context.Background(), userID, strings.NewReader(body))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Total != 5 || result.Valid != 1 || result.Invalid != 4 {
		t.Fatalf("expected total=5 valid=1 invalid=4, got %+v", result)
	}
	if len(result.Errors) != 4 {
		t.Fatalf("expected 4 row errors, got %d", len(result.Errors))
	}
	if len(result.Preview) != 1 || result.Preview[0].Row != 2 || result.Preview[0].Amount != 12.5 {
		t.Fatalf("unexpected preview: %+v", result.Preview)
	}
}

func TestTransactionServiceImportTransactionsCreatesBatch(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	accountID := uuid.New()
	foodID := uuid.New()
	accountRepo := &fakeAccountRepo{accounts: []model.Account{{ID: accountID, UserID: userID, Name: "Cash"}}}
	categoryRepo := &fakeCategoryRepo{categories: []model.Category{{ID: foodID, UserID: userID, Name: "Food", Type: "expense"}}}
	repo := &fakeTransactionRepo{}
	transactions := service.NewTransactionService(repo, accountRepo, categoryRepo)

	body := "date,type,amount,description,account,category\n2026-10-01,expense,12.50,Coffee,Cash,Food\n"

	result, err := transactions.ImportTransactions(context.Background(), userID, strings.NewReader(body), false)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.Valid != 1 {
		t.Fatalf("expected 1 valid row, got %+v", result)
	}
	if len(repo.lastBatch) != 1 {
		t.Fatalf("expected 1 transaction in batch, got %d", len(repo.lastBatch))
	}
	if repo.lastBatch[0].AccountID != accountID || repo.lastBatch[0].CategoryID != foodID {
		t.Fatalf("expected batch row bound to resolved ids, got %+v", repo.lastBatch[0])
	}
	if repo.lastBatch[0].UserID != userID {
		t.Fatalf("expected batch row scoped to caller %s, got %s", userID, repo.lastBatch[0].UserID)
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
		{ID: uuid.New(), UserID: userID, Balance: 100.50, Currency: "USD"},
		{ID: uuid.New(), UserID: userID, Balance: 200, Currency: "USD"},
		{ID: uuid.New(), UserID: userID, Balance: 5000, Currency: "IDR"},
	}}
	recent := []model.Transaction{{ID: uuid.New()}, {ID: uuid.New()}}
	transactionRepo := &fakeTransactionRepo{
		list:  recent,
		total: 42,
		monthly: []model.MonthlyCurrencyTotal{
			{Currency: "USD", Income: 1000, Expense: 250},
		},
	}
	transactions := service.NewTransactionService(transactionRepo, accountRepo, &fakeCategoryRepo{})

	summary, err := transactions.GetDashboardSummary(context.Background(), userID)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(summary.Balances) != 2 {
		t.Fatalf("expected 2 currency balances, got %+v", summary.Balances)
	}
	if summary.Balances[0].Currency != "IDR" || summary.Balances[0].Total != 5000 {
		t.Fatalf("unexpected IDR balance: %+v", summary.Balances[0])
	}
	if summary.Balances[1].Currency != "USD" || summary.Balances[1].Total != 300.50 {
		t.Fatalf("unexpected USD balance: %+v", summary.Balances[1])
	}
	if len(summary.Monthly) != 1 || summary.Monthly[0].Net != 750 {
		t.Fatalf("unexpected monthly totals: %+v", summary.Monthly)
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

	err := transactions.DeleteTransaction(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, service.ErrTransactionNotFound) {
		t.Fatalf("expected ErrTransactionNotFound, got %v", err)
	}
}

func TestTransactionServiceCreateTransferCreatesPair(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	fromID := uuid.New()
	toID := uuid.New()
	repo := &fakeTransactionRepo{}
	accounts := &fakeAccountRepo{accounts: []model.Account{
		{ID: fromID, UserID: userID, Currency: "USD"},
		{ID: toID, UserID: userID, Currency: "USD"},
	}}
	transactions := service.NewTransactionService(repo, accounts, &fakeCategoryRepo{})

	transfer, err := transactions.CreateTransfer(context.Background(), userID, model.CreateTransferRequest{
		FromAccountID: fromID,
		ToAccountID:   toID,
		Amount:        100,
		Date:          "2026-11-08",
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if transfer == nil || transfer.TransferGroupID == uuid.Nil {
		t.Fatalf("expected a transfer group id, got %+v", transfer)
	}
	if repo.lastTransfer.Date != "2026-11-08" {
		t.Fatalf("expected date %q forwarded, got %q", "2026-11-08", repo.lastTransfer.Date)
	}
}

func TestTransactionServiceCreateTransferRejectsSameAccount(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	accountID := uuid.New()
	accounts := &fakeAccountRepo{accounts: []model.Account{
		{ID: accountID, UserID: userID, Currency: "USD"},
	}}
	transactions := service.NewTransactionService(&fakeTransactionRepo{}, accounts, &fakeCategoryRepo{})

	_, err := transactions.CreateTransfer(context.Background(), userID, model.CreateTransferRequest{
		FromAccountID: accountID,
		ToAccountID:   accountID,
		Amount:        100,
		Date:          "2026-11-08",
	})
	if !errors.Is(err, service.ErrSameAccount) {
		t.Fatalf("expected ErrSameAccount, got %v", err)
	}
}

func TestTransactionServiceCreateTransferRejectsCrossCurrency(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	fromID := uuid.New()
	toID := uuid.New()
	accounts := &fakeAccountRepo{accounts: []model.Account{
		{ID: fromID, UserID: userID, Currency: "USD"},
		{ID: toID, UserID: userID, Currency: "IDR"},
	}}
	transactions := service.NewTransactionService(&fakeTransactionRepo{}, accounts, &fakeCategoryRepo{})

	_, err := transactions.CreateTransfer(context.Background(), userID, model.CreateTransferRequest{
		FromAccountID: fromID,
		ToAccountID:   toID,
		Amount:        100,
		Date:          "2026-11-08",
	})
	if !errors.Is(err, service.ErrTransferCurrencyMismatch) {
		t.Fatalf("expected ErrTransferCurrencyMismatch, got %v", err)
	}
}

func TestTransactionServiceCreateTransferRejectsUnknownAccount(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	knownID := uuid.New()
	accounts := &fakeAccountRepo{accounts: []model.Account{
		{ID: knownID, UserID: userID, Currency: "USD"},
	}}
	transactions := service.NewTransactionService(&fakeTransactionRepo{}, accounts, &fakeCategoryRepo{})

	_, err := transactions.CreateTransfer(context.Background(), userID, model.CreateTransferRequest{
		FromAccountID: knownID,
		ToAccountID:   uuid.New(),
		Amount:        100,
		Date:          "2026-11-08",
	})
	if !errors.Is(err, service.ErrAccountNotFound) {
		t.Fatalf("expected ErrAccountNotFound, got %v", err)
	}
}

func TestTransactionServiceCreateTransferRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	fromID := uuid.New()
	toID := uuid.New()
	accounts := &fakeAccountRepo{accounts: []model.Account{
		{ID: fromID, UserID: userID, Currency: "USD"},
		{ID: toID, UserID: userID, Currency: "USD"},
	}}

	tests := []struct {
		name   string
		amount float64
		date   string
		want   error
	}{
		{"zero amount", 0, "2026-11-08", service.ErrInvalidAmount},
		{"negative amount", -10, "2026-11-08", service.ErrInvalidAmount},
		{"bad date", 100, "08/11/2026", service.ErrInvalidDate},
		{"empty date", 100, "", service.ErrInvalidDate},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			transactions := service.NewTransactionService(&fakeTransactionRepo{}, accounts, &fakeCategoryRepo{})
			_, err := transactions.CreateTransfer(context.Background(), userID, model.CreateTransferRequest{
				FromAccountID: fromID,
				ToAccountID:   toID,
				Amount:        test.amount,
				Date:          test.date,
			})
			if !errors.Is(err, test.want) {
				t.Fatalf("expected %v, got %v", test.want, err)
			}
		})
	}
}

func TestTransactionServiceDeleteTransferNotFound(t *testing.T) {
	t.Parallel()

	transactions := service.NewTransactionService(
		&fakeTransactionRepo{deleteTransferErr: service.ErrTransferNotFound},
		&fakeAccountRepo{},
		&fakeCategoryRepo{},
	)

	err := transactions.DeleteTransfer(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, service.ErrTransferNotFound) {
		t.Fatalf("expected ErrTransferNotFound, got %v", err)
	}
}

func TestAccountServiceDeleteAccountWithTransferLeg(t *testing.T) {
	t.Parallel()

	accounts := service.NewAccountService(&fakeAccountRepo{deleteErr: service.ErrAccountHasTransfers})

	err := accounts.DeleteAccount(context.Background(), uuid.New(), uuid.New())
	if !errors.Is(err, service.ErrAccountHasTransfers) {
		t.Fatalf("expected ErrAccountHasTransfers, got %v", err)
	}
}

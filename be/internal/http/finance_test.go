package httpserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

	httpserver "goskeleton/internal/http"
	"goskeleton/internal/model"
	"goskeleton/internal/service"
	"goskeleton/internal/spreadsheet"
)

const testJWTSecret = "test-secret"

type fakeAccountService struct {
	accounts   []model.Account
	account    *model.Account
	err        error
	lastUserID uuid.UUID
	lastID     uuid.UUID
	lastCreate model.CreateAccountRequest
	lastUpdate model.UpdateAccountRequest
	lastAdjust model.AdjustAccountRequest
}

func (f *fakeAccountService) CreateAccount(ctx context.Context, userID uuid.UUID, req model.CreateAccountRequest) (*model.Account, error) {
	f.lastUserID = userID
	f.lastCreate = req
	if f.err != nil {
		return nil, f.err
	}
	if f.account != nil {
		return f.account, nil
	}
	return &model.Account{
		ID:             uuid.New(),
		UserID:         userID,
		Name:           req.Name,
		Balance:        req.OpeningBalance,
		OpeningBalance: req.OpeningBalance,
		Currency:       req.Currency,
	}, nil
}

func (f *fakeAccountService) AdjustAccountBalance(ctx context.Context, userID uuid.UUID, id uuid.UUID, req model.AdjustAccountRequest) (*model.Account, error) {
	f.lastUserID = userID
	f.lastID = id
	f.lastAdjust = req
	if f.err != nil {
		return nil, f.err
	}
	if f.account != nil {
		return f.account, nil
	}
	return &model.Account{ID: id, UserID: userID, Balance: req.TargetBalance}, nil
}

func (f *fakeAccountService) GetAccount(ctx context.Context, id uuid.UUID) (*model.Account, error) {
	f.lastID = id
	if f.err != nil {
		return nil, f.err
	}
	return f.account, nil
}

func (f *fakeAccountService) ListAccounts(ctx context.Context, userID uuid.UUID) ([]model.Account, error) {
	f.lastUserID = userID
	if f.err != nil {
		return nil, f.err
	}
	return f.accounts, nil
}

func (f *fakeAccountService) UpdateAccount(ctx context.Context, userID uuid.UUID, id uuid.UUID, req model.UpdateAccountRequest) (*model.Account, error) {
	f.lastUserID = userID
	f.lastID = id
	f.lastUpdate = req
	if f.err != nil {
		return nil, f.err
	}
	if f.account != nil {
		return f.account, nil
	}
	return &model.Account{ID: id}, nil
}

func (f *fakeAccountService) DeleteAccount(ctx context.Context, userID uuid.UUID, id uuid.UUID) error {
	f.lastUserID = userID
	f.lastID = id
	return f.err
}

type fakeCategoryService struct {
	categories []model.Category
	category   *model.Category
	err        error
	updateErr  error
	lastUserID uuid.UUID
	lastID     uuid.UUID
	lastCreate model.CreateCategoryRequest
	lastUpdate model.UpdateCategoryRequest
}

func (f *fakeCategoryService) CreateCategory(ctx context.Context, userID uuid.UUID, req model.CreateCategoryRequest) (*model.Category, error) {
	f.lastUserID = userID
	f.lastCreate = req
	if f.err != nil {
		return nil, f.err
	}
	if f.category != nil {
		return f.category, nil
	}
	return &model.Category{ID: uuid.New(), UserID: userID, Name: req.Name, Type: req.Type, Color: req.Color}, nil
}

func (f *fakeCategoryService) GetCategory(ctx context.Context, id uuid.UUID) (*model.Category, error) {
	f.lastID = id
	if f.err != nil {
		return nil, f.err
	}
	return f.category, nil
}

func (f *fakeCategoryService) ListCategories(ctx context.Context, userID uuid.UUID) ([]model.Category, error) {
	f.lastUserID = userID
	if f.err != nil {
		return nil, f.err
	}
	return f.categories, nil
}

func (f *fakeCategoryService) UpdateCategory(ctx context.Context, userID uuid.UUID, id uuid.UUID, req model.UpdateCategoryRequest) (*model.Category, error) {
	f.lastUserID = userID
	f.lastID = id
	f.lastUpdate = req
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	if f.category != nil {
		return f.category, nil
	}
	return &model.Category{ID: id, UserID: userID}, nil
}

func (f *fakeCategoryService) DeleteCategory(ctx context.Context, userID uuid.UUID, id uuid.UUID) error {
	f.lastUserID = userID
	f.lastID = id
	return f.err
}

type fakeTransactionService struct {
	transaction       *model.Transaction
	paginated         *model.PaginatedTransactions
	summary           *model.DashboardSummary
	err               error
	lastUserID        uuid.UUID
	lastID            uuid.UUID
	lastFilters       model.TransactionFilters
	lastCreate        model.CreateTransactionRequest
	lastUpdate        model.UpdateTransactionRequest
	exportRows        []model.TransactionExportRow
	importResult      *model.ImportResult
	transfer          *model.TransferResult
	lastTransfer      model.CreateTransferRequest
	lastTransferGroup uuid.UUID
}

func (f *fakeTransactionService) CreateTransaction(ctx context.Context, userID uuid.UUID, req model.CreateTransactionRequest) (*model.Transaction, error) {
	f.lastUserID = userID
	f.lastCreate = req
	if f.err != nil {
		return nil, f.err
	}
	if f.transaction != nil {
		return f.transaction, nil
	}
	return &model.Transaction{ID: uuid.New(), UserID: userID, AccountID: req.AccountID, CategoryID: req.CategoryID, Type: req.Type, Amount: req.Amount}, nil
}

func (f *fakeTransactionService) GetTransaction(ctx context.Context, id uuid.UUID) (*model.Transaction, error) {
	f.lastID = id
	if f.err != nil {
		return nil, f.err
	}
	return f.transaction, nil
}

func (f *fakeTransactionService) ListTransactions(ctx context.Context, userID uuid.UUID, filters model.TransactionFilters) (*model.PaginatedTransactions, error) {
	f.lastUserID = userID
	f.lastFilters = filters
	if f.err != nil {
		return nil, f.err
	}
	if f.paginated != nil {
		return f.paginated, nil
	}
	return &model.PaginatedTransactions{Data: []model.Transaction{}, Page: 1, PageSize: 20}, nil
}

func (f *fakeTransactionService) ExportTransactions(ctx context.Context, userID uuid.UUID, filters model.TransactionFilters, emit func(model.TransactionExportRow) error) error {
	f.lastUserID = userID
	f.lastFilters = filters
	if f.err != nil {
		return f.err
	}
	for _, row := range f.exportRows {
		if err := emit(row); err != nil {
			return err
		}
	}
	return nil
}

func (f *fakeTransactionService) PreviewImport(ctx context.Context, userID uuid.UUID, r io.Reader) (*model.ImportResult, error) {
	f.lastUserID = userID
	if f.err != nil {
		return nil, f.err
	}
	if f.importResult != nil {
		return f.importResult, nil
	}
	return &model.ImportResult{Errors: []model.ImportRowError{}, Preview: []model.ImportPreviewRow{}}, nil
}

func (f *fakeTransactionService) ImportTransactions(ctx context.Context, userID uuid.UUID, r io.Reader, allowDuplicates bool) (*model.ImportResult, error) {
	f.lastUserID = userID
	if f.err != nil {
		return nil, f.err
	}
	if f.importResult != nil {
		return f.importResult, nil
	}
	return &model.ImportResult{Errors: []model.ImportRowError{}, Preview: []model.ImportPreviewRow{}}, nil
}

func (f *fakeTransactionService) UpdateTransaction(ctx context.Context, userID uuid.UUID, id uuid.UUID, req model.UpdateTransactionRequest) (*model.Transaction, error) {
	f.lastUserID = userID
	f.lastID = id
	f.lastUpdate = req
	if f.err != nil {
		return nil, f.err
	}
	if f.transaction != nil {
		return f.transaction, nil
	}
	return &model.Transaction{ID: id, UserID: userID}, nil
}

func (f *fakeTransactionService) DeleteTransaction(ctx context.Context, userID uuid.UUID, id uuid.UUID) error {
	f.lastUserID = userID
	f.lastID = id
	return f.err
}

func (f *fakeTransactionService) CreateTransfer(ctx context.Context, userID uuid.UUID, req model.CreateTransferRequest) (*model.TransferResult, error) {
	f.lastUserID = userID
	f.lastTransfer = req
	if f.err != nil {
		return nil, f.err
	}
	if f.transfer != nil {
		return f.transfer, nil
	}
	return &model.TransferResult{TransferGroupID: uuid.New()}, nil
}

func (f *fakeTransactionService) DeleteTransfer(ctx context.Context, userID uuid.UUID, groupID uuid.UUID) error {
	f.lastUserID = userID
	f.lastTransferGroup = groupID
	return f.err
}

func (f *fakeTransactionService) GetDashboardSummary(ctx context.Context, userID uuid.UUID) (*model.DashboardSummary, error) {
	f.lastUserID = userID
	if f.err != nil {
		return nil, f.err
	}
	if f.summary != nil {
		return f.summary, nil
	}
	return &model.DashboardSummary{RecentTransactions: []model.Transaction{}}, nil
}

type fakeImportService struct {
	result      *model.ImportResult
	err         error
	lastUserID  uuid.UUID
	lastOptions service.ImportOptions
	lastTable   *spreadsheet.Table
	lastSheets  []string
}

func (f *fakeImportService) Preview(ctx context.Context, userID uuid.UUID, table *spreadsheet.Table, sheets []string, opts service.ImportOptions) (*model.ImportResult, error) {
	f.lastUserID = userID
	f.lastOptions = opts
	f.lastTable = table
	f.lastSheets = sheets
	if f.err != nil {
		return nil, f.err
	}
	return f.result, nil
}

func (f *fakeImportService) Import(ctx context.Context, userID uuid.UUID, table *spreadsheet.Table, sheets []string, opts service.ImportOptions) (*model.ImportResult, error) {
	f.lastUserID = userID
	f.lastOptions = opts
	f.lastTable = table
	f.lastSheets = sheets
	if f.err != nil {
		return nil, f.err
	}
	return f.result, nil
}

func newTestHandler(accounts *fakeAccountService, categories *fakeCategoryService, transactions *fakeTransactionService, origins []string) http.Handler {
	imports := &fakeImportService{result: transactions.importResult}
	return newTestHandlerWithImports(accounts, categories, transactions, imports, origins)
}

func newTestHandlerWithImports(accounts *fakeAccountService, categories *fakeCategoryService, transactions *fakeTransactionService, imports httpserver.ImportService, origins []string) http.Handler {
	return httpserver.NewHandler(service.NewHealth(nil), nil, accounts, categories, transactions, imports, service.ImportModeAI, nil, nil, nil, nil, nil, testJWTSecret, origins)
}

func newTestHandlerWithImportMode(imports httpserver.ImportService, importMode service.ImportMode, origins []string) http.Handler {
	return httpserver.NewHandler(service.NewHealth(nil), nil, &fakeAccountService{}, &fakeCategoryService{}, &fakeTransactionService{}, imports, importMode, nil, nil, nil, nil, nil, testJWTSecret, origins)
}

func signToken(t *testing.T, secret string, userID uuid.UUID) string {
	t.Helper()

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub": userID.String(),
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("expected token signing to succeed, got error: %v", err)
	}

	return signed
}

func doRequest(t *testing.T, handler http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()

	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}

	request := httptest.NewRequest(method, path, reader)
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	return recorder
}

func TestFinanceEndpointsRequireAuth(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(&fakeAccountService{}, &fakeCategoryService{}, &fakeTransactionService{}, nil)
	id := uuid.New().String()

	tests := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/accounts"},
		{http.MethodPost, "/api/accounts"},
		{http.MethodPatch, "/api/accounts/" + id},
		{http.MethodDelete, "/api/accounts/" + id},
		{http.MethodGet, "/api/categories"},
		{http.MethodPost, "/api/categories"},
		{http.MethodDelete, "/api/categories/" + id},
		{http.MethodGet, "/api/transactions"},
		{http.MethodPost, "/api/transactions"},
		{http.MethodDelete, "/api/transactions/" + id},
		{http.MethodPost, "/api/transfers"},
		{http.MethodDelete, "/api/transfers/" + id},
		{http.MethodGet, "/api/dashboard/summary"},
	}

	for _, test := range tests {
		test := test
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			t.Parallel()

			recorder := doRequest(t, handler, test.method, test.path, "", "")
			if recorder.Code != http.StatusUnauthorized {
				t.Fatalf("expected status 401, got %d with body %q", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestFinanceEndpointsRejectInvalidToken(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(&fakeAccountService{}, &fakeCategoryService{}, &fakeTransactionService{}, nil)

	recorder := doRequest(t, handler, http.MethodGet, "/api/accounts", "not-a-token", "")
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d with body %q", recorder.Code, recorder.Body.String())
	}
}

func TestFinanceEndpointsRejectTokenWithWrongSignature(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(&fakeAccountService{}, &fakeCategoryService{}, &fakeTransactionService{}, nil)
	token := signToken(t, "other-secret", uuid.New())

	recorder := doRequest(t, handler, http.MethodGet, "/api/accounts", token, "")
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d with body %q", recorder.Code, recorder.Body.String())
	}
}

func TestListAccountsReturnsAccounts(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	accounts := &fakeAccountService{accounts: []model.Account{
		{ID: uuid.New(), UserID: userID, Name: "Checking", Balance: 100, Currency: "USD"},
		{ID: uuid.New(), UserID: userID, Name: "Savings", Balance: 200, Currency: "USD"},
	}}
	handler := newTestHandler(accounts, &fakeCategoryService{}, &fakeTransactionService{}, nil)

	recorder := doRequest(t, handler, http.MethodGet, "/api/accounts", signToken(t, testJWTSecret, userID), "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d with body %q", recorder.Code, recorder.Body.String())
	}

	var response []model.Account
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected JSON array, got error: %v", err)
	}
	if len(response) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(response))
	}
	if accounts.lastUserID != userID {
		t.Fatalf("expected user id %s, got %s", userID, accounts.lastUserID)
	}
}

func TestCreateAccount(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	accounts := &fakeAccountService{}
	handler := newTestHandler(accounts, &fakeCategoryService{}, &fakeTransactionService{}, nil)

	recorder := doRequest(t, handler, http.MethodPost, "/api/accounts", signToken(t, testJWTSecret, userID),
		`{"name":"Checking","opening_balance":100,"currency":"USD"}`)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d with body %q", recorder.Code, recorder.Body.String())
	}
	if accounts.lastCreate.Name != "Checking" {
		t.Fatalf("expected name %q, got %q", "Checking", accounts.lastCreate.Name)
	}
	if accounts.lastUserID != userID {
		t.Fatalf("expected user id %s, got %s", userID, accounts.lastUserID)
	}
}

func TestUpdateAccount(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	accountID := uuid.New()
	accounts := &fakeAccountService{account: &model.Account{ID: accountID, UserID: userID, Name: "Updated"}}
	handler := newTestHandler(accounts, &fakeCategoryService{}, &fakeTransactionService{}, nil)

	recorder := doRequest(t, handler, http.MethodPatch, "/api/accounts/"+accountID.String(), signToken(t, testJWTSecret, userID),
		`{"name":"Updated"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d with body %q", recorder.Code, recorder.Body.String())
	}
	if accounts.lastID != accountID {
		t.Fatalf("expected account id %s, got %s", accountID, accounts.lastID)
	}
	if accounts.lastUserID != userID {
		t.Fatalf("expected caller %s forwarded to UpdateAccount, got %s", userID, accounts.lastUserID)
	}
	if accounts.lastUpdate.Name == nil || *accounts.lastUpdate.Name != "Updated" {
		t.Fatalf("expected name %q, got %v", "Updated", accounts.lastUpdate.Name)
	}
}

func TestUpdateAccountRejectsBalanceField(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	accountID := uuid.New()
	accounts := &fakeAccountService{}
	handler := newTestHandler(accounts, &fakeCategoryService{}, &fakeTransactionService{}, nil)

	recorder := doRequest(t, handler, http.MethodPatch, "/api/accounts/"+accountID.String(), signToken(t, testJWTSecret, userID),
		`{"balance":2500}`)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d with body %q", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "adjustment") {
		t.Fatalf("expected adjustment guidance, got %q", recorder.Body.String())
	}
}

func TestCreateAccountRejectsBalanceField(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	accounts := &fakeAccountService{}
	handler := newTestHandler(accounts, &fakeCategoryService{}, &fakeTransactionService{}, nil)

	recorder := doRequest(t, handler, http.MethodPost, "/api/accounts", signToken(t, testJWTSecret, userID),
		`{"name":"Checking","balance":100}`)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d with body %q", recorder.Code, recorder.Body.String())
	}
}

func TestAdjustAccountBalance(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	accountID := uuid.New()
	accounts := &fakeAccountService{
		account: &model.Account{ID: accountID, UserID: userID, Balance: 950},
	}
	handler := newTestHandler(accounts, &fakeCategoryService{}, &fakeTransactionService{}, nil)

	recorder := doRequest(t, handler, http.MethodPost, "/api/accounts/"+accountID.String()+"/adjustments", signToken(t, testJWTSecret, userID),
		`{"target_balance":950,"description":"Bank statement"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d with body %q", recorder.Code, recorder.Body.String())
	}
	if accounts.lastID != accountID {
		t.Fatalf("expected account id %s, got %s", accountID, accounts.lastID)
	}
	if accounts.lastUserID != userID {
		t.Fatalf("expected caller %s forwarded, got %s", userID, accounts.lastUserID)
	}
	if accounts.lastAdjust.TargetBalance != 950 || accounts.lastAdjust.Description != "Bank statement" {
		t.Fatalf("unexpected adjustment request: %+v", accounts.lastAdjust)
	}
}

func TestDeleteAccount(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	accountID := uuid.New()
	accounts := &fakeAccountService{}
	handler := newTestHandler(accounts, &fakeCategoryService{}, &fakeTransactionService{}, nil)

	recorder := doRequest(t, handler, http.MethodDelete, "/api/accounts/"+accountID.String(), signToken(t, testJWTSecret, userID), "")
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d with body %q", recorder.Code, recorder.Body.String())
	}
	if accounts.lastID != accountID {
		t.Fatalf("expected account id %s, got %s", accountID, accounts.lastID)
	}
	if accounts.lastUserID != userID {
		t.Fatalf("expected caller %s forwarded to DeleteAccount, got %s", userID, accounts.lastUserID)
	}
}

func TestInvalidAccountIDReturnsBadRequest(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(&fakeAccountService{}, &fakeCategoryService{}, &fakeTransactionService{}, nil)

	recorder := doRequest(t, handler, http.MethodDelete, "/api/accounts/not-a-uuid", signToken(t, testJWTSecret, uuid.New()), "")
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d with body %q", recorder.Code, recorder.Body.String())
	}
}

func TestListCategoriesReturnsCategories(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	categories := &fakeCategoryService{categories: []model.Category{
		{ID: uuid.New(), UserID: userID, Name: "Salary", Type: service.TypeIncome, Color: "#22C55E"},
	}}
	handler := newTestHandler(&fakeAccountService{}, categories, &fakeTransactionService{}, nil)

	recorder := doRequest(t, handler, http.MethodGet, "/api/categories", signToken(t, testJWTSecret, userID), "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d with body %q", recorder.Code, recorder.Body.String())
	}

	var response []model.Category
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected JSON array, got error: %v", err)
	}
	if len(response) != 1 {
		t.Fatalf("expected 1 category, got %d", len(response))
	}
}

func TestCreateCategory(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	categories := &fakeCategoryService{}
	handler := newTestHandler(&fakeAccountService{}, categories, &fakeTransactionService{}, nil)

	recorder := doRequest(t, handler, http.MethodPost, "/api/categories", signToken(t, testJWTSecret, userID),
		`{"name":"Groceries","type":"expense","color":"#FB923C"}`)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d with body %q", recorder.Code, recorder.Body.String())
	}
	if categories.lastCreate.Type != service.TypeExpense {
		t.Fatalf("expected type %q, got %q", service.TypeExpense, categories.lastCreate.Type)
	}
}

func TestDeleteCategory(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	categoryID := uuid.New()
	categories := &fakeCategoryService{}
	handler := newTestHandler(&fakeAccountService{}, categories, &fakeTransactionService{}, nil)

	recorder := doRequest(t, handler, http.MethodDelete, "/api/categories/"+categoryID.String(), signToken(t, testJWTSecret, userID), "")
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d with body %q", recorder.Code, recorder.Body.String())
	}
	if categories.lastID != categoryID {
		t.Fatalf("expected category id %s, got %s", categoryID, categories.lastID)
	}
	if categories.lastUserID != userID {
		t.Fatalf("expected caller %s forwarded to DeleteCategory, got %s", userID, categories.lastUserID)
	}
}

func TestListTransactionsParsesFilters(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	accountID := uuid.New()
	categoryID := uuid.New()
	transactions := &fakeTransactionService{paginated: &model.PaginatedTransactions{
		Data:     []model.Transaction{{ID: uuid.New()}},
		Total:    1,
		Page:     2,
		PageSize: 5,
	}}
	handler := newTestHandler(&fakeAccountService{}, &fakeCategoryService{}, transactions, nil)

	path := "/api/transactions?account_id=" + accountID.String() +
		"&category_id=" + categoryID.String() +
		"&type=expense&date_from=2026-01-01&date_to=2026-01-31&page=2&page_size=5"

	recorder := doRequest(t, handler, http.MethodGet, path, signToken(t, testJWTSecret, userID), "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d with body %q", recorder.Code, recorder.Body.String())
	}

	filters := transactions.lastFilters
	if filters.AccountID == nil || *filters.AccountID != accountID {
		t.Fatalf("expected account id %s, got %v", accountID, filters.AccountID)
	}
	if filters.CategoryID == nil || *filters.CategoryID != categoryID {
		t.Fatalf("expected category id %s, got %v", categoryID, filters.CategoryID)
	}
	if filters.Type != service.TypeExpense {
		t.Fatalf("expected type %q, got %q", service.TypeExpense, filters.Type)
	}
	if filters.DateFrom == nil || filters.DateFrom.Format("2006-01-02") != "2026-01-01" {
		t.Fatalf("expected date_from 2026-01-01, got %v", filters.DateFrom)
	}
	if filters.DateTo == nil || filters.DateTo.Format("2006-01-02") != "2026-01-31" {
		t.Fatalf("expected date_to 2026-01-31, got %v", filters.DateTo)
	}
	if filters.Page != 2 || filters.PageSize != 5 {
		t.Fatalf("expected page 2 size 5, got page %d size %d", filters.Page, filters.PageSize)
	}
	if transactions.lastUserID != userID {
		t.Fatalf("expected user id %s, got %s", userID, transactions.lastUserID)
	}
}

func TestListTransactionsRejectsInvalidFilters(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(&fakeAccountService{}, &fakeCategoryService{}, &fakeTransactionService{}, nil)
	token := signToken(t, testJWTSecret, uuid.New())

	tests := []string{
		"/api/transactions?account_id=not-a-uuid",
		"/api/transactions?category_id=not-a-uuid",
		"/api/transactions?date_from=not-a-date",
		"/api/transactions?date_to=not-a-date",
		"/api/transactions?page=0",
		"/api/transactions?page_size=0",
	}

	for _, path := range tests {
		path := path
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			recorder := doRequest(t, handler, http.MethodGet, path, token, "")
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("expected status 400, got %d with body %q", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestCreateTransaction(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	accountID := uuid.New()
	categoryID := uuid.New()
	transactions := &fakeTransactionService{}
	handler := newTestHandler(&fakeAccountService{}, &fakeCategoryService{}, transactions, nil)

	body := `{"account_id":"` + accountID.String() + `","category_id":"` + categoryID.String() +
		`","type":"expense","amount":42.5,"description":"Groceries","date":"2026-10-25T00:00:00Z"}`

	recorder := doRequest(t, handler, http.MethodPost, "/api/transactions", signToken(t, testJWTSecret, userID), body)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d with body %q", recorder.Code, recorder.Body.String())
	}
	if transactions.lastCreate.AccountID != accountID {
		t.Fatalf("expected account id %s, got %s", accountID, transactions.lastCreate.AccountID)
	}
	if transactions.lastCreate.Amount != 42.5 {
		t.Fatalf("expected amount 42.5, got %v", transactions.lastCreate.Amount)
	}
}

func TestUpdateTransaction(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	transactionID := uuid.New()
	transactions := &fakeTransactionService{}
	handler := newTestHandler(&fakeAccountService{}, &fakeCategoryService{}, transactions, nil)

	body := `{"amount":42.5,"description":"Groceries","date":"2026-10-25T00:00:00Z"}`

	recorder := doRequest(t, handler, http.MethodPatch, "/api/transactions/"+transactionID.String(), signToken(t, testJWTSecret, userID), body)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d with body %q", recorder.Code, recorder.Body.String())
	}
	if transactions.lastID != transactionID {
		t.Fatalf("expected transaction id %s, got %s", transactionID, transactions.lastID)
	}
	if transactions.lastUserID != userID {
		t.Fatalf("expected caller %s forwarded to UpdateTransaction, got %s", userID, transactions.lastUserID)
	}
	if transactions.lastUpdate.Amount == nil || *transactions.lastUpdate.Amount != 42.5 {
		t.Fatalf("expected amount 42.5, got %v", transactions.lastUpdate.Amount)
	}
	if transactions.lastUpdate.Description == nil || *transactions.lastUpdate.Description != "Groceries" {
		t.Fatalf("expected description %q, got %v", "Groceries", transactions.lastUpdate.Description)
	}
	if transactions.lastUpdate.Date == nil {
		t.Fatal("expected date to be decoded")
	}
	if transactions.lastUpdate.AccountID != nil || transactions.lastUpdate.Type != nil {
		t.Fatal("expected omitted fields to stay nil")
	}
}

func doMultipartRequest(t *testing.T, handler http.Handler, path, token, csvBody string, confirm bool) *httptest.ResponseRecorder {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", "transactions.csv")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte(csvBody)); err != nil {
		t.Fatalf("write csv body: %v", err)
	}
	if confirm {
		if err := writer.WriteField("confirm", "true"); err != nil {
			t.Fatalf("write confirm field: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, path, body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func TestPreviewImportTransactions(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	transactions := &fakeTransactionService{
		importResult: &model.ImportResult{
			Total:   1,
			Valid:   1,
			Invalid: 0,
			Errors:  []model.ImportRowError{},
			Preview: []model.ImportPreviewRow{{Row: 2, Date: "2026-10-01", Type: "expense", Amount: 12.5, Account: "Cash", Category: "Food"}},
		},
	}
	imports := &fakeImportService{result: transactions.importResult}
	handler := newTestHandlerWithImports(&fakeAccountService{}, &fakeCategoryService{}, transactions, imports, nil)

	recorder := doMultipartRequest(t, handler, "/api/transactions/import/preview", signToken(t, testJWTSecret, userID),
		"date,type,amount,description,account,category\n2026-10-01,expense,12.50,Coffee,Cash,Food\n", false)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d with body %q", recorder.Code, recorder.Body.String())
	}

	var result model.ImportResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if result.Valid != 1 || len(result.Preview) != 1 {
		t.Fatalf("unexpected import preview: %+v", result)
	}
	if imports.lastUserID != userID {
		t.Fatalf("expected caller %s forwarded, got %s", userID, imports.lastUserID)
	}
}

func TestImportTransactionsRequiresConfirm(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	handler := newTestHandler(&fakeAccountService{}, &fakeCategoryService{}, &fakeTransactionService{}, nil)

	recorder := doMultipartRequest(t, handler, "/api/transactions/import", signToken(t, testJWTSecret, userID),
		"date,type,amount,description,account,category\n", false)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 without confirm, got %d with body %q", recorder.Code, recorder.Body.String())
	}
}

func TestPreviewImportUsesAIImportService(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	imports := &fakeImportService{result: &model.ImportResult{Total: 1, Valid: 1, Mode: "ai", Sheet: "JANUARI", Sheets: []string{"JANUARI"}}}
	handler := newTestHandlerWithImports(&fakeAccountService{}, &fakeCategoryService{}, &fakeTransactionService{}, imports, nil)

	recorder := doMultipartRequest(t, handler, "/api/transactions/import/preview", signToken(t, testJWTSecret, userID),
		"date,type,amount,description,account,category\n2026-10-01,expense,12.50,Coffee,Cash,Food\n", false)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %q", recorder.Code, recorder.Body.String())
	}
	if imports.lastOptions.Mode != service.ImportModeAI || imports.lastOptions.Filename != "transactions.csv" {
		t.Fatalf("unexpected options: %+v", imports.lastOptions)
	}
}

func TestPreviewImportMapsAIUnavailableTo502(t *testing.T) {
	t.Parallel()

	imports := &fakeImportService{err: fmt.Errorf("%w: timeout", service.ErrImportAIUnavailable)}
	handler := newTestHandlerWithImports(&fakeAccountService{}, &fakeCategoryService{}, &fakeTransactionService{}, imports, nil)

	recorder := doMultipartRequest(t, handler, "/api/transactions/import/preview", signToken(t, testJWTSecret, uuid.New()),
		"date,type,amount,description,account,category\n", false)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d with body %q", recorder.Code, recorder.Body.String())
	}
}

func TestPreviewImportStrictConfigIsNotRateLimited(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	imports := &fakeImportService{result: &model.ImportResult{Total: 1, Valid: 1, Mode: "strict"}}
	handler := newTestHandlerWithImportMode(imports, service.ImportModeStrict, nil)

	token := signToken(t, testJWTSecret, userID)
	for i := 0; i < 21; i++ {
		recorder := doMultipartRequest(t, handler, "/api/transactions/import/preview", token,
			"date,type,amount,description,account,category\n2026-10-01,expense,12.50,Coffee,Cash,Food\n", false)
		if recorder.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d with body %q", i+1, recorder.Code, recorder.Body.String())
		}
	}
}

func TestImportRejectsOversizedMapping(t *testing.T) {
	t.Parallel()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	filePart, err := writer.CreateFormFile("file", "transactions.csv")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := filePart.Write([]byte("date,type,amount,description,account,category\n")); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := writer.WriteField("confirm", "true"); err != nil {
		t.Fatalf("write confirm: %v", err)
	}
	if err := writer.WriteField("mapping", strings.Repeat("x", service.MaxMappingBytes+1)); err != nil {
		t.Fatalf("write mapping: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/transactions/import", body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("Authorization", "Bearer "+signToken(t, testJWTSecret, uuid.New()))

	recorder := httptest.NewRecorder()
	newTestHandler(&fakeAccountService{}, &fakeCategoryService{}, &fakeTransactionService{}, nil).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d with body %q", recorder.Code, recorder.Body.String())
	}
}

func TestExportTransactions(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	transactions := &fakeTransactionService{
		exportRows: []model.TransactionExportRow{
			{
				Date:         time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
				Type:         "expense",
				Amount:       12.5,
				Description:  "Coffee, with milk",
				AccountName:  "Cash",
				CategoryName: "Food",
			},
		},
	}
	handler := newTestHandler(&fakeAccountService{}, &fakeCategoryService{}, transactions, nil)

	recorder := doRequest(t, handler, http.MethodGet, "/api/transactions/export.csv?type=expense", signToken(t, testJWTSecret, userID), "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d with body %q", recorder.Code, recorder.Body.String())
	}
	if contentType := recorder.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/csv") {
		t.Fatalf("expected text/csv content type, got %q", contentType)
	}
	if disposition := recorder.Header().Get("Content-Disposition"); !strings.Contains(disposition, "transactions-") {
		t.Fatalf("expected attachment filename, got %q", disposition)
	}

	want := "date,type,amount,description,account,category\n2026-10-01,expense,12.50,\"Coffee, with milk\",Cash,Food\n"
	if recorder.Body.String() != want {
		t.Fatalf("unexpected csv body:\nwant %q\ngot  %q", want, recorder.Body.String())
	}
	if transactions.lastUserID != userID {
		t.Fatalf("expected caller %s forwarded, got %s", userID, transactions.lastUserID)
	}
	if transactions.lastFilters.Type != "expense" {
		t.Fatalf("expected type filter %q, got %q", "expense", transactions.lastFilters.Type)
	}
}

func TestDeleteTransaction(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	transactionID := uuid.New()
	transactions := &fakeTransactionService{}
	handler := newTestHandler(&fakeAccountService{}, &fakeCategoryService{}, transactions, nil)

	recorder := doRequest(t, handler, http.MethodDelete, "/api/transactions/"+transactionID.String(), signToken(t, testJWTSecret, userID), "")
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d with body %q", recorder.Code, recorder.Body.String())
	}
	if transactions.lastID != transactionID {
		t.Fatalf("expected transaction id %s, got %s", transactionID, transactions.lastID)
	}
	if transactions.lastUserID != userID {
		t.Fatalf("expected caller %s forwarded to DeleteTransaction, got %s", userID, transactions.lastUserID)
	}
}

func TestDashboardSummary(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	transactions := &fakeTransactionService{summary: &model.DashboardSummary{
		Balances: []model.CurrencyTotal{
			{Currency: "USD", Total: 300.5},
		},
		Monthly: []model.MonthlyCurrencyTotal{
			{Currency: "USD", Income: 1000, Expense: 250, Net: 750},
		},
		TransactionCount:   42,
		RecentTransactions: []model.Transaction{{ID: uuid.New()}},
	}}
	handler := newTestHandler(&fakeAccountService{}, &fakeCategoryService{}, transactions, nil)

	recorder := doRequest(t, handler, http.MethodGet, "/api/dashboard/summary", signToken(t, testJWTSecret, userID), "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d with body %q", recorder.Code, recorder.Body.String())
	}

	var response model.DashboardSummary
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected JSON response, got error: %v", err)
	}
	if len(response.Balances) != 1 || response.Balances[0].Total != 300.5 {
		t.Fatalf("expected USD balance 300.5, got %+v", response.Balances)
	}
	if len(response.Monthly) != 1 || response.Monthly[0].Net != 750 {
		t.Fatalf("expected monthly net 750, got %+v", response.Monthly)
	}
	if response.TransactionCount != 42 {
		t.Fatalf("expected transaction count 42, got %d", response.TransactionCount)
	}
	if len(response.RecentTransactions) != 1 {
		t.Fatalf("expected 1 recent transaction, got %d", len(response.RecentTransactions))
	}
}

func TestServiceErrorsMapToHTTPStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		err    error
		status int
	}{
		{"account not found", service.ErrAccountNotFound, http.StatusNotFound},
		{"category not found", service.ErrCategoryNotFound, http.StatusNotFound},
		{"transaction not found", service.ErrTransactionNotFound, http.StatusNotFound},
		{"account name exists", service.ErrAccountNameExists, http.StatusConflict},
		{"category name exists", service.ErrCategoryNameExists, http.StatusConflict},
		{"invalid amount", service.ErrInvalidAmount, http.StatusBadRequest},
		{"invalid type", service.ErrInvalidType, http.StatusBadRequest},
		{"invalid balance", service.ErrInvalidBalance, http.StatusBadRequest},
		{"use adjustment", service.ErrUseAdjustmentEndpoint, http.StatusBadRequest},
		{"currency locked", service.ErrCurrencyLocked, http.StatusConflict},
		{"system entry", service.ErrSystemEntryImmutable, http.StatusConflict},
		{"invalid name", service.ErrInvalidName, http.StatusBadRequest},
		{"invalid currency", service.ErrInvalidCurrency, http.StatusBadRequest},
		{"internal error", io.ErrUnexpectedEOF, http.StatusInternalServerError},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			accounts := &fakeAccountService{err: test.err}
			handler := newTestHandler(accounts, &fakeCategoryService{}, &fakeTransactionService{}, nil)

			recorder := doRequest(t, handler, http.MethodGet, "/api/accounts", signToken(t, testJWTSecret, uuid.New()), "")
			if recorder.Code != test.status {
				t.Fatalf("expected status %d, got %d with body %q", test.status, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestCORSPreflightAllowsConfiguredOrigin(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(&fakeAccountService{}, &fakeCategoryService{}, &fakeTransactionService{}, []string{"http://localhost:5173"})

	request := httptest.NewRequest(http.MethodOptions, "/api/accounts", nil)
	request.Header.Set("Origin", "http://localhost:5173")
	request.Header.Set("Access-Control-Request-Method", http.MethodGet)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d with body %q", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("expected allow origin %q, got %q", "http://localhost:5173", got)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(got, http.MethodPatch) {
		t.Fatalf("expected allow methods to include PATCH, got %q", got)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Headers"); !strings.Contains(got, "Authorization") {
		t.Fatalf("expected allow headers to include Authorization, got %q", got)
	}
}

func TestCORSRejectsUnconfiguredOrigin(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(&fakeAccountService{}, &fakeCategoryService{}, &fakeTransactionService{}, []string{"http://localhost:5173"})

	request := httptest.NewRequest(http.MethodGet, "/api/accounts", nil)
	request.Header.Set("Origin", "http://evil.example.com")
	request.Header.Set("Authorization", "Bearer "+signToken(t, testJWTSecret, uuid.New()))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("expected no allow origin header, got %q", got)
	}
}

func TestCORSAllowsAllOriginsWhenWildcardConfigured(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(&fakeAccountService{}, &fakeCategoryService{}, &fakeTransactionService{}, []string{"*"})

	request := httptest.NewRequest(http.MethodGet, "/api/accounts", nil)
	request.Header.Set("Origin", "http://anywhere.example.com")
	request.Header.Set("Authorization", "Bearer "+signToken(t, testJWTSecret, uuid.New()))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("expected allow origin %q, got %q", "*", got)
	}
}

func doMultipartUpload(t *testing.T, handler http.Handler, path, token, filename string, content []byte, fields map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write file body: %v", err)
	}
	for key, value := range fields {
		if err := writer.WriteField(key, value); err != nil {
			t.Fatalf("write field %s: %v", key, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, path, body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func TestImportTransactionsForwardsMappingAndMode(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	imports := &fakeImportService{result: &model.ImportResult{Total: 1, Valid: 1, Mode: "ai"}}
	handler := newTestHandlerWithImports(&fakeAccountService{}, &fakeCategoryService{}, &fakeTransactionService{}, imports, nil)

	mapping := `{"sheet":"transactions.csv","header_row":1,"data_start":2,"data_end":2,"fields":{"date":{"column":1},"amount":{"column":3},"description":{"column":4},"account":{"column":5},"category":{"column":6}},"type":{"column":2}}`
	recorder := doMultipartUpload(t, handler, "/api/transactions/import", signToken(t, testJWTSecret, userID), "transactions.csv",
		[]byte("date,type,amount,description,account,category\n2026-10-01,expense,12.50,Coffee,Cash,Food\n"),
		map[string]string{"confirm": "true", "mode": "ai", "mapping": mapping})

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %q", recorder.Code, recorder.Body.String())
	}
	if imports.lastOptions.Mode != service.ImportModeAI {
		t.Fatalf("expected AI mode, got %q", imports.lastOptions.Mode)
	}
	if imports.lastOptions.Mapping == nil || imports.lastOptions.Mapping.Fields.Date.Column != 1 || imports.lastOptions.Mapping.Fields.Amount.Column != 3 {
		t.Fatalf("expected the mapping to be forwarded, got %+v", imports.lastOptions.Mapping)
	}
}

func TestImportTransactionsHonorsStrictMode(t *testing.T) {
	t.Parallel()

	imports := &fakeImportService{result: &model.ImportResult{Total: 1, Valid: 1, Mode: "strict"}}
	handler := newTestHandlerWithImports(&fakeAccountService{}, &fakeCategoryService{}, &fakeTransactionService{}, imports, nil)

	recorder := doMultipartUpload(t, handler, "/api/transactions/import", signToken(t, testJWTSecret, uuid.New()), "transactions.csv",
		[]byte("date,type,amount,description,account,category\n2026-10-01,expense,12.50,Coffee,Cash,Food\n"),
		map[string]string{"confirm": "true", "mode": "strict"})

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %q", recorder.Code, recorder.Body.String())
	}
	if imports.lastOptions.Mode != service.ImportModeStrict {
		t.Fatalf("expected strict mode, got %q", imports.lastOptions.Mode)
	}
}

func TestImportTransactionsForwardsAllowDuplicates(t *testing.T) {
	t.Parallel()

	imports := &fakeImportService{result: &model.ImportResult{Total: 1, Valid: 1, Mode: "strict"}}
	handler := newTestHandlerWithImports(&fakeAccountService{}, &fakeCategoryService{}, &fakeTransactionService{}, imports, nil)

	recorder := doMultipartUpload(t, handler, "/api/transactions/import", signToken(t, testJWTSecret, uuid.New()), "transactions.csv",
		[]byte("date,type,amount,description,account,category\n2026-10-01,expense,12.50,Coffee,Cash,Food\n"),
		map[string]string{"confirm": "true", "mode": "strict", "allow_duplicates": "true"})

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %q", recorder.Code, recorder.Body.String())
	}
	if !imports.lastOptions.AllowDuplicates {
		t.Fatalf("expected allow_duplicates to be forwarded, got %+v", imports.lastOptions)
	}
}

func TestPreviewImportMapsMissingColumnsTo400(t *testing.T) {
	t.Parallel()

	imports := &fakeImportService{err: fmt.Errorf("%w: missing column \"date\"", service.ErrImportMissingColumns)}
	handler := newTestHandlerWithImports(&fakeAccountService{}, &fakeCategoryService{}, &fakeTransactionService{}, imports, nil)

	recorder := doMultipartUpload(t, handler, "/api/transactions/import/preview", signToken(t, testJWTSecret, uuid.New()), "transactions.csv",
		[]byte("pocket,total\nSaving,100\n"), nil)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d with body %q", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "missing column") {
		t.Fatalf("expected the missing column message, got %q", recorder.Body.String())
	}
}

func TestPreviewImportRateLimitReturns429(t *testing.T) {
	t.Parallel()

	imports := &fakeImportService{result: &model.ImportResult{Total: 1, Valid: 1, Mode: "ai"}}
	handler := newTestHandlerWithImportMode(imports, service.ImportModeAI, nil)
	token := signToken(t, testJWTSecret, uuid.New())

	for i := 0; i < 20; i++ {
		recorder := doMultipartUpload(t, handler, "/api/transactions/import/preview", token, "transactions.csv",
			[]byte("date,type,amount,description,account,category\n2026-10-01,expense,12.50,Coffee,Cash,Food\n"), nil)
		if recorder.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d with body %q", i+1, recorder.Code, recorder.Body.String())
		}
	}

	recorder := doMultipartUpload(t, handler, "/api/transactions/import/preview", token, "transactions.csv",
		[]byte("date,type,amount,description,account,category\n2026-10-01,expense,12.50,Coffee,Cash,Food\n"), nil)
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 on the 21st request, got %d with body %q", recorder.Code, recorder.Body.String())
	}
}

func TestPreviewImportParsesXLSXThroughHandler(t *testing.T) {
	t.Parallel()

	workbook := excelize.NewFile()
	sheet := "February"
	workbook.SetSheetName("Sheet1", sheet)
	rows := [][]any{
		{"Date", "Type", "Amount", "Description", "Account", "Category"},
		{"2026-02-01", "expense", 9.5, "Lunch", "Cash", "Food"},
	}
	for index, row := range rows {
		cell, err := excelize.CoordinatesToCellName(1, index+1)
		if err != nil {
			t.Fatalf("coordinates: %v", err)
		}
		if err := workbook.SetSheetRow(sheet, cell, &row); err != nil {
			t.Fatalf("set sheet row: %v", err)
		}
	}
	content, err := workbook.WriteToBuffer()
	if err != nil {
		t.Fatalf("write workbook: %v", err)
	}

	imports := &fakeImportService{result: &model.ImportResult{Total: 1, Valid: 1, Mode: "ai", Sheet: sheet, Sheets: []string{sheet}}}
	handler := newTestHandlerWithImports(&fakeAccountService{}, &fakeCategoryService{}, &fakeTransactionService{}, imports, nil)

	recorder := doMultipartUpload(t, handler, "/api/transactions/import/preview", signToken(t, testJWTSecret, uuid.New()), "transactions.xlsx",
		content.Bytes(), map[string]string{"sheet": sheet})

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %q", recorder.Code, recorder.Body.String())
	}
	if imports.lastTable == nil || imports.lastTable.Name != sheet || len(imports.lastTable.Rows) != 2 {
		t.Fatalf("expected the XLSX sheet to be parsed, got %+v", imports.lastTable)
	}
	if len(imports.lastSheets) != 1 || imports.lastSheets[0] != sheet {
		t.Fatalf("expected the sheet list, got %v", imports.lastSheets)
	}
}

func TestUpdateCategoryRenames(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	categoryID := uuid.New()
	categories := &fakeCategoryService{category: &model.Category{ID: categoryID, UserID: userID, Name: "Groceries"}}
	handler := newTestHandler(&fakeAccountService{}, categories, &fakeTransactionService{}, nil)

	recorder := doRequest(t, handler, http.MethodPatch, "/api/categories/"+categoryID.String(), signToken(t, testJWTSecret, userID),
		`{"name":"Dining"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %q", recorder.Code, recorder.Body.String())
	}
	if categories.lastUserID != userID || categories.lastID != categoryID {
		t.Fatalf("expected update scoped to user %s category %s", userID, categoryID)
	}
	if categories.lastUpdate.Name == nil || *categories.lastUpdate.Name != "Dining" {
		t.Fatalf("expected name %q, got %v", "Dining", categories.lastUpdate.Name)
	}
}

func TestUpdateCategoryNotFound(t *testing.T) {
	t.Parallel()

	categories := &fakeCategoryService{updateErr: service.ErrCategoryNotFound}
	handler := newTestHandler(&fakeAccountService{}, categories, &fakeTransactionService{}, nil)

	recorder := doRequest(t, handler, http.MethodPatch, "/api/categories/"+uuid.New().String(), signToken(t, testJWTSecret, uuid.New()),
		`{"name":"Dining"}`)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d with body %q", recorder.Code, recorder.Body.String())
	}
}

func TestUpdateCategoryRejectsTypeChange(t *testing.T) {
	t.Parallel()

	categories := &fakeCategoryService{}
	handler := newTestHandler(&fakeAccountService{}, categories, &fakeTransactionService{}, nil)

	recorder := doRequest(t, handler, http.MethodPatch, "/api/categories/"+uuid.New().String(), signToken(t, testJWTSecret, uuid.New()),
		`{"type":"income"}`)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d with body %q", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "type") {
		t.Fatalf("expected the error to mention type, got %q", recorder.Body.String())
	}
}

func TestUpdateCategoryNameConflictIs409(t *testing.T) {
	t.Parallel()

	categories := &fakeCategoryService{updateErr: service.ErrCategoryNameExists}
	handler := newTestHandler(&fakeAccountService{}, categories, &fakeTransactionService{}, nil)

	recorder := doRequest(t, handler, http.MethodPatch, "/api/categories/"+uuid.New().String(), signToken(t, testJWTSecret, uuid.New()),
		`{"name":"Dining"}`)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d with body %q", recorder.Code, recorder.Body.String())
	}
}

func TestDeleteCategoryInUseIs409(t *testing.T) {
	t.Parallel()

	categories := &fakeCategoryService{err: service.ErrCategoryInUse}
	handler := newTestHandler(&fakeAccountService{}, categories, &fakeTransactionService{}, nil)

	recorder := doRequest(t, handler, http.MethodDelete, "/api/categories/"+uuid.New().String(), signToken(t, testJWTSecret, uuid.New()), "")
	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d with body %q", recorder.Code, recorder.Body.String())
	}
}

func TestDeleteUnusedCategorySucceeds(t *testing.T) {
	t.Parallel()

	categoryID := uuid.New()
	categories := &fakeCategoryService{}
	handler := newTestHandler(&fakeAccountService{}, categories, &fakeTransactionService{}, nil)

	recorder := doRequest(t, handler, http.MethodDelete, "/api/categories/"+categoryID.String(), signToken(t, testJWTSecret, uuid.New()), "")
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d with body %q", recorder.Code, recorder.Body.String())
	}
	if categories.lastID != categoryID {
		t.Fatalf("expected category %s to be deleted, got %s", categoryID, categories.lastID)
	}
}

func TestCreateTransfer(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	fromID := uuid.New()
	toID := uuid.New()
	groupID := uuid.New()
	transactions := &fakeTransactionService{transfer: &model.TransferResult{
		TransferGroupID: groupID,
		From:            model.Transaction{AccountID: fromID, Type: service.TypeExpense, Amount: 100},
		To:              model.Transaction{AccountID: toID, Type: service.TypeIncome, Amount: 100},
	}}
	handler := newTestHandler(&fakeAccountService{}, &fakeCategoryService{}, transactions, nil)

	recorder := doRequest(t, handler, http.MethodPost, "/api/transfers", signToken(t, testJWTSecret, userID),
		`{"from_account_id":"`+fromID.String()+`","to_account_id":"`+toID.String()+`","amount":100,"date":"2026-11-08"}`)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d with body %q", recorder.Code, recorder.Body.String())
	}
	if transactions.lastUserID != userID {
		t.Fatalf("expected caller %s forwarded, got %s", userID, transactions.lastUserID)
	}
	if transactions.lastTransfer.FromAccountID != fromID || transactions.lastTransfer.ToAccountID != toID {
		t.Fatalf("unexpected transfer request: %+v", transactions.lastTransfer)
	}

	var response model.TransferResult
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("expected JSON transfer result, got error: %v", err)
	}
	if response.TransferGroupID != groupID {
		t.Fatalf("expected group id %s, got %s", groupID, response.TransferGroupID)
	}
}

func TestDeleteTransfer(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	groupID := uuid.New()
	transactions := &fakeTransactionService{}
	handler := newTestHandler(&fakeAccountService{}, &fakeCategoryService{}, transactions, nil)

	recorder := doRequest(t, handler, http.MethodDelete, "/api/transfers/"+groupID.String(), signToken(t, testJWTSecret, userID), "")
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d with body %q", recorder.Code, recorder.Body.String())
	}
	if transactions.lastTransferGroup != groupID {
		t.Fatalf("expected group id %s forwarded, got %s", groupID, transactions.lastTransferGroup)
	}
	if transactions.lastUserID != userID {
		t.Fatalf("expected caller %s forwarded, got %s", userID, transactions.lastUserID)
	}
}

func TestCreateTransferMapsErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		err    error
		status int
	}{
		{"same account", service.ErrSameAccount, http.StatusBadRequest},
		{"currency mismatch", service.ErrTransferCurrencyMismatch, http.StatusBadRequest},
		{"invalid amount", service.ErrInvalidAmount, http.StatusBadRequest},
		{"invalid date", service.ErrInvalidDate, http.StatusBadRequest},
		{"unknown account", service.ErrAccountNotFound, http.StatusNotFound},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			transactions := &fakeTransactionService{err: test.err}
			handler := newTestHandler(&fakeAccountService{}, &fakeCategoryService{}, transactions, nil)

			recorder := doRequest(t, handler, http.MethodPost, "/api/transfers", signToken(t, testJWTSecret, uuid.New()),
				`{"from_account_id":"`+uuid.New().String()+`","to_account_id":"`+uuid.New().String()+`","amount":100,"date":"2026-11-08"}`)
			if recorder.Code != test.status {
				t.Fatalf("expected status %d, got %d with body %q", test.status, recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestDeleteTransferNotFound(t *testing.T) {
	t.Parallel()

	transactions := &fakeTransactionService{err: service.ErrTransferNotFound}
	handler := newTestHandler(&fakeAccountService{}, &fakeCategoryService{}, transactions, nil)

	recorder := doRequest(t, handler, http.MethodDelete, "/api/transfers/"+uuid.New().String(), signToken(t, testJWTSecret, uuid.New()), "")
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d with body %q", recorder.Code, recorder.Body.String())
	}
}

func TestDeleteAccountWithTransferLegReturnsConflict(t *testing.T) {
	t.Parallel()

	accounts := &fakeAccountService{err: service.ErrAccountHasTransfers}
	handler := newTestHandler(accounts, &fakeCategoryService{}, &fakeTransactionService{}, nil)

	recorder := doRequest(t, handler, http.MethodDelete, "/api/accounts/"+uuid.New().String(), signToken(t, testJWTSecret, uuid.New()), "")
	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected status 409, got %d with body %q", recorder.Code, recorder.Body.String())
	}
}

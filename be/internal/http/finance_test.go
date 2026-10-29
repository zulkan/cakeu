package httpserver_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	httpserver "goskeleton/internal/http"
	"goskeleton/internal/model"
	"goskeleton/internal/service"
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
	return &model.Account{ID: uuid.New(), UserID: userID, Name: req.Name, Balance: req.Balance, Currency: req.Currency}, nil
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

func (f *fakeAccountService) UpdateAccount(ctx context.Context, id uuid.UUID, req model.UpdateAccountRequest) (*model.Account, error) {
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

func (f *fakeAccountService) DeleteAccount(ctx context.Context, id uuid.UUID) error {
	f.lastID = id
	return f.err
}

type fakeCategoryService struct {
	categories []model.Category
	category   *model.Category
	err        error
	lastUserID uuid.UUID
	lastID     uuid.UUID
	lastCreate model.CreateCategoryRequest
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

func (f *fakeCategoryService) DeleteCategory(ctx context.Context, id uuid.UUID) error {
	f.lastID = id
	return f.err
}

type fakeTransactionService struct {
	transaction *model.Transaction
	paginated   *model.PaginatedTransactions
	summary     *model.DashboardSummary
	err         error
	lastUserID  uuid.UUID
	lastID      uuid.UUID
	lastFilters model.TransactionFilters
	lastCreate  model.CreateTransactionRequest
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

func (f *fakeTransactionService) DeleteTransaction(ctx context.Context, id uuid.UUID) error {
	f.lastID = id
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

func newTestHandler(accounts *fakeAccountService, categories *fakeCategoryService, transactions *fakeTransactionService, origins []string) http.Handler {
	return httpserver.NewHandler(service.NewHealth(nil), nil, accounts, categories, transactions, testJWTSecret, origins)
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
		`{"name":"Checking","balance":100,"currency":"USD"}`)
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
	if accounts.lastUpdate.Name == nil || *accounts.lastUpdate.Name != "Updated" {
		t.Fatalf("expected name %q, got %v", "Updated", accounts.lastUpdate.Name)
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
}

func TestDashboardSummary(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	transactions := &fakeTransactionService{summary: &model.DashboardSummary{
		TotalBalance:       300.5,
		MonthlyIncome:      1000,
		MonthlyExpense:     250,
		MonthlyNet:         750,
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
	if response.TotalBalance != 300.5 {
		t.Fatalf("expected total balance 300.5, got %v", response.TotalBalance)
	}
	if response.MonthlyNet != 750 {
		t.Fatalf("expected monthly net 750, got %v", response.MonthlyNet)
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

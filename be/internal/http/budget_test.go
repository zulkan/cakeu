package httpserver_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	httpserver "goskeleton/internal/http"
	"goskeleton/internal/model"
	"goskeleton/internal/service"
)

type fakeBudgetRepoHTTP struct {
	budgets         []model.Budget
	upsertCurrency  string
	deleteCurrency  string
	spentByCurrency map[string]map[uuid.UUID]float64
}

func (f *fakeBudgetRepoHTTP) Upsert(ctx context.Context, userID, categoryID uuid.UUID, month time.Time, currency string, amount float64) (*model.Budget, error) {
	f.upsertCurrency = currency
	return &model.Budget{
		ID:         uuid.New(),
		UserID:     userID,
		CategoryID: categoryID,
		Month:      month,
		Currency:   currency,
		Amount:     amount,
	}, nil
}

func (f *fakeBudgetRepoHTTP) Delete(ctx context.Context, userID, categoryID uuid.UUID, month time.Time, currency string) error {
	f.deleteCurrency = currency
	return nil
}

func (f *fakeBudgetRepoHTTP) ListByMonth(ctx context.Context, userID uuid.UUID, month time.Time) ([]model.Budget, error) {
	return f.budgets, nil
}

func (f *fakeBudgetRepoHTTP) SpentByCategory(ctx context.Context, userID uuid.UUID, month time.Time, currency string) (map[uuid.UUID]float64, error) {
	return f.spentByCurrency[currency], nil
}

func newBudgetTestHandler(repo *fakeBudgetRepoHTTP) http.Handler {
	return httpserver.NewHandler(
		service.NewHealth(nil), nil, nil, nil, nil, nil, service.ImportModeAI,
		service.NewBudgetService(repo), nil, nil, nil, nil, testJWTSecret, nil,
	)
}

func TestBudgetListIncludesCurrency(t *testing.T) {
	t.Parallel()

	categoryID := uuid.New()
	repo := &fakeBudgetRepoHTTP{
		budgets: []model.Budget{
			{ID: uuid.New(), CategoryID: categoryID, CategoryName: "Food", Month: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), Currency: "USD", Amount: 200},
		},
		spentByCurrency: map[string]map[uuid.UUID]float64{
			"USD": {categoryID: 50},
		},
	}
	handler := newBudgetTestHandler(repo)
	token := signToken(t, testJWTSecret, uuid.New())

	recorder := doRequest(t, handler, http.MethodGet, "/api/budgets?month=2026-10", token, "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %q", recorder.Code, recorder.Body.String())
	}

	var statuses []model.BudgetStatus
	if err := json.Unmarshal(recorder.Body.Bytes(), &statuses); err != nil {
		t.Fatalf("expected JSON array, got error: %v", err)
	}
	if len(statuses) != 1 || statuses[0].Currency != "USD" {
		t.Fatalf("expected one USD budget status, got %+v", statuses)
	}
}

func TestUpsertBudgetRequiresCurrency(t *testing.T) {
	t.Parallel()

	handler := newBudgetTestHandler(&fakeBudgetRepoHTTP{})
	token := signToken(t, testJWTSecret, uuid.New())
	body := `{"category_id":"` + uuid.New().String() + `","month":"2026-10","amount":200}`

	recorder := doRequest(t, handler, http.MethodPut, "/api/budgets", token, body)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when currency is missing, got %d with body %q", recorder.Code, recorder.Body.String())
	}

	invalidBody := `{"category_id":"` + uuid.New().String() + `","month":"2026-10","currency":"US","amount":200}`
	recorder = doRequest(t, handler, http.MethodPut, "/api/budgets", token, invalidBody)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when currency is invalid, got %d with body %q", recorder.Code, recorder.Body.String())
	}
}

func TestUpsertBudgetNormalisesCurrency(t *testing.T) {
	t.Parallel()

	repo := &fakeBudgetRepoHTTP{}
	handler := newBudgetTestHandler(repo)
	token := signToken(t, testJWTSecret, uuid.New())
	body := `{"category_id":"` + uuid.New().String() + `","month":"2026-10","currency":" usd ","amount":200}`

	recorder := doRequest(t, handler, http.MethodPut, "/api/budgets", token, body)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %q", recorder.Code, recorder.Body.String())
	}
	if repo.upsertCurrency != "USD" {
		t.Fatalf("expected normalised USD to reach the repository, got %q", repo.upsertCurrency)
	}
}

func TestDeleteBudgetRequiresCurrency(t *testing.T) {
	t.Parallel()

	repo := &fakeBudgetRepoHTTP{}
	handler := newBudgetTestHandler(repo)
	token := signToken(t, testJWTSecret, uuid.New())
	path := "/api/budgets/" + uuid.New().String() + "?month=2026-10"

	recorder := doRequest(t, handler, http.MethodDelete, path, token, "")
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when currency is missing, got %d with body %q", recorder.Code, recorder.Body.String())
	}

	recorder = doRequest(t, handler, http.MethodDelete, path+"&currency=usd", token, "")
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d with body %q", recorder.Code, recorder.Body.String())
	}
	if repo.deleteCurrency != "USD" {
		t.Fatalf("expected normalised USD to reach the repository, got %q", repo.deleteCurrency)
	}
}

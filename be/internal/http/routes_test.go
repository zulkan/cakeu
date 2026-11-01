package httpserver_test

import (
	"net/http"
	"testing"
)

func TestNewRoutesRequireAuthentication(t *testing.T) {
	t.Parallel()

	handler := newTestHandler(&fakeAccountService{}, &fakeCategoryService{}, &fakeTransactionService{}, nil)

	cases := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/budgets"},
		{http.MethodPut, "/api/budgets"},
		{http.MethodDelete, "/api/budgets/11111111-1111-1111-1111-111111111111"},
		{http.MethodGet, "/api/recurring"},
		{http.MethodPost, "/api/recurring"},
		{http.MethodPatch, "/api/recurring/11111111-1111-1111-1111-111111111111"},
		{http.MethodDelete, "/api/recurring/11111111-1111-1111-1111-111111111111"},
		{http.MethodPost, "/api/recurring/11111111-1111-1111-1111-111111111111/run-now"},
		{http.MethodGet, "/api/reports/cashflow?from=2026-01-01&to=2026-02-01"},
		{http.MethodGet, "/api/reports/categories?from=2026-01-01&to=2026-02-01"},
		{http.MethodGet, "/api/reports/merchants?from=2026-01-01&to=2026-02-01"},
	}

	for _, testCase := range cases {
		recorder := doRequest(t, handler, testCase.method, testCase.path, "", "")
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s: expected 401, got %d", testCase.method, testCase.path, recorder.Code)
		}
	}
}

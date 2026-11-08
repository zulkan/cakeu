package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"

	"goskeleton/internal/model"
	"goskeleton/internal/service"
)

type BudgetService interface {
	List(ctx context.Context, userID uuid.UUID, month time.Time) ([]model.BudgetStatus, error)
	Upsert(ctx context.Context, userID, categoryID uuid.UUID, month time.Time, currency string, amount float64) (*model.Budget, error)
	Delete(ctx context.Context, userID, categoryID uuid.UUID, month time.Time, currency string) error
}

func HandleListBudgets(budgets BudgetService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := userIDFromContext(r.Context())
		if err != nil {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		month, ok := parseMonthQuery(w, r)
		if !ok {
			return
		}

		result, err := budgets.List(r.Context(), userID, month)
		if err != nil {
			respondBudgetError(w, err)
			return
		}

		respondJSON(w, http.StatusOK, result)
	}
}

func HandleUpsertBudget(budgets BudgetService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := userIDFromContext(r.Context())
		if err != nil {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		var req model.UpsertBudgetRequest
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if req.CategoryID == uuid.Nil {
			respondError(w, http.StatusBadRequest, "category_id is required")
			return
		}

		month, err := parseMonth(req.Month)
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid month")
			return
		}

		budget, err := budgets.Upsert(r.Context(), userID, req.CategoryID, month, req.Currency, req.Amount)
		if err != nil {
			respondBudgetError(w, err)
			return
		}

		respondJSON(w, http.StatusOK, budget)
	}
}

func HandleDeleteBudget(budgets BudgetService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := userIDFromContext(r.Context())
		if err != nil {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		categoryID, err := uuid.Parse(r.PathValue("category_id"))
		if err != nil {
			respondError(w, http.StatusBadRequest, "invalid category_id")
			return
		}

		month, ok := parseMonthQuery(w, r)
		if !ok {
			return
		}

		currency := r.URL.Query().Get("currency")

		if err := budgets.Delete(r.Context(), userID, categoryID, month, currency); err != nil {
			respondBudgetError(w, err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func parseMonthQuery(w http.ResponseWriter, r *http.Request) (time.Time, bool) {
	value := r.URL.Query().Get("month")
	if value == "" {
		now := time.Now().UTC()
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC), true
	}

	month, err := parseMonth(value)
	if err != nil {
		respondError(w, http.StatusBadRequest, "invalid month")
		return time.Time{}, false
	}

	return month, true
}

func parseMonth(value string) (time.Time, error) {
	return time.Parse("2006-01", value)
}

func respondBudgetError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrBudgetNotFound):
		respondError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrInvalidBudgetAmount),
		errors.Is(err, service.ErrInvalidMonth),
		errors.Is(err, service.ErrInvalidCurrency):
		respondError(w, http.StatusBadRequest, err.Error())
	default:
		respondError(w, http.StatusInternalServerError, "internal server error")
	}
}

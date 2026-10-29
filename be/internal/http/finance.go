package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"goskeleton/internal/model"
	"goskeleton/internal/service"
)

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

// ListAccounts godoc
// @Summary List accounts
// @Description List all accounts belonging to the authenticated user
// @Tags accounts
// @Produce json
// @Security BearerAuth
// @Success 200 {array} model.Account
// @Failure 401 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /api/accounts [get]
func handleListAccounts(accounts AccountService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := userIDFromContext(r.Context())
		if err != nil {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		result, err := accounts.ListAccounts(r.Context(), userID)
		if err != nil {
			respondServiceError(w, err)
			return
		}

		respondJSON(w, http.StatusOK, result)
	}
}

// CreateAccount godoc
// @Summary Create an account
// @Description Create a new account for the authenticated user
// @Tags accounts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body model.CreateAccountRequest true "Account request"
// @Success 201 {object} model.Account
// @Failure 400 {object} model.ErrorResponse
// @Failure 401 {object} model.ErrorResponse
// @Failure 409 {object} model.ErrorResponse
// @Router /api/accounts [post]
func handleCreateAccount(accounts AccountService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := userIDFromContext(r.Context())
		if err != nil {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		var req model.CreateAccountRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		account, err := accounts.CreateAccount(r.Context(), userID, req)
		if err != nil {
			respondServiceError(w, err)
			return
		}

		respondJSON(w, http.StatusCreated, account)
	}
}

// UpdateAccount godoc
// @Summary Update an account
// @Description Partially update an account
// @Tags accounts
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Account ID"
// @Param request body model.UpdateAccountRequest true "Account update request"
// @Success 200 {object} model.Account
// @Failure 400 {object} model.ErrorResponse
// @Failure 401 {object} model.ErrorResponse
// @Failure 404 {object} model.ErrorResponse
// @Failure 409 {object} model.ErrorResponse
// @Router /api/accounts/{id} [patch]
func handleUpdateAccount(accounts AccountService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseUUIDPath(w, r, "id")
		if !ok {
			return
		}

		var req model.UpdateAccountRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		account, err := accounts.UpdateAccount(r.Context(), id, req)
		if err != nil {
			respondServiceError(w, err)
			return
		}

		respondJSON(w, http.StatusOK, account)
	}
}

// DeleteAccount godoc
// @Summary Delete an account
// @Description Delete an account and its transactions
// @Tags accounts
// @Security BearerAuth
// @Param id path string true "Account ID"
// @Success 204
// @Failure 401 {object} model.ErrorResponse
// @Failure 404 {object} model.ErrorResponse
// @Router /api/accounts/{id} [delete]
func handleDeleteAccount(accounts AccountService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseUUIDPath(w, r, "id")
		if !ok {
			return
		}

		if err := accounts.DeleteAccount(r.Context(), id); err != nil {
			respondServiceError(w, err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// ListCategories godoc
// @Summary List categories
// @Description List all categories belonging to the authenticated user
// @Tags categories
// @Produce json
// @Security BearerAuth
// @Success 200 {array} model.Category
// @Failure 401 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /api/categories [get]
func handleListCategories(categories CategoryService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := userIDFromContext(r.Context())
		if err != nil {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		result, err := categories.ListCategories(r.Context(), userID)
		if err != nil {
			respondServiceError(w, err)
			return
		}

		respondJSON(w, http.StatusOK, result)
	}
}

// CreateCategory godoc
// @Summary Create a category
// @Description Create a new income or expense category
// @Tags categories
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body model.CreateCategoryRequest true "Category request"
// @Success 201 {object} model.Category
// @Failure 400 {object} model.ErrorResponse
// @Failure 401 {object} model.ErrorResponse
// @Failure 409 {object} model.ErrorResponse
// @Router /api/categories [post]
func handleCreateCategory(categories CategoryService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := userIDFromContext(r.Context())
		if err != nil {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		var req model.CreateCategoryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		category, err := categories.CreateCategory(r.Context(), userID, req)
		if err != nil {
			respondServiceError(w, err)
			return
		}

		respondJSON(w, http.StatusCreated, category)
	}
}

// DeleteCategory godoc
// @Summary Delete a category
// @Description Delete a category and its transactions
// @Tags categories
// @Security BearerAuth
// @Param id path string true "Category ID"
// @Success 204
// @Failure 401 {object} model.ErrorResponse
// @Failure 404 {object} model.ErrorResponse
// @Router /api/categories/{id} [delete]
func handleDeleteCategory(categories CategoryService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseUUIDPath(w, r, "id")
		if !ok {
			return
		}

		if err := categories.DeleteCategory(r.Context(), id); err != nil {
			respondServiceError(w, err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// ListTransactions godoc
// @Summary List transactions
// @Description List transactions with optional filters and pagination
// @Tags transactions
// @Produce json
// @Security BearerAuth
// @Param account_id query string false "Account ID"
// @Param category_id query string false "Category ID"
// @Param type query string false "Transaction type (income or expense)"
// @Param date_from query string false "Start date (YYYY-MM-DD)"
// @Param date_to query string false "End date (YYYY-MM-DD)"
// @Param page query int false "Page number"
// @Param page_size query int false "Page size"
// @Success 200 {object} model.PaginatedTransactions
// @Failure 400 {object} model.ErrorResponse
// @Failure 401 {object} model.ErrorResponse
// @Router /api/transactions [get]
func handleListTransactions(transactions TransactionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := userIDFromContext(r.Context())
		if err != nil {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		filters, err := parseTransactionFilters(r)
		if err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}

		result, err := transactions.ListTransactions(r.Context(), userID, filters)
		if err != nil {
			respondServiceError(w, err)
			return
		}

		respondJSON(w, http.StatusOK, result)
	}
}

// CreateTransaction godoc
// @Summary Create a transaction
// @Description Create a transaction and update the account balance
// @Tags transactions
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body model.CreateTransactionRequest true "Transaction request"
// @Success 201 {object} model.Transaction
// @Failure 400 {object} model.ErrorResponse
// @Failure 401 {object} model.ErrorResponse
// @Failure 404 {object} model.ErrorResponse
// @Router /api/transactions [post]
func handleCreateTransaction(transactions TransactionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := userIDFromContext(r.Context())
		if err != nil {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		var req model.CreateTransactionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		transaction, err := transactions.CreateTransaction(r.Context(), userID, req)
		if err != nil {
			respondServiceError(w, err)
			return
		}

		respondJSON(w, http.StatusCreated, transaction)
	}
}

// DeleteTransaction godoc
// @Summary Delete a transaction
// @Description Delete a transaction and restore the account balance
// @Tags transactions
// @Security BearerAuth
// @Param id path string true "Transaction ID"
// @Success 204
// @Failure 401 {object} model.ErrorResponse
// @Failure 404 {object} model.ErrorResponse
// @Router /api/transactions/{id} [delete]
func handleDeleteTransaction(transactions TransactionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseUUIDPath(w, r, "id")
		if !ok {
			return
		}

		if err := transactions.DeleteTransaction(r.Context(), id); err != nil {
			respondServiceError(w, err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// GetDashboardSummary godoc
// @Summary Get dashboard summary
// @Description Get total balance, monthly totals, transaction count and recent transactions
// @Tags dashboard
// @Produce json
// @Security BearerAuth
// @Success 200 {object} model.DashboardSummary
// @Failure 401 {object} model.ErrorResponse
// @Failure 500 {object} model.ErrorResponse
// @Router /api/dashboard/summary [get]
func handleGetDashboardSummary(transactions TransactionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := userIDFromContext(r.Context())
		if err != nil {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		summary, err := transactions.GetDashboardSummary(r.Context(), userID)
		if err != nil {
			respondServiceError(w, err)
			return
		}

		respondJSON(w, http.StatusOK, summary)
	}
}

func parseTransactionFilters(r *http.Request) (model.TransactionFilters, error) {
	var filters model.TransactionFilters
	query := r.URL.Query()

	if value := query.Get("account_id"); value != "" {
		id, err := uuid.Parse(value)
		if err != nil {
			return filters, errors.New("invalid account_id")
		}
		filters.AccountID = &id
	}

	if value := query.Get("category_id"); value != "" {
		id, err := uuid.Parse(value)
		if err != nil {
			return filters, errors.New("invalid category_id")
		}
		filters.CategoryID = &id
	}

	if value := query.Get("type"); value != "" {
		filters.Type = value
	}

	if value := query.Get("date_from"); value != "" {
		date, err := parseDate(value)
		if err != nil {
			return filters, errors.New("invalid date_from")
		}
		filters.DateFrom = &date
	}

	if value := query.Get("date_to"); value != "" {
		date, err := parseDate(value)
		if err != nil {
			return filters, errors.New("invalid date_to")
		}
		filters.DateTo = &date
	}

	if value := query.Get("page"); value != "" {
		page, err := strconv.Atoi(value)
		if err != nil || page < 1 {
			return filters, errors.New("invalid page")
		}
		filters.Page = page
	}

	if value := query.Get("page_size"); value != "" {
		pageSize, err := strconv.Atoi(value)
		if err != nil || pageSize < 1 {
			return filters, errors.New("invalid page_size")
		}
		filters.PageSize = pageSize
	}

	return filters, nil
}

func parseDate(value string) (time.Time, error) {
	if date, err := time.Parse("2006-01-02", value); err == nil {
		return date, nil
	}

	return time.Parse(time.RFC3339, value)
}

func parseUUIDPath(w http.ResponseWriter, r *http.Request, name string) (uuid.UUID, bool) {
	id, err := uuid.Parse(r.PathValue(name))
	if err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("invalid %s", name))
		return uuid.Nil, false
	}

	return id, true
}

func respondServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrAccountNotFound),
		errors.Is(err, service.ErrCategoryNotFound),
		errors.Is(err, service.ErrTransactionNotFound):
		respondError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrAccountNameExists),
		errors.Is(err, service.ErrCategoryNameExists):
		respondError(w, http.StatusConflict, err.Error())
	case errors.Is(err, service.ErrInvalidAmount),
		errors.Is(err, service.ErrInvalidType),
		errors.Is(err, service.ErrInvalidName),
		errors.Is(err, service.ErrInvalidCurrency):
		respondError(w, http.StatusBadRequest, err.Error())
	default:
		respondError(w, http.StatusInternalServerError, "internal server error")
	}
}

package httpserver

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"goskeleton/internal/model"
	"goskeleton/internal/service"
	"goskeleton/internal/spreadsheet"
)

type AccountService interface {
	CreateAccount(ctx context.Context, userID uuid.UUID, req model.CreateAccountRequest) (*model.Account, error)
	GetAccount(ctx context.Context, id uuid.UUID) (*model.Account, error)
	ListAccounts(ctx context.Context, userID uuid.UUID) ([]model.Account, error)
	UpdateAccount(ctx context.Context, userID uuid.UUID, id uuid.UUID, req model.UpdateAccountRequest) (*model.Account, error)
	DeleteAccount(ctx context.Context, userID uuid.UUID, id uuid.UUID) error
}

type CategoryService interface {
	CreateCategory(ctx context.Context, userID uuid.UUID, req model.CreateCategoryRequest) (*model.Category, error)
	GetCategory(ctx context.Context, id uuid.UUID) (*model.Category, error)
	ListCategories(ctx context.Context, userID uuid.UUID) ([]model.Category, error)
	DeleteCategory(ctx context.Context, userID uuid.UUID, id uuid.UUID) error
}

type TransactionService interface {
	CreateTransaction(ctx context.Context, userID uuid.UUID, req model.CreateTransactionRequest) (*model.Transaction, error)
	GetTransaction(ctx context.Context, id uuid.UUID) (*model.Transaction, error)
	ListTransactions(ctx context.Context, userID uuid.UUID, filters model.TransactionFilters) (*model.PaginatedTransactions, error)
	ExportTransactions(ctx context.Context, userID uuid.UUID, filters model.TransactionFilters, emit func(model.TransactionExportRow) error) error
	PreviewImport(ctx context.Context, userID uuid.UUID, r io.Reader) (*model.ImportResult, error)
	ImportTransactions(ctx context.Context, userID uuid.UUID, r io.Reader) (*model.ImportResult, error)
	UpdateTransaction(ctx context.Context, userID uuid.UUID, id uuid.UUID, req model.UpdateTransactionRequest) (*model.Transaction, error)
	DeleteTransaction(ctx context.Context, userID uuid.UUID, id uuid.UUID) error
	GetDashboardSummary(ctx context.Context, userID uuid.UUID) (*model.DashboardSummary, error)
}

type ImportService interface {
	Preview(ctx context.Context, userID uuid.UUID, table *spreadsheet.Table, sheets []string, opts service.ImportOptions) (*model.ImportResult, error)
	Import(ctx context.Context, userID uuid.UUID, table *spreadsheet.Table, sheets []string, opts service.ImportOptions) (*model.ImportResult, error)
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
		userID, err := userIDFromContext(r.Context())
		if err != nil {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		id, ok := parseUUIDPath(w, r, "id")
		if !ok {
			return
		}

		var req model.UpdateAccountRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		account, err := accounts.UpdateAccount(r.Context(), userID, id, req)
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
		userID, err := userIDFromContext(r.Context())
		if err != nil {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		id, ok := parseUUIDPath(w, r, "id")
		if !ok {
			return
		}

		if err := accounts.DeleteAccount(r.Context(), userID, id); err != nil {
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
		userID, err := userIDFromContext(r.Context())
		if err != nil {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		id, ok := parseUUIDPath(w, r, "id")
		if !ok {
			return
		}

		if err := categories.DeleteCategory(r.Context(), userID, id); err != nil {
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
func handleExportTransactions(transactions TransactionService) http.HandlerFunc {
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

		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set(
			"Content-Disposition",
			fmt.Sprintf(`attachment; filename="transactions-%s.csv"`, time.Now().UTC().Format("2006-01-02")),
		)

		writer := csv.NewWriter(w)
		if err := writer.Write([]string{"date", "type", "amount", "description", "account", "category"}); err != nil {
			return
		}

		err = transactions.ExportTransactions(r.Context(), userID, filters, func(row model.TransactionExportRow) error {
			return writer.Write([]string{
				row.Date.Format("2006-01-02"),
				row.Type,
				strconv.FormatFloat(row.Amount, 'f', 2, 64),
				row.Description,
				row.AccountName,
				row.CategoryName,
			})
		})
		writer.Flush()

		if err != nil {
			slog.ErrorContext(r.Context(), "export transactions", "error", err)
		}
	}
}

const maxImportBytes = 5 << 20

func parseUploadedFile(w http.ResponseWriter, r *http.Request, maxBytes int64) (multipart.File, string, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)

	if err := r.ParseMultipartForm(maxBytes); err != nil {
		respondError(w, http.StatusBadRequest, "invalid multipart form or file too large")
		return nil, "", false
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		respondError(w, http.StatusBadRequest, "file is required")
		return nil, "", false
	}

	return file, header.Filename, true
}

func handlePreviewImportTransactions(imports ImportService, limiter *rateLimiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := userIDFromContext(r.Context())
		if err != nil {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		file, filename, ok := parseUploadedFile(w, r, maxImportBytes)
		if !ok {
			return
		}
		defer func() {
			_ = file.Close()
		}()

		table, sheets, err := selectImportSheet(r, file, filename)
		if err != nil {
			respondSpreadsheetError(w, err)
			return
		}

		mode := importModeFromRequest(r)
		if mode == service.ImportModeAI && limiter != nil && !limiter.allow("import-ai|"+userID.String()) {
			slog.WarnContext(r.Context(), "import ai preview rate limited", "user_id", userID.String(), "sheet", table.Name)
			respondError(w, http.StatusTooManyRequests, "too many requests")
			return
		}

		result, err := imports.Preview(r.Context(), userID, table, sheets, service.ImportOptions{
			Filename: filename,
			Sheet:    table.Name,
			Mode:     mode,
		})
		if err != nil {
			respondImportError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, result)
	}
}

func handleImportTransactions(imports ImportService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := userIDFromContext(r.Context())
		if err != nil {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		file, filename, ok := parseUploadedFile(w, r, maxImportBytes)
		if !ok {
			return
		}
		defer func() {
			_ = file.Close()
		}()

		if r.FormValue("confirm") != "true" {
			respondError(w, http.StatusBadRequest, "confirm=true is required")
			return
		}

		table, sheets, err := selectImportSheet(r, file, filename)
		if err != nil {
			respondSpreadsheetError(w, err)
			return
		}

		mapping, err := parseImportMapping(r)
		if err != nil {
			respondImportError(w, err)
			return
		}

		result, err := imports.Import(r.Context(), userID, table, sheets, service.ImportOptions{
			Filename: filename,
			Sheet:    table.Name,
			Mode:     importModeFromRequest(r),
			Mapping:  mapping,
		})
		if err != nil {
			respondImportError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, result)
	}
}

func importModeFromRequest(r *http.Request) service.ImportMode {
	if strings.EqualFold(strings.TrimSpace(r.FormValue("mode")), "strict") {
		return service.ImportModeStrict
	}
	return service.ImportModeAI
}

func parseImportMapping(r *http.Request) (*model.ImportMapping, error) {
	raw := r.FormValue("mapping")
	if raw == "" {
		return nil, nil
	}
	if len(raw) > service.MaxMappingBytes {
		return nil, fmt.Errorf("%w: mapping exceeds 32 KB", service.ErrInvalidMapping)
	}
	var mapping model.ImportMapping
	if err := json.Unmarshal([]byte(raw), &mapping); err != nil {
		return nil, fmt.Errorf("%w: malformed mapping", service.ErrInvalidMapping)
	}
	return &mapping, nil
}

func respondImportError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrImportAIUnavailable):
		respondError(w, http.StatusBadGateway, "AI could not read this sheet. Try again.")
	case errors.Is(err, service.ErrInvalidMapping),
		errors.Is(err, service.ErrImportMissingColumns),
		errors.Is(err, service.ErrImportTooManyRows):
		respondError(w, http.StatusBadRequest, err.Error())
	default:
		respondServiceError(w, err)
	}
}

func selectImportSheet(r *http.Request, file multipart.File, filename string) (*spreadsheet.Table, []string, error) {
	workbook, err := spreadsheet.Parse(filename, file)
	if err != nil {
		return nil, nil, err
	}

	table, err := workbook.Pick(r.FormValue("sheet"))
	if err != nil {
		return nil, nil, err
	}

	return table, workbook.Names(), nil
}

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

// UpdateTransaction godoc
// @Summary Update a transaction
// @Description Update any subset of a transaction's account, category, type, amount, description or date
// @Tags transactions
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Transaction ID"
// @Param request body model.UpdateTransactionRequest true "Transaction update request"
// @Success 200 {object} model.Transaction
// @Failure 400 {object} model.ErrorResponse
// @Failure 401 {object} model.ErrorResponse
// @Failure 404 {object} model.ErrorResponse
// @Router /api/transactions/{id} [patch]
func handleUpdateTransaction(transactions TransactionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := userIDFromContext(r.Context())
		if err != nil {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		id, ok := parseUUIDPath(w, r, "id")
		if !ok {
			return
		}

		var req model.UpdateTransactionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		transaction, err := transactions.UpdateTransaction(r.Context(), userID, id, req)
		if err != nil {
			respondServiceError(w, err)
			return
		}

		respondJSON(w, http.StatusOK, transaction)
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
		userID, err := userIDFromContext(r.Context())
		if err != nil {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		id, ok := parseUUIDPath(w, r, "id")
		if !ok {
			return
		}

		if err := transactions.DeleteTransaction(r.Context(), userID, id); err != nil {
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
		errors.Is(err, service.ErrInvalidCurrency),
		errors.Is(err, service.ErrImportTooManyRows),
		errors.Is(err, service.ErrImportMissingColumns):
		respondError(w, http.StatusBadRequest, err.Error())
	default:
		respondError(w, http.StatusInternalServerError, "internal server error")
	}
}

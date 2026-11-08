package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/netip"
	"time"

	"github.com/google/uuid"
	httpSwagger "github.com/swaggo/http-swagger"

	"goskeleton/internal/model"
	"goskeleton/internal/service"
)

type HealthChecker interface {
	Check(ctx context.Context) service.Status
}

type AuthService interface {
	Register(ctx context.Context, req model.RegisterRequest) (*model.User, error)
	Login(ctx context.Context, req model.LoginRequest) (*model.LoginResponse, error)
	RequestPasswordReset(ctx context.Context, req model.ForgotPasswordRequest) error
	ResetPassword(ctx context.Context, req model.ResetPasswordRequest) error
	ChangePassword(ctx context.Context, userID uuid.UUID, currentPassword, newPassword string) error
}

// NewHandler creates a new HTTP handler with all routes.
// @title GoSkeleton API
// @version 1.0
// @description This is a sample GoSkeleton API with authentication and email features.
// @host localhost:8080
// @BasePath /
// @schemes http
func NewHandler(health HealthChecker, auth AuthService, accounts AccountService, categories CategoryService, transactions TransactionService, imports ImportService, importMode service.ImportMode, budgets BudgetService, recurring RecurringService, reports ReportService, chat ChatService, attachments AttachmentService, jwtSecret string, allowedOrigins []string) http.Handler {
	mux := http.NewServeMux()

	// Health endpoint (local only)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if !isLocalRequest(r.RemoteAddr) {
			http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		if err := json.NewEncoder(w).Encode(health.Check(r.Context())); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	// Auth endpoints
	authLimiter := newRateLimiter(5, 15*time.Minute)
	loginLimiter := newRateLimiter(10, 15*time.Minute)
	changePasswordLimiter := newRateLimiter(5, 15*time.Minute)
	mux.HandleFunc("POST /api/auth/register", handleRegister(auth))
	mux.HandleFunc("POST /api/auth/login", loginLimiter.wrap(handleLogin(auth)))
	mux.HandleFunc("POST /api/auth/forgot-password", authLimiter.wrap(handleForgotPassword(auth)))
	mux.HandleFunc("POST /api/auth/reset-password", handleResetPassword(auth))
	mux.Handle("POST /api/auth/change-password", JWTAuth(jwtSecret)(http.HandlerFunc(handleChangePassword(auth, changePasswordLimiter))))

	// Finance API endpoints (JWT protected)
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("GET /api/accounts", handleListAccounts(accounts))
	apiMux.HandleFunc("POST /api/accounts", handleCreateAccount(accounts))
	apiMux.HandleFunc("PATCH /api/accounts/{id}", handleUpdateAccount(accounts))
	apiMux.HandleFunc("POST /api/accounts/{id}/adjustments", handleAdjustAccountBalance(accounts))
	apiMux.HandleFunc("DELETE /api/accounts/{id}", handleDeleteAccount(accounts, attachments))

	apiMux.HandleFunc("GET /api/categories", handleListCategories(categories))
	apiMux.HandleFunc("POST /api/categories", handleCreateCategory(categories))
	apiMux.HandleFunc("PATCH /api/categories/{id}", handleUpdateCategory(categories))
	apiMux.HandleFunc("DELETE /api/categories/{id}", handleDeleteCategory(categories))

	apiMux.HandleFunc("GET /api/transactions", handleListTransactions(transactions))
	apiMux.HandleFunc("GET /api/transactions/export.csv", handleExportTransactions(transactions))
	var importLimiter *rateLimiter
	if importMode != service.ImportModeStrict {
		importLimiter = newRateLimiter(20, time.Minute)
	}
	apiMux.HandleFunc("POST /api/transactions/import/preview", handlePreviewImportTransactions(imports, importLimiter))
	apiMux.HandleFunc("POST /api/transactions/import", handleImportTransactions(imports))
	apiMux.HandleFunc("POST /api/transactions", handleCreateTransaction(transactions))
	apiMux.HandleFunc("PATCH /api/transactions/{id}", handleUpdateTransaction(transactions))
	apiMux.HandleFunc("DELETE /api/transactions/{id}", handleDeleteTransaction(transactions, attachments))

	apiMux.HandleFunc("POST /api/transactions/{id}/attachments", handleUploadTransactionAttachment(attachments))
	apiMux.HandleFunc("GET /api/transactions/{id}/attachments", handleListTransactionAttachments(attachments))
	apiMux.HandleFunc("GET /api/attachments/{id}", handleDownloadAttachment(attachments))
	apiMux.HandleFunc("DELETE /api/attachments/{id}", handleDeleteAttachment(attachments))

	apiMux.HandleFunc("POST /api/transfers", handleCreateTransfer(transactions))
	apiMux.HandleFunc("DELETE /api/transfers/{group_id}", handleDeleteTransfer(transactions))

	apiMux.HandleFunc("GET /api/dashboard/summary", handleGetDashboardSummary(transactions))

	apiMux.HandleFunc("GET /api/budgets", HandleListBudgets(budgets))
	apiMux.HandleFunc("PUT /api/budgets", HandleUpsertBudget(budgets))
	apiMux.HandleFunc("DELETE /api/budgets/{category_id}", HandleDeleteBudget(budgets))

	apiMux.HandleFunc("GET /api/recurring", HandleListRecurring(recurring))
	apiMux.HandleFunc("POST /api/recurring", HandleCreateRecurring(recurring))
	apiMux.HandleFunc("PATCH /api/recurring/{id}", HandleUpdateRecurring(recurring))
	apiMux.HandleFunc("DELETE /api/recurring/{id}", HandleDeleteRecurring(recurring))
	apiMux.HandleFunc("POST /api/recurring/{id}/run-now", HandleRunRecurringNow(recurring))

	apiMux.HandleFunc("GET /api/reports/cashflow", HandleCashflowReport(reports))
	apiMux.HandleFunc("GET /api/reports/categories", HandleCategoryReport(reports))
	apiMux.HandleFunc("GET /api/reports/merchants", HandleMerchantReport(reports))

	chatLimiter := newRateLimiter(20, time.Minute)
	apiMux.HandleFunc("POST /api/chat", handleChat(chat, chatLimiter))
	apiMux.HandleFunc("POST /api/chat/files", handleChatFileUpload(chat, chatLimiter))
	apiMux.HandleFunc("GET /api/chat/history", handleChatHistory(chat))
	apiMux.HandleFunc("DELETE /api/chat/history", handleChatClear(chat))
	apiMux.HandleFunc("POST /api/chat/actions/{id}/confirm", handleChatConfirm(chat))
	apiMux.HandleFunc("POST /api/chat/actions/{id}/cancel", handleChatCancel(chat))

	mux.Handle("/api/", JWTAuth(jwtSecret)(apiMux))

	// Swagger UI
	mux.Handle("/swagger/", httpSwagger.WrapHandler)

	return CORS(allowedOrigins)(mux)
}

// Register godoc
// @Summary Register a new user
// @Description Register a new user with email and password
// @Tags auth
// @Accept json
// @Produce json
// @Param request body model.RegisterRequest true "Registration request"
// @Success 201 {object} model.User
// @Failure 400 {object} model.ErrorResponse
// @Failure 409 {object} model.ErrorResponse
// @Router /api/auth/register [post]
func handleRegister(auth AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			respondError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		var req model.RegisterRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if req.Email == "" || req.Password == "" {
			respondError(w, http.StatusBadRequest, "email and password are required")
			return
		}

		user, err := auth.Register(r.Context(), req)
		if err != nil {
			switch {
			case errors.Is(err, service.ErrEmailExists):
				respondError(w, http.StatusConflict, "email already registered")
			case errors.Is(err, service.ErrPasswordTooShort):
				respondError(w, http.StatusBadRequest, "password must be at least 8 characters")
			case errors.Is(err, service.ErrPasswordTooLong):
				respondError(w, http.StatusBadRequest, "password must be at most 72 bytes")
			default:
				respondError(w, http.StatusInternalServerError, "registration failed")
			}
			return
		}

		respondJSON(w, http.StatusCreated, user)
	}
}

// Login godoc
// @Summary Login user
// @Description Login with email and password to receive JWT token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body model.LoginRequest true "Login request"
// @Success 200 {object} model.LoginResponse
// @Failure 400 {object} model.ErrorResponse
// @Failure 401 {object} model.ErrorResponse
// @Router /api/auth/login [post]
func handleLogin(auth AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			respondError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		var req model.LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if req.Email == "" || req.Password == "" {
			respondError(w, http.StatusBadRequest, "email and password are required")
			return
		}

		resp, err := auth.Login(r.Context(), req)
		if err != nil {
			if errors.Is(err, service.ErrInvalidPassword) || errors.Is(err, service.ErrUserNotFound) {
				respondError(w, http.StatusUnauthorized, "invalid credentials")
				return
			}
			respondError(w, http.StatusInternalServerError, "login failed")
			return
		}

		respondJSON(w, http.StatusOK, resp)
	}
}

// ForgotPassword godoc
// @Summary Request password reset
// @Description Request a password reset token via email
// @Tags auth
// @Accept json
// @Produce json
// @Param request body model.ForgotPasswordRequest true "Forgot password request"
// @Success 200 {object} model.MessageResponse
// @Failure 400 {object} model.ErrorResponse
// @Router /api/auth/forgot-password [post]
func handleForgotPassword(auth AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			respondError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		var req model.ForgotPasswordRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if req.Email == "" {
			respondError(w, http.StatusBadRequest, "email is required")
			return
		}

		if err := auth.RequestPasswordReset(r.Context(), req); err != nil {
			respondError(w, http.StatusInternalServerError, "failed to process request")
			return
		}

		respondJSON(w, http.StatusOK, model.MessageResponse{Message: "if the email exists, a reset link will be sent"})
	}
}

// ResetPassword godoc
// @Summary Reset password
// @Description Reset password using token received via email
// @Tags auth
// @Accept json
// @Produce json
// @Param request body model.ResetPasswordRequest true "Reset password request"
// @Success 200 {object} model.MessageResponse
// @Failure 400 {object} model.ErrorResponse
// @Failure 401 {object} model.ErrorResponse
// @Router /api/auth/reset-password [post]
func handleResetPassword(auth AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			respondError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		var req model.ResetPasswordRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if req.Token == "" || req.Password == "" {
			respondError(w, http.StatusBadRequest, "token and password are required")
			return
		}

		if err := auth.ResetPassword(r.Context(), req); err != nil {
			switch {
			case errors.Is(err, service.ErrInvalidToken), errors.Is(err, service.ErrTokenExpired):
				respondError(w, http.StatusBadRequest, "invalid or expired token")
			case errors.Is(err, service.ErrPasswordTooShort):
				respondError(w, http.StatusBadRequest, "password must be at least 8 characters")
			case errors.Is(err, service.ErrPasswordTooLong):
				respondError(w, http.StatusBadRequest, "password must be at most 72 bytes")
			default:
				respondError(w, http.StatusInternalServerError, "failed to reset password")
			}
			return
		}

		respondJSON(w, http.StatusOK, model.MessageResponse{Message: "password reset successful"})
	}
}

// ChangePassword godoc
// @Summary Change the authenticated user's password
// @Description Verify the current password and replace it with a new one
// @Tags auth
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body model.ChangePasswordRequest true "Change password request"
// @Success 200 {object} model.MessageResponse
// @Failure 400 {object} model.ErrorResponse
// @Failure 401 {object} model.ErrorResponse
// @Router /api/auth/change-password [post]
func handleChangePassword(auth AuthService, limiter *rateLimiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := userIDFromContext(r.Context())
		if err != nil {
			respondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		if limiter != nil && !limiter.allow("change-password|"+userID.String()) {
			respondError(w, http.StatusTooManyRequests, "too many requests")
			return
		}

		var req model.ChangePasswordRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if req.CurrentPassword == "" || req.NewPassword == "" {
			respondError(w, http.StatusBadRequest, "current_password and new_password are required")
			return
		}

		if err := auth.ChangePassword(r.Context(), userID, req.CurrentPassword, req.NewPassword); err != nil {
			switch {
			case errors.Is(err, service.ErrInvalidPassword), errors.Is(err, service.ErrUserNotFound):
				respondError(w, http.StatusUnauthorized, "invalid current password")
			case errors.Is(err, service.ErrPasswordTooShort):
				respondError(w, http.StatusBadRequest, "password must be at least 8 characters")
			case errors.Is(err, service.ErrPasswordTooLong):
				respondError(w, http.StatusBadRequest, "password must be at most 72 bytes")
			default:
				respondError(w, http.StatusInternalServerError, "failed to change password")
			}
			return
		}

		respondJSON(w, http.StatusOK, model.MessageResponse{Message: "password changed successfully"})
	}
}

func respondJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, model.ErrorResponse{Error: message})
}

func isLocalRequest(remoteAddr string) bool {
	addr, err := netip.ParseAddrPort(remoteAddr)
	if err != nil {
		return false
	}

	ip := addr.Addr()
	return ip.IsLoopback()
}

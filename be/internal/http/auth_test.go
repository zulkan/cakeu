package httpserver_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"

	httpserver "goskeleton/internal/http"
	"goskeleton/internal/model"
	"goskeleton/internal/service"
)

type fakeAuthService struct {
	registerUser  *model.User
	registerErr   error
	loginResp     *model.LoginResponse
	loginErr      error
	forgotErr     error
	forgotCalled  int
	resetErr      error
	changeErr     error
	changedUserID uuid.UUID
	changedPair   [2]string
}

func (f *fakeAuthService) Register(ctx context.Context, req model.RegisterRequest) (*model.User, error) {
	return f.registerUser, f.registerErr
}

func (f *fakeAuthService) Login(ctx context.Context, req model.LoginRequest) (*model.LoginResponse, error) {
	return f.loginResp, f.loginErr
}

func (f *fakeAuthService) RequestPasswordReset(ctx context.Context, req model.ForgotPasswordRequest) error {
	f.forgotCalled++
	return f.forgotErr
}

func (f *fakeAuthService) ResetPassword(ctx context.Context, req model.ResetPasswordRequest) error {
	return f.resetErr
}

func (f *fakeAuthService) ChangePassword(ctx context.Context, userID uuid.UUID, currentPassword, newPassword string) error {
	f.changedUserID = userID
	f.changedPair = [2]string{currentPassword, newPassword}
	return f.changeErr
}

func newAuthTestHandler(auth *fakeAuthService) http.Handler {
	return httpserver.NewHandler(service.NewHealth(nil), auth, &fakeAccountService{}, &fakeCategoryService{}, &fakeTransactionService{}, nil, service.ImportModeAI, nil, nil, nil, nil, nil, testJWTSecret, nil)
}

func TestAuthRoutesReachableWithoutJWTAndProtectedRoutes401(t *testing.T) {
	t.Parallel()

	auth := &fakeAuthService{loginResp: &model.LoginResponse{Token: "t", User: model.User{ID: uuid.New()}}, resetErr: service.ErrInvalidToken}
	handler := newAuthTestHandler(auth)

	recorder := doRequest(t, handler, http.MethodPost, "/api/auth/reset-password", "",
		`{"token":"bad","password":"newpassword123"}`)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad token, got %d with body %q", recorder.Code, recorder.Body.String())
	}

	// protected route still 401.
	recorder = doRequest(t, handler, http.MethodGet, "/api/accounts", "", "")
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for protected route, got %d", recorder.Code)
	}
}

func TestForgotPasswordAlways200(t *testing.T) {
	t.Parallel()

	auth := &fakeAuthService{}
	handler := newAuthTestHandler(auth)

	recorder := doRequest(t, handler, http.MethodPost, "/api/auth/forgot-password", "",
		`{"email":"known@example.com"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 for known email, got %d with body %q", recorder.Code, recorder.Body.String())
	}

	recorder = doRequest(t, handler, http.MethodPost, "/api/auth/forgot-password", "",
		`{"email":"unknown@example.com"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200 for unknown email, got %d with body %q", recorder.Code, recorder.Body.String())
	}
}

func TestResetPasswordShortPassword400(t *testing.T) {
	t.Parallel()

	auth := &fakeAuthService{resetErr: service.ErrPasswordTooShort}
	handler := newAuthTestHandler(auth)

	recorder := doRequest(t, handler, http.MethodPost, "/api/auth/reset-password", "",
		`{"token":"t","password":"short"}`)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for short password, got %d with body %q", recorder.Code, recorder.Body.String())
	}
}

func TestForgotPasswordRateLimit429(t *testing.T) {
	t.Parallel()

	auth := &fakeAuthService{}
	handler := newAuthTestHandler(auth)

	var last int
	for i := 0; i < 6; i++ {
		recorder := doRequest(t, handler, http.MethodPost, "/api/auth/forgot-password", "",
			`{"email":"rl@example.com"}`)
		last = recorder.Code
	}
	if last != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after 5 requests, got %d", last)
	}
}

func TestChangePasswordRequiresAuth(t *testing.T) {
	t.Parallel()

	handler := newAuthTestHandler(&fakeAuthService{})

	recorder := doRequest(t, handler, http.MethodPost, "/api/auth/change-password", "",
		`{"current_password":"oldpassword123","new_password":"newpassword123"}`)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without a token, got %d with body %q", recorder.Code, recorder.Body.String())
	}
}

func TestChangePasswordSucceeds(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	auth := &fakeAuthService{}
	handler := newAuthTestHandler(auth)

	recorder := doRequest(t, handler, http.MethodPost, "/api/auth/change-password", signToken(t, testJWTSecret, userID),
		`{"current_password":"oldpassword123","new_password":"newpassword123"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d with body %q", recorder.Code, recorder.Body.String())
	}
	if auth.changedUserID != userID {
		t.Fatalf("expected authenticated user %s, got %s", userID, auth.changedUserID)
	}
	if auth.changedPair != [2]string{"oldpassword123", "newpassword123"} {
		t.Fatalf("expected forwarded password pair, got %v", auth.changedPair)
	}
}

func TestChangePasswordWrongCurrentPassword401(t *testing.T) {
	t.Parallel()

	auth := &fakeAuthService{changeErr: service.ErrInvalidPassword}
	handler := newAuthTestHandler(auth)

	recorder := doRequest(t, handler, http.MethodPost, "/api/auth/change-password", signToken(t, testJWTSecret, uuid.New()),
		`{"current_password":"wrongpassword","new_password":"newpassword123"}`)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d with body %q", recorder.Code, recorder.Body.String())
	}
}

func TestChangePasswordShortPassword400(t *testing.T) {
	t.Parallel()

	auth := &fakeAuthService{changeErr: service.ErrPasswordTooShort}
	handler := newAuthTestHandler(auth)

	recorder := doRequest(t, handler, http.MethodPost, "/api/auth/change-password", signToken(t, testJWTSecret, uuid.New()),
		`{"current_password":"oldpassword123","new_password":"short"}`)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d with body %q", recorder.Code, recorder.Body.String())
	}
}

func TestLoginRateLimit429(t *testing.T) {
	t.Parallel()

	auth := &fakeAuthService{loginErr: service.ErrInvalidPassword}
	handler := newAuthTestHandler(auth)

	var last int
	for i := 0; i < 10; i++ {
		recorder := doRequest(t, handler, http.MethodPost, "/api/auth/login", "",
			`{"email":"rl@example.com","password":"wrongpassword"}`)
		last = recorder.Code
	}
	if last != http.StatusUnauthorized {
		t.Fatalf("expected 401 for the first 10 attempts, got %d", last)
	}

	recorder := doRequest(t, handler, http.MethodPost, "/api/auth/login", "",
		`{"email":"rl@example.com","password":"wrongpassword"}`)
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 on the 11th attempt, got %d with body %q", recorder.Code, recorder.Body.String())
	}
}

func TestLoginRateLimitIsScopedByEmailAndIP(t *testing.T) {
	t.Parallel()

	auth := &fakeAuthService{loginErr: service.ErrInvalidPassword}
	handler := newAuthTestHandler(auth)

	for i := 0; i < 10; i++ {
		doRequest(t, handler, http.MethodPost, "/api/auth/login", "",
			`{"email":"locked@example.com","password":"wrongpassword"}`)
	}

	otherEmail := doRequest(t, handler, http.MethodPost, "/api/auth/login", "",
		`{"email":"other@example.com","password":"wrongpassword"}`)
	if otherEmail.Code == http.StatusTooManyRequests {
		t.Fatalf("expected another email to be unaffected, got %d", otherEmail.Code)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/auth/login",
		strings.NewReader(`{"email":"locked@example.com","password":"wrongpassword"}`))
	request.RemoteAddr = "203.0.113.9:5555"
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code == http.StatusTooManyRequests {
		t.Fatalf("expected another IP to be unaffected, got %d", recorder.Code)
	}
}

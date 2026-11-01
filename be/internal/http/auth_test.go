package httpserver_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"

	httpserver "goskeleton/internal/http"
	"goskeleton/internal/model"
	"goskeleton/internal/service"
)

type fakeAuthService struct {
	registerUser *model.User
	registerErr  error
	loginResp    *model.LoginResponse
	loginErr     error
	forgotErr    error
	forgotCalled int
	resetErr     error
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

func newAuthTestHandler(auth *fakeAuthService) http.Handler {
	return httpserver.NewHandler(service.NewHealth(nil), auth, &fakeAccountService{}, &fakeCategoryService{}, &fakeTransactionService{}, nil, nil, nil, nil, testJWTSecret, nil)
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

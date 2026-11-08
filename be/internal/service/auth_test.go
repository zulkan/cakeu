package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"goskeleton/internal/model"
	"goskeleton/internal/service"
)

type fakeUserRepo struct {
	userByEmail *model.User
	userByID    *model.User
	createErr   error
	getErr      error
	updateErr   error
	updatedPw   string
	updatedID   uuid.UUID
}

func (f *fakeUserRepo) CreateUser(ctx context.Context, email, passwordHash string) (*model.User, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	return &model.User{ID: uuid.New(), Email: email, PasswordHash: passwordHash}, nil
}

func (f *fakeUserRepo) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.userByEmail != nil {
		return f.userByEmail, nil
	}
	return nil, service.ErrUserNotFound
}

func (f *fakeUserRepo) GetUserByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	if f.userByID != nil {
		return f.userByID, nil
	}
	return nil, service.ErrUserNotFound
}

func (f *fakeUserRepo) UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash string) error {
	f.updatedID = id
	f.updatedPw = passwordHash
	return f.updateErr
}

type fakeTokenRepo struct {
	token            *model.PasswordResetToken
	createErr        error
	getErr           error
	markUsedErr      error
	invalidateErr    error
	invalidateCalls  int
	invalidateLastID uuid.UUID
}

func (f *fakeTokenRepo) CreateToken(ctx context.Context, userID uuid.UUID, token string, expiresAt time.Time) error {
	return f.createErr
}

func (f *fakeTokenRepo) GetTokenByToken(ctx context.Context, token string) (*model.PasswordResetToken, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.token != nil {
		return f.token, nil
	}
	return nil, service.ErrInvalidToken
}

func (f *fakeTokenRepo) MarkTokenAsUsed(ctx context.Context, id uuid.UUID) error {
	return f.markUsedErr
}

func (f *fakeTokenRepo) InvalidateUserTokens(ctx context.Context, userID uuid.UUID) error {
	f.invalidateCalls++
	f.invalidateLastID = userID
	return f.invalidateErr
}

type fakeEmailSender struct {
	sent bool
	err  error
}

func (f *fakeEmailSender) SendPasswordResetEmail(ctx context.Context, to, resetToken string) error {
	f.sent = true
	return f.err
}

func TestAuthServiceRegisterCreatesUser(t *testing.T) {
	t.Parallel()

	userRepo := &fakeUserRepo{}
	tokenRepo := &fakeTokenRepo{}
	emailer := &fakeEmailSender{}
	auth := service.NewAuthService(userRepo, tokenRepo, emailer, "secret")

	req := model.RegisterRequest{Email: "test@example.com", Password: "password123"}
	user, err := auth.Register(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if user.Email != req.Email {
		t.Fatalf("expected email %q, got %q", req.Email, user.Email)
	}
}

func TestAuthServiceRegisterReturnsEmailExists(t *testing.T) {
	t.Parallel()

	userRepo := &fakeUserRepo{createErr: service.ErrEmailExists}
	tokenRepo := &fakeTokenRepo{}
	emailer := &fakeEmailSender{}
	auth := service.NewAuthService(userRepo, tokenRepo, emailer, "secret")

	req := model.RegisterRequest{Email: "test@example.com", Password: "password123"}
	_, err := auth.Register(context.Background(), req)
	if !errors.Is(err, service.ErrEmailExists) {
		t.Fatalf("expected ErrEmailExists, got %v", err)
	}
}

func TestAuthServiceLoginReturnsToken(t *testing.T) {
	t.Parallel()

	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	userRepo := &fakeUserRepo{
		userByEmail: &model.User{ID: uuid.New(), Email: "test@example.com", PasswordHash: string(hash)},
	}
	tokenRepo := &fakeTokenRepo{}
	emailer := &fakeEmailSender{}
	auth := service.NewAuthService(userRepo, tokenRepo, emailer, "secret")

	req := model.LoginRequest{Email: "test@example.com", Password: "password123"}
	resp, err := auth.Login(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if resp.Token == "" {
		t.Fatal("expected token to be non-empty")
	}
}

func TestAuthServiceLoginReturnsInvalidPassword(t *testing.T) {
	t.Parallel()

	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	userRepo := &fakeUserRepo{
		userByEmail: &model.User{ID: uuid.New(), Email: "test@example.com", PasswordHash: string(hash)},
	}
	tokenRepo := &fakeTokenRepo{}
	emailer := &fakeEmailSender{}
	auth := service.NewAuthService(userRepo, tokenRepo, emailer, "secret")

	req := model.LoginRequest{Email: "test@example.com", Password: "wrongpassword"}
	_, err := auth.Login(context.Background(), req)
	if !errors.Is(err, service.ErrInvalidPassword) {
		t.Fatalf("expected ErrInvalidPassword, got %v", err)
	}
}

func TestAuthServiceRequestPasswordReset(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	userRepo := &fakeUserRepo{
		userByEmail: &model.User{ID: userID, Email: "test@example.com"},
	}
	tokenRepo := &fakeTokenRepo{}
	emailer := &fakeEmailSender{}
	auth := service.NewAuthService(userRepo, tokenRepo, emailer, "secret")

	req := model.ForgotPasswordRequest{Email: "test@example.com"}
	err := auth.RequestPasswordReset(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !emailer.sent {
		t.Fatal("expected email to be sent")
	}
	if tokenRepo.invalidateCalls != 1 || tokenRepo.invalidateLastID != userID {
		t.Fatalf("expected prior tokens invalidated for user %s, got calls=%d", userID, tokenRepo.invalidateCalls)
	}
}

func TestAuthServiceRequestPasswordResetUnknownEmail(t *testing.T) {
	t.Parallel()

	userRepo := &fakeUserRepo{}
	tokenRepo := &fakeTokenRepo{}
	emailer := &fakeEmailSender{}
	auth := service.NewAuthService(userRepo, tokenRepo, emailer, "secret")

	req := model.ForgotPasswordRequest{Email: "nobody@example.com"}
	err := auth.RequestPasswordReset(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error for unknown email, got %v", err)
	}
	if emailer.sent {
		t.Fatal("expected no email to be sent for unknown email")
	}
	if tokenRepo.invalidateCalls != 0 {
		t.Fatalf("expected no token invalidation for unknown email, got calls=%d", tokenRepo.invalidateCalls)
	}
}

func TestAuthServiceResetPassword(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	userRepo := &fakeUserRepo{
		userByID: &model.User{ID: userID, Email: "test@example.com"},
	}
	now := time.Now().UTC()
	tokenRepo := &fakeTokenRepo{
		token: &model.PasswordResetToken{
			ID:        uuid.New(),
			UserID:    userID,
			Token:     "valid-token",
			ExpiresAt: now.Add(1 * time.Hour),
			UsedAt:    nil,
		},
	}
	emailer := &fakeEmailSender{}
	auth := service.NewAuthService(userRepo, tokenRepo, emailer, "secret")

	req := model.ResetPasswordRequest{Token: "valid-token", Password: "newpassword123"}
	err := auth.ResetPassword(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if userRepo.updatedID != userID {
		t.Fatalf("expected password update for user %s, got %s", userID, userRepo.updatedID)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(userRepo.updatedPw), []byte("newpassword123")); err != nil {
		t.Fatalf("expected new password hash to match, got %v", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(userRepo.updatedPw), []byte("oldpassword123")); err == nil {
		t.Fatal("expected old password to not match the new hash")
	}
	if tokenRepo.invalidateCalls != 1 || tokenRepo.invalidateLastID != userID {
		t.Fatalf("expected sibling tokens invalidated for user %s, got calls=%d last=%s", userID, tokenRepo.invalidateCalls, tokenRepo.invalidateLastID)
	}
}

func TestAuthServiceRegisterRejectsInvalidPasswordLength(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		password string
		wantErr  error
	}{
		{name: "too short", password: "short", wantErr: service.ErrPasswordTooShort},
		{name: "too long", password: strings.Repeat("a", 73), wantErr: service.ErrPasswordTooLong},
		{name: "minimum length", password: strings.Repeat("a", 8), wantErr: nil},
		{name: "maximum length", password: strings.Repeat("a", 72), wantErr: nil},
	}

	for _, testCase := range tests {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			userRepo := &fakeUserRepo{}
			auth := service.NewAuthService(userRepo, &fakeTokenRepo{}, &fakeEmailSender{}, "secret")

			_, err := auth.Register(context.Background(), model.RegisterRequest{
				Email:    "test@example.com",
				Password: testCase.password,
			})
			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("expected %v, got %v", testCase.wantErr, err)
			}
		})
	}
}

func TestAuthServiceChangePassword(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	hash, _ := bcrypt.GenerateFromPassword([]byte("oldpassword123"), bcrypt.DefaultCost)
	userRepo := &fakeUserRepo{
		userByID: &model.User{ID: userID, Email: "test@example.com", PasswordHash: string(hash)},
	}
	auth := service.NewAuthService(userRepo, &fakeTokenRepo{}, &fakeEmailSender{}, "secret")

	err := auth.ChangePassword(context.Background(), userID, "oldpassword123", "newpassword123")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if userRepo.updatedID != userID {
		t.Fatalf("expected password update for user %s, got %s", userID, userRepo.updatedID)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(userRepo.updatedPw), []byte("newpassword123")); err != nil {
		t.Fatalf("expected new password hash to match, got %v", err)
	}
}

func TestAuthServiceChangePasswordRejectsWrongCurrentPassword(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	hash, _ := bcrypt.GenerateFromPassword([]byte("oldpassword123"), bcrypt.DefaultCost)
	userRepo := &fakeUserRepo{
		userByID: &model.User{ID: userID, PasswordHash: string(hash)},
	}
	auth := service.NewAuthService(userRepo, &fakeTokenRepo{}, &fakeEmailSender{}, "secret")

	err := auth.ChangePassword(context.Background(), userID, "wrongpassword", "newpassword123")
	if !errors.Is(err, service.ErrInvalidPassword) {
		t.Fatalf("expected ErrInvalidPassword, got %v", err)
	}
	if userRepo.updatedPw != "" {
		t.Fatal("expected no password update when the current password is wrong")
	}
}

func TestAuthServiceChangePasswordRejectsInvalidNewPassword(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		newPassword string
		wantErr     error
	}{
		{name: "too short", newPassword: "short", wantErr: service.ErrPasswordTooShort},
		{name: "too long", newPassword: strings.Repeat("a", 73), wantErr: service.ErrPasswordTooLong},
	}

	for _, testCase := range tests {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			userID := uuid.New()
			hash, _ := bcrypt.GenerateFromPassword([]byte("oldpassword123"), bcrypt.DefaultCost)
			userRepo := &fakeUserRepo{
				userByID: &model.User{ID: userID, PasswordHash: string(hash)},
			}
			auth := service.NewAuthService(userRepo, &fakeTokenRepo{}, &fakeEmailSender{}, "secret")

			err := auth.ChangePassword(context.Background(), userID, "oldpassword123", testCase.newPassword)
			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("expected %v, got %v", testCase.wantErr, err)
			}
			if userRepo.updatedPw != "" {
				t.Fatal("expected no password update for an invalid new password")
			}
		})
	}
}

func TestAuthServiceChangePasswordReturnsUserNotFound(t *testing.T) {
	t.Parallel()

	userRepo := &fakeUserRepo{}
	auth := service.NewAuthService(userRepo, &fakeTokenRepo{}, &fakeEmailSender{}, "secret")

	err := auth.ChangePassword(context.Background(), uuid.New(), "oldpassword123", "newpassword123")
	if !errors.Is(err, service.ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func TestAuthServiceResetPasswordRejectsShortPassword(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	userRepo := &fakeUserRepo{}
	now := time.Now().UTC()
	tokenRepo := &fakeTokenRepo{
		token: &model.PasswordResetToken{
			ID:        uuid.New(),
			UserID:    userID,
			Token:     "valid-token",
			ExpiresAt: now.Add(1 * time.Hour),
			UsedAt:    nil,
		},
	}
	emailer := &fakeEmailSender{}
	auth := service.NewAuthService(userRepo, tokenRepo, emailer, "secret")

	req := model.ResetPasswordRequest{Token: "valid-token", Password: "short"}
	err := auth.ResetPassword(context.Background(), req)
	if !errors.Is(err, service.ErrPasswordTooShort) {
		t.Fatalf("expected ErrPasswordTooShort, got %v", err)
	}
	if userRepo.updatedPw != "" {
		t.Fatal("expected no password update for short password")
	}
}

func TestAuthServiceResetPasswordRejectsUsedToken(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	userRepo := &fakeUserRepo{}
	now := time.Now().UTC()
	usedAt := now.Add(-time.Minute)
	tokenRepo := &fakeTokenRepo{
		token: &model.PasswordResetToken{
			ID:        uuid.New(),
			UserID:    userID,
			Token:     "used-token",
			ExpiresAt: now.Add(1 * time.Hour),
			UsedAt:    &usedAt,
		},
	}
	emailer := &fakeEmailSender{}
	auth := service.NewAuthService(userRepo, tokenRepo, emailer, "secret")

	req := model.ResetPasswordRequest{Token: "used-token", Password: "newpassword123"}
	err := auth.ResetPassword(context.Background(), req)
	if !errors.Is(err, service.ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
	if userRepo.updatedPw != "" {
		t.Fatal("expected no password update for used token")
	}
}

func TestAuthServiceResetPasswordReturnsInvalidToken(t *testing.T) {
	t.Parallel()

	userRepo := &fakeUserRepo{}
	tokenRepo := &fakeTokenRepo{getErr: service.ErrInvalidToken}
	emailer := &fakeEmailSender{}
	auth := service.NewAuthService(userRepo, tokenRepo, emailer, "secret")

	req := model.ResetPasswordRequest{Token: "invalid-token", Password: "newpassword123"}
	err := auth.ResetPassword(context.Background(), req)
	if !errors.Is(err, service.ErrInvalidToken) {
		t.Fatalf("expected ErrInvalidToken, got %v", err)
	}
}

func TestAuthServiceResetPasswordReturnsTokenExpired(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	userRepo := &fakeUserRepo{
		userByID: &model.User{ID: userID, Email: "test@example.com"},
	}
	now := time.Now().UTC()
	tokenRepo := &fakeTokenRepo{
		token: &model.PasswordResetToken{
			ID:        uuid.New(),
			UserID:    userID,
			Token:     "expired-token",
			ExpiresAt: now.Add(-1 * time.Hour),
			UsedAt:    nil,
		},
	}
	emailer := &fakeEmailSender{}
	auth := service.NewAuthService(userRepo, tokenRepo, emailer, "secret")

	req := model.ResetPasswordRequest{Token: "expired-token", Password: "newpassword123"}
	err := auth.ResetPassword(context.Background(), req)
	if !errors.Is(err, service.ErrTokenExpired) {
		t.Fatalf("expected ErrTokenExpired, got %v", err)
	}
}

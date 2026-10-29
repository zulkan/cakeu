package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"goskeleton/internal/model"
)

var (
	ErrUserNotFound     = errors.New("user not found")
	ErrInvalidPassword  = errors.New("invalid password")
	ErrEmailExists      = errors.New("email already registered")
	ErrInvalidToken     = errors.New("invalid or expired token")
	ErrTokenExpired     = errors.New("token expired")
	ErrPasswordTooShort = errors.New("password too short")
)

type UserRepository interface {
	CreateUser(ctx context.Context, email, passwordHash string) (*model.User, error)
	GetUserByEmail(ctx context.Context, email string) (*model.User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*model.User, error)
	UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash string) error
}

type TokenRepository interface {
	CreateToken(ctx context.Context, userID uuid.UUID, token string, expiresAt time.Time) error
	GetTokenByToken(ctx context.Context, token string) (*model.PasswordResetToken, error)
	MarkTokenAsUsed(ctx context.Context, id uuid.UUID) error
	InvalidateUserTokens(ctx context.Context, userID uuid.UUID) error
}

type EmailSender interface {
	SendPasswordResetEmail(ctx context.Context, to, resetToken string) error
}

type AuthService struct {
	userRepo  UserRepository
	tokenRepo TokenRepository
	emailer   EmailSender
	jwtSecret string
}

func NewAuthService(userRepo UserRepository, tokenRepo TokenRepository, emailer EmailSender, jwtSecret string) *AuthService {
	return &AuthService{
		userRepo:  userRepo,
		tokenRepo: tokenRepo,
		emailer:   emailer,
		jwtSecret: jwtSecret,
	}
}

func (s *AuthService) Register(ctx context.Context, req model.RegisterRequest) (*model.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user, err := s.userRepo.CreateUser(ctx, req.Email, string(hash))
	if err != nil {
		if errors.Is(err, ErrEmailExists) {
			return nil, ErrEmailExists
		}
		return nil, fmt.Errorf("create user: %w", err)
	}

	return user, nil
}

func (s *AuthService) Login(ctx context.Context, req model.LoginRequest) (*model.LoginResponse, error) {
	user, err := s.userRepo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, ErrInvalidPassword
		}
		return nil, fmt.Errorf("get user: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, ErrInvalidPassword
	}

	token, err := s.generateJWT(user)
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	return &model.LoginResponse{
		Token: token,
		User:  *user,
	}, nil
}

func (s *AuthService) RequestPasswordReset(ctx context.Context, req model.ForgotPasswordRequest) error {
	user, err := s.userRepo.GetUserByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			// Return nil to prevent email enumeration
			return nil
		}
		return fmt.Errorf("get user: %w", err)
	}

	rawToken := make([]byte, 32)
	if _, err := rand.Read(rawToken); err != nil {
		return fmt.Errorf("generate token: %w", err)
	}
	tokenStr := hex.EncodeToString(rawToken)

	if err := s.tokenRepo.InvalidateUserTokens(ctx, user.ID); err != nil {
		return fmt.Errorf("invalidate old tokens: %w", err)
	}

	expiresAt := time.Now().UTC().Add(1 * time.Hour)
	if err := s.tokenRepo.CreateToken(ctx, user.ID, tokenStr, expiresAt); err != nil {
		return fmt.Errorf("save token: %w", err)
	}

	if err := s.emailer.SendPasswordResetEmail(ctx, user.Email, tokenStr); err != nil {
		slog.ErrorContext(ctx, "send password reset email", "error", err, "to", user.Email)
	}

	return nil
}

func (s *AuthService) ResetPassword(ctx context.Context, req model.ResetPasswordRequest) error {
	token, err := s.tokenRepo.GetTokenByToken(ctx, req.Token)
	if err != nil {
		if errors.Is(err, ErrInvalidToken) {
			return ErrInvalidToken
		}
		return fmt.Errorf("get token: %w", err)
	}

	if token.UsedAt != nil {
		return ErrInvalidToken
	}

	if time.Now().UTC().After(token.ExpiresAt) {
		return ErrTokenExpired
	}

	if len(req.Password) < 8 {
		return ErrPasswordTooShort
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	if err := s.userRepo.UpdatePassword(ctx, token.UserID, string(hash)); err != nil {
		return fmt.Errorf("update password: %w", err)
	}

	if err := s.tokenRepo.InvalidateUserTokens(ctx, token.UserID); err != nil {
		return fmt.Errorf("invalidate tokens: %w", err)
	}

	return nil
}

func (s *AuthService) generateJWT(user *model.User) (string, error) {
	claims := jwt.MapClaims{
		"sub":   user.ID.String(),
		"email": user.Email,
		"iat":   time.Now().UTC().Unix(),
		"exp":   time.Now().UTC().Add(24 * time.Hour).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.jwtSecret))
}

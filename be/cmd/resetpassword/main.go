// Command resetpassword sets a new password for an existing user directly in
// the database, bypassing the email reset flow.
//
// Run it from be/ so .env is picked up:
//
//	go run ./cmd/resetpassword -email user@example.com
//
// When -password is omitted the password is read from stdin, which keeps it out
// of shell history.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"

	appconfig "goskeleton/internal/config"
	"goskeleton/internal/db"
	"goskeleton/internal/service"
)

const minPasswordLength = 8

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "reset password:", err)
		os.Exit(1)
	}
}

func run() error {
	email := flag.String("email", "", "email address of the account to update")
	password := flag.String("password", "", "new password (min 8 chars); read from stdin when empty")
	flag.Parse()

	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("load .env: %w", err)
	}

	normalizedEmail := strings.ToLower(strings.TrimSpace(*email))
	if normalizedEmail == "" {
		return errors.New("-email is required")
	}

	newPassword := *password
	if newPassword == "" {
		fmt.Fprint(os.Stderr, "New password: ")
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			return fmt.Errorf("read password: %w", err)
		}
		newPassword = strings.TrimRight(line, "\r\n")
	}

	if len(newPassword) < minPasswordLength {
		return fmt.Errorf("password must be at least %d characters", minPasswordLength)
	}

	cfg, err := appconfig.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := db.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer client.Close()

	if err := client.Ping(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	userRepo := db.NewUserRepo(client.Pool())
	user, err := userRepo.GetUserByEmail(ctx, normalizedEmail)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			return fmt.Errorf("no account found for %s", normalizedEmail)
		}
		return fmt.Errorf("look up user: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	if err := userRepo.UpdatePassword(ctx, user.ID, string(hash)); err != nil {
		return fmt.Errorf("update password: %w", err)
	}

	if err := db.NewTokenRepo(client.Pool()).InvalidateUserTokens(ctx, user.ID); err != nil {
		return fmt.Errorf("invalidate reset tokens: %w", err)
	}

	fmt.Printf("Password updated for %s. Pending reset links are now invalid.\n", user.Email)
	return nil
}

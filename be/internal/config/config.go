package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// defaultJWTSecret is the historical placeholder. It must never reach a
// running server because signing tokens with a public value is equivalent to
// having no authentication at all.
const defaultJWTSecret = "your-super-secret-jwt-key-change-in-production"

type Config struct {
	DatabaseURL  string
	Port         string
	JWTSecret    string
	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPassword string
	SMTPFrom     string
	FrontendURL  string

	LLMBaseURL        string
	LLMModel          string
	LLMAPIKey         string
	LLMTimeoutSeconds int
	LLMMaxTokens      int

	SmartImportMode       string
	SmartImportSampleRows int

	AttachmentsDir string

	Warnings []string
}

func Load() (*Config, error) {
	port := getEnv("PORT", "8080")

	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5432")
	dbUser := getEnv("DB_USER", "root")
	dbPassword := getEnv("DB_PASSWORD", "example")
	dbName := getEnv("DB_NAME", "goskeleton")

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable", dbUser, dbPassword, dbHost, dbPort, dbName)
	}

	smtpPort, err := strconv.Atoi(getEnv("SMTP_PORT", "587"))
	if err != nil {
		smtpPort = 587
	}

	llmTimeout, err := strconv.Atoi(getEnv("LLM_TIMEOUT_SECONDS", "60"))
	if err != nil || llmTimeout <= 0 {
		llmTimeout = 60
	}

	llmMaxTokens, err := strconv.Atoi(getEnv("LLM_MAX_TOKENS", "1024"))
	if err != nil || llmMaxTokens <= 0 {
		llmMaxTokens = 1024
	}

	smartImportMode := strings.ToLower(strings.TrimSpace(getEnv("SMART_IMPORT_MODE", "ai")))
	if smartImportMode != "ai" && smartImportMode != "strict" {
		smartImportMode = "ai"
	}

	smartImportSampleRows, err := strconv.Atoi(getEnv("SMART_IMPORT_SAMPLE_ROWS", "30"))
	if err != nil || smartImportSampleRows < 5 {
		smartImportSampleRows = 30
	}
	if smartImportSampleRows > 50 {
		smartImportSampleRows = 50
	}

	cfg := &Config{
		DatabaseURL:  databaseURL,
		Port:         port,
		JWTSecret:    os.Getenv("JWT_SECRET"),
		SMTPHost:     getEnv("SMTP_HOST", "smtp.gmail.com"),
		SMTPPort:     smtpPort,
		SMTPUser:     os.Getenv("SMTP_USER"),
		SMTPPassword: os.Getenv("SMTP_PASSWORD"),
		SMTPFrom:     getEnv("SMTP_FROM", "noreply@goskeleton.local"),
		FrontendURL:  getEnv("FRONTEND_URL", "http://localhost:3000"),

		LLMBaseURL:        getEnv("LLM_BASE_URL", "https://api.cfrouter.com/v1"),
		LLMModel:          getEnv("LLM_MODEL", "deepseek-v4.1-flash-1017"),
		LLMAPIKey:         os.Getenv("LLM_API_KEY"),
		LLMTimeoutSeconds: llmTimeout,
		LLMMaxTokens:      llmMaxTokens,

		SmartImportMode:       smartImportMode,
		SmartImportSampleRows: smartImportSampleRows,

		AttachmentsDir: getEnv("ATTACHMENTS_DIR", "./data/attachments"),
	}

	if err := validateJWTSecret(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func validateJWTSecret(cfg *Config) error {
	secret := strings.TrimSpace(cfg.JWTSecret)
	if secret == "" {
		return errors.New("JWT_SECRET must be set")
	}
	if secret == defaultJWTSecret {
		return errors.New("JWT_SECRET must not use the default placeholder value")
	}
	if len(secret) < 32 {
		cfg.Warnings = append(cfg.Warnings, "JWT_SECRET is shorter than 32 characters; use a longer random secret in production")
	}
	cfg.JWTSecret = secret

	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

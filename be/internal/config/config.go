package config

import (
	"fmt"
	"os"
	"strconv"
)

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

	cfg := &Config{
		DatabaseURL:  databaseURL,
		Port:         port,
		JWTSecret:    getEnv("JWT_SECRET", "your-super-secret-jwt-key-change-in-production"),
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
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

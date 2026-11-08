package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	stdhttp "net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	_ "goskeleton/docs"
	attachmentstore "goskeleton/internal/attachment"
	appconfig "goskeleton/internal/config"
	"goskeleton/internal/db"
	httpserver "goskeleton/internal/http"
	"goskeleton/internal/llm"
	"goskeleton/internal/logging"
	"goskeleton/internal/service"
)

func main() {
	if err := loadDotEnv(); err != nil {
		panic(fmt.Errorf("load .env: %w", err))
	}

	cfg, err := appconfig.Load()
	if err != nil {
		panic(fmt.Errorf("load config: %w", err))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger, cleanupLogger, err := logging.New("logs")
	if err != nil {
		panic(fmt.Errorf("create logger: %w", err))
	}
	defer func() {
		if err := cleanupLogger(); err != nil {
			logger.Error("close logger", "error", err)
		}
	}()

	for _, warning := range cfg.Warnings {
		logger.Warn(warning)
	}

	attachmentStore, err := attachmentstore.NewLocalStore(cfg.AttachmentsDir)
	if err != nil {
		logger.Error("create attachment store", "error", err, "dir", cfg.AttachmentsDir)
		os.Exit(1)
	}

	dbClient, err := db.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("create database client", "error", err)
		os.Exit(1)
	}
	defer dbClient.Close()

	// Run migrations
	if err := db.RunMigrations(cfg.DatabaseURL); err != nil {
		logger.Error("run database migrations", "error", err)
		os.Exit(1)
	}
	logger.Info("database migrations completed")

	healthService := service.NewHealth(dbClient)

	// Setup auth dependencies
	userRepo := db.NewUserRepo(dbClient.Pool())
	tokenRepo := db.NewTokenRepo(dbClient.Pool())
	emailConfig := service.EmailConfig{
		Host:        cfg.SMTPHost,
		Port:        cfg.SMTPPort,
		Username:    cfg.SMTPUser,
		Password:    cfg.SMTPPassword,
		From:        cfg.SMTPFrom,
		FrontendURL: cfg.FrontendURL,
	}
	emailService := service.NewEmailService(emailConfig, logger)
	authService := service.NewAuthService(userRepo, tokenRepo, emailService, cfg.JWTSecret)

	// Setup finance dependencies
	accountRepo := db.NewAccountRepo(dbClient.Pool())
	categoryRepo := db.NewCategoryRepo(dbClient.Pool())
	transactionRepo := db.NewTransactionRepo(dbClient.Pool())

	accountService := service.NewAccountService(accountRepo)
	categoryService := service.NewCategoryService(categoryRepo)
	transactionService := service.NewTransactionService(transactionRepo, accountRepo, categoryRepo)
	attachmentService := service.NewAttachmentService(db.NewAttachmentRepo(dbClient.Pool()), transactionService, attachmentStore)

	budgetService := service.NewBudgetService(db.NewBudgetRepo(dbClient.Pool()))
	recurringService := service.NewRecurringService(db.NewRecurringRepo(dbClient.Pool()), transactionService)
	reportService := service.NewReportService(db.NewReportRepo(dbClient.Pool()))

	llmClient := llm.NewHTTPClient(
		cfg.LLMBaseURL,
		cfg.LLMAPIKey,
		cfg.LLMModel,
		cfg.LLMMaxTokens,
		time.Duration(cfg.LLMTimeoutSeconds)*time.Second,
	)
	chatHistoryRepo := db.NewChatHistoryRepo(dbClient.Pool())
	chatService := service.NewChatService(accountService, categoryService, transactionService, budgetService, recurringService, reportService, chatHistoryRepo, llmClient, logger)

	importService := service.NewImportService(
		llmClient,
		transactionService,
		accountRepo,
		categoryRepo,
		service.ImportSettings{
			Mode:       service.ImportMode(cfg.SmartImportMode),
			SampleRows: cfg.SmartImportSampleRows,
		},
		logger,
	)

	handler := httpserver.NewHandler(
		healthService,
		authService,
		accountService,
		categoryService,
		transactionService,
		importService,
		service.ImportMode(cfg.SmartImportMode),
		budgetService,
		recurringService,
		reportService,
		chatService,
		attachmentService,
		cfg.JWTSecret,
		buildAllowedOrigins(cfg.FrontendURL),
	)

	port := cfg.Port
	if port == "" {
		port = "8080"
	}

	server := newServer(":"+port, handler)

	go func() {
		logger.Info("http server started", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != stdhttp.ErrServerClosed {
			logger.Error("http server failed", "error", err)
			stop()
		}
	}()

	go runRecurringScheduler(ctx, logger, recurringService)

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("http server shutdown failed", "error", err)
		os.Exit(1)
	}

	logger.Info("http server stopped")
}

func newServer(addr string, handler stdhttp.Handler) *stdhttp.Server {
	return &stdhttp.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      300 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
}

func loadDotEnv() error {
	err := godotenv.Load()
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}

	return err
}

func buildAllowedOrigins(frontendURL string) []string {
	if strings.TrimSpace(frontendURL) == "" {
		return []string{"*"}
	}

	origins := make([]string, 0, 3)
	seen := make(map[string]struct{}, 3)

	add := func(origin string) {
		origin = strings.TrimSpace(origin)
		if origin == "" {
			return
		}
		if _, ok := seen[origin]; ok {
			return
		}
		seen[origin] = struct{}{}
		origins = append(origins, origin)
	}

	for _, origin := range strings.Split(frontendURL, ",") {
		add(origin)
	}
	add("http://localhost:5173")
	add("http://127.0.0.1:5173")

	return origins
}

func runRecurringScheduler(ctx context.Context, logger *slog.Logger, recurring service.RecurringService) {
	run := func() {
		posted, err := recurring.RunDue(ctx, time.Now().UTC())
		if err != nil {
			logger.Error("run recurring transactions", "error", err)
		}
		if posted > 0 {
			logger.Info("posted recurring transactions", "count", posted)
		}
	}

	run()

	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}

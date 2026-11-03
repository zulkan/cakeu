package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"goskeleton/internal/llm"
	"goskeleton/internal/model"
	"goskeleton/internal/spreadsheet"
)

type ImportMode string

const (
	ImportModeAI     ImportMode = "ai"
	ImportModeStrict ImportMode = "strict"
)

var ErrImportAIUnavailable = errors.New("AI could not read this sheet")

type ImportSettings struct {
	Mode       ImportMode
	SampleRows int
}

type ImportOptions struct {
	Filename string
	Sheet    string
	Mode     ImportMode
	Mapping  *model.ImportMapping
}

type ImportService interface {
	Preview(ctx context.Context, userID uuid.UUID, table *spreadsheet.Table, sheetNames []string, opts ImportOptions) (*model.ImportResult, error)
	Import(ctx context.Context, userID uuid.UUID, table *spreadsheet.Table, sheetNames []string, opts ImportOptions) (*model.ImportResult, error)
}

type importService struct {
	client       llm.Client
	transactions TransactionService
	accounts     AccountRepository
	categories   CategoryRepository
	settings     ImportSettings
	logger       *slog.Logger
}

func NewImportService(
	client llm.Client,
	transactions TransactionService,
	accounts AccountRepository,
	categories CategoryRepository,
	settings ImportSettings,
	logger *slog.Logger,
) ImportService {
	if logger == nil {
		logger = slog.Default()
	}
	if settings.SampleRows <= 0 {
		settings.SampleRows = 30
	}
	if settings.Mode != ImportModeStrict {
		settings.Mode = ImportModeAI
	}
	return &importService{
		client:       client,
		transactions: transactions,
		accounts:     accounts,
		categories:   categories,
		settings:     settings,
		logger:       logger,
	}
}

func (s *importService) Preview(ctx context.Context, userID uuid.UUID, table *spreadsheet.Table, sheetNames []string, opts ImportOptions) (*model.ImportResult, error) {
	if s.effectiveMode(opts.Mode) == ImportModeStrict {
		return s.previewStrict(ctx, userID, table, sheetNames)
	}
	if s.client == nil {
		s.logger.WarnContext(ctx, "import ai unavailable; using strict import", "sheet", table.Name, "cause", llm.ErrNotConfigured)
		return s.previewStrict(ctx, userID, table, sheetNames)
	}

	plan, err := s.mapSheet(ctx, userID, opts.Filename, table)
	if err != nil {
		if errors.Is(err, llm.ErrNotConfigured) {
			s.logger.WarnContext(ctx, "import ai unavailable; using strict import", "sheet", table.Name, "cause", err)
			return s.previewStrict(ctx, userID, table, sheetNames)
		}
		return nil, err
	}

	return s.previewWithPlan(ctx, userID, table, sheetNames, plan)
}

func (s *importService) Import(ctx context.Context, userID uuid.UUID, table *spreadsheet.Table, sheetNames []string, opts ImportOptions) (*model.ImportResult, error) {
	if s.effectiveMode(opts.Mode) == ImportModeStrict {
		return s.importStrict(ctx, userID, table, sheetNames)
	}
	if opts.Mapping == nil {
		return nil, fmt.Errorf("%w: mapping is required for AI imports", ErrInvalidMapping)
	}
	return s.importWithPlan(ctx, userID, table, sheetNames, opts.Mapping)
}

func (s *importService) effectiveMode(requested ImportMode) ImportMode {
	if s.settings.Mode == ImportModeStrict {
		return ImportModeStrict
	}
	if requested == ImportModeStrict {
		return ImportModeStrict
	}
	return ImportModeAI
}

func (s *importService) previewStrict(ctx context.Context, userID uuid.UUID, table *spreadsheet.Table, sheetNames []string) (*model.ImportResult, error) {
	csvText, err := table.ToCSV()
	if err != nil {
		return nil, fmt.Errorf("encode sheet: %w", err)
	}
	result, err := s.transactions.PreviewImport(ctx, userID, strings.NewReader(csvText))
	if err != nil {
		return nil, err
	}
	result.Mode = string(ImportModeStrict)
	result.Sheet = table.Name
	result.Sheets = namesOrEmpty(sheetNames)
	return result, nil
}

func (s *importService) importStrict(ctx context.Context, userID uuid.UUID, table *spreadsheet.Table, sheetNames []string) (*model.ImportResult, error) {
	csvText, err := table.ToCSV()
	if err != nil {
		return nil, fmt.Errorf("encode sheet: %w", err)
	}
	result, err := s.transactions.ImportTransactions(ctx, userID, strings.NewReader(csvText))
	if err != nil {
		return nil, err
	}
	result.Mode = string(ImportModeStrict)
	result.Sheet = table.Name
	result.Sheets = namesOrEmpty(sheetNames)
	return result, nil
}

func (s *importService) previewWithPlan(ctx context.Context, userID uuid.UUID, table *spreadsheet.Table, sheetNames []string, plan *model.ImportMapping) (*model.ImportResult, error) {
	csvText, origins, skipped, err := s.applyPlanForUser(ctx, userID, table, plan)
	if err != nil {
		return nil, err
	}
	result, err := s.transactions.PreviewImport(ctx, userID, strings.NewReader(csvText))
	if err != nil {
		return nil, err
	}
	RenumberImportResult(result, origins)
	result.Skipped = skipped
	result.Mode = string(ImportModeAI)
	result.Mapping = resolvedMappingForResult(plan, table)
	result.Sheet = table.Name
	result.Sheets = namesOrEmpty(sheetNames)
	return result, nil
}

func (s *importService) importWithPlan(ctx context.Context, userID uuid.UUID, table *spreadsheet.Table, sheetNames []string, plan *model.ImportMapping) (*model.ImportResult, error) {
	csvText, origins, skipped, err := s.applyPlanForUser(ctx, userID, table, plan)
	if err != nil {
		return nil, err
	}
	result, err := s.transactions.ImportTransactions(ctx, userID, strings.NewReader(csvText))
	if err != nil {
		return nil, err
	}
	RenumberImportResult(result, origins)
	result.Skipped = skipped
	result.Mode = string(ImportModeAI)
	result.Mapping = resolvedMappingForResult(plan, table)
	result.Sheet = table.Name
	result.Sheets = namesOrEmpty(sheetNames)
	return result, nil
}

// applyPlanForUser runs the same deterministic guard for preview and commit:
// load the caller's vocabulary, validate the plan, validate its constants, then extract.
func (s *importService) applyPlanForUser(ctx context.Context, userID uuid.UUID, table *spreadsheet.Table, plan *model.ImportMapping) (string, []int, int, error) {
	accounts, err := s.accounts.ListByUser(ctx, userID)
	if err != nil {
		return "", nil, 0, fmt.Errorf("list accounts: %w", err)
	}
	categories, err := s.categories.ListByUser(ctx, userID)
	if err != nil {
		return "", nil, 0, fmt.Errorf("list categories: %w", err)
	}
	if err := ValidateMapping(plan, table); err != nil {
		return "", nil, 0, err
	}
	if err := validateMappingVocabulary(plan, accounts, categories); err != nil {
		return "", nil, 0, err
	}
	return ApplyMapping(plan, table)
}

func resolvedMappingForResult(plan *model.ImportMapping, table *spreadsheet.Table) *model.ImportMapping {
	resolved, err := resolveMapping(plan, table)
	if err != nil {
		return plan
	}
	copy := *plan
	copy.DataStart = resolved.dataStart
	copy.DataEnd = resolved.dataEnd
	return &copy
}

func (s *importService) mapSheet(ctx context.Context, userID uuid.UUID, filename string, table *spreadsheet.Table) (*model.ImportMapping, error) {
	accounts, err := s.accounts.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	categories, err := s.categories.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}

	messages := importMappingMessages(buildImportMappingRequest(filename, table, accounts, categories, s.settings.SampleRows))
	for attempt := 0; attempt < 2; attempt++ {
		started := time.Now()
		reply, err := s.client.Complete(ctx, messages, nil)
		if err != nil {
			if errors.Is(err, llm.ErrNotConfigured) {
				return nil, err
			}
			s.logger.WarnContext(ctx, "import ai call failed",
				"sheet", table.Name,
				"attempt", attempt+1,
				"duration_ms", time.Since(started).Milliseconds(),
				"error", err,
			)
			return nil, fmt.Errorf("%w: %w", ErrImportAIUnavailable, err)
		}
		if reply == nil {
			s.logger.WarnContext(ctx, "import ai returned no message",
				"sheet", table.Name,
				"attempt", attempt+1,
				"duration_ms", time.Since(started).Milliseconds(),
			)
			return nil, fmt.Errorf("%w: the model returned no message", ErrImportAIUnavailable)
		}

		plan, err := parseImportMappingResponse(reply.Content)
		if err == nil {
			err = ValidateMapping(plan, table)
		}
		if err == nil {
			err = validateMappingVocabulary(plan, accounts, categories)
		}
		if err == nil {
			s.logger.InfoContext(ctx, "import mapping applied",
				"sheet", table.Name,
				"header_row", plan.HeaderRow,
				"confidence", plan.Confidence,
				"duration_ms", time.Since(started).Milliseconds(),
			)
			return plan, nil
		}

		if attempt == 0 {
			messages = append(messages,
				llm.Message{Role: "assistant", Content: reply.Content},
				llm.Message{Role: "user", Content: "That answer was invalid: " + err.Error() + ". Return only the JSON object described in the system prompt."},
			)
			continue
		}
		s.logger.WarnContext(ctx, "import ai returned an invalid mapping twice",
			"sheet", table.Name,
			"attempt", attempt+1,
			"duration_ms", time.Since(started).Milliseconds(),
			"error", err,
		)
		return nil, fmt.Errorf("%w: %w", ErrImportAIUnavailable, err)
	}

	return nil, ErrImportAIUnavailable
}

func validateMappingVocabulary(plan *model.ImportMapping, accounts []model.Account, categories []model.Category) error {
	if plan.Fields.Account.Column == 0 {
		name := strings.ToLower(strings.TrimSpace(plan.Fields.Account.Constant))
		found := false
		for _, account := range accounts {
			if strings.ToLower(strings.TrimSpace(account.Name)) == name {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("%w: unknown account %q", ErrInvalidMapping, plan.Fields.Account.Constant)
		}
	}

	if plan.Fields.Category.Column == 0 {
		typeName := strings.ToLower(strings.TrimSpace(plan.Type.Constant))
		name := strings.ToLower(strings.TrimSpace(plan.Fields.Category.Constant))
		found := false
		for _, category := range categories {
			if strings.ToLower(strings.TrimSpace(category.Name)) != name {
				continue
			}
			if typeName != "" && strings.ToLower(strings.TrimSpace(category.Type)) != typeName {
				continue
			}
			found = true
			break
		}
		if !found {
			if typeName != "" {
				return fmt.Errorf("%w: unknown category %q for type %q", ErrInvalidMapping, plan.Fields.Category.Constant, typeName)
			}
			return fmt.Errorf("%w: unknown category %q", ErrInvalidMapping, plan.Fields.Category.Constant)
		}
	}

	return nil
}

func namesOrEmpty(names []string) []string {
	if names == nil {
		return []string{}
	}
	return names
}

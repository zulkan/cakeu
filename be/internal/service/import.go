package service

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"goskeleton/internal/model"
)

// MaxImportRows caps how many data rows a single import may contain.
const MaxImportRows = 5000

// MaxImportPreviewRows caps how many validated rows are echoed back to the client.
const MaxImportPreviewRows = 100

var (
	ErrImportTooManyRows = errors.New("import has too many rows")
	ErrImportMissingFile = errors.New("import file is required")
)

type importCandidate struct {
	preview    model.ImportPreviewRow
	accountID  uuid.UUID
	categoryID uuid.UUID
	date       time.Time
}

func (s *transactionService) PreviewImport(ctx context.Context, userID uuid.UUID, r io.Reader) (*model.ImportResult, error) {
	_, result, err := s.parseImport(ctx, userID, r)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *transactionService) ImportTransactions(ctx context.Context, userID uuid.UUID, r io.Reader) (*model.ImportResult, error) {
	candidates, result, err := s.parseImport(ctx, userID, r)
	if err != nil {
		return nil, err
	}

	if len(candidates) == 0 {
		return result, nil
	}

	transactions := make([]model.Transaction, 0, len(candidates))
	for _, candidate := range candidates {
		transactions = append(transactions, model.Transaction{
			UserID:      userID,
			AccountID:   candidate.accountID,
			CategoryID:  candidate.categoryID,
			Type:        candidate.preview.Type,
			Amount:      candidate.preview.Amount,
			Description: candidate.preview.Description,
			Date:        candidate.date,
		})
	}

	if err := s.repo.CreateBatch(ctx, userID, transactions); err != nil {
		return nil, fmt.Errorf("import transactions: %w", err)
	}

	return result, nil
}

func (s *transactionService) parseImport(ctx context.Context, userID uuid.UUID, r io.Reader) ([]importCandidate, *model.ImportResult, error) {
	accounts, err := s.accounts.ListByUser(ctx, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("list accounts: %w", err)
	}

	categories, err := s.categories.ListByUser(ctx, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("list categories: %w", err)
	}

	accountIDs := make(map[string]uuid.UUID, len(accounts))
	for _, account := range accounts {
		accountIDs[strings.ToLower(strings.TrimSpace(account.Name))] = account.ID
	}

	categoryIDs := make(map[string]uuid.UUID, len(categories))
	for _, category := range categories {
		categoryIDs[categoryKey(category.Name, category.Type)] = category.ID
	}

	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1

	header, err := reader.Read()
	if err != nil {
		return nil, nil, fmt.Errorf("read header: %w", err)
	}

	columns := make(map[string]int, len(header))
	for i, name := range header {
		columns[strings.ToLower(strings.TrimSpace(strings.TrimPrefix(name, "\ufeff")))] = i
	}

	for _, required := range []string{"date", "type", "amount", "description", "account", "category"} {
		if _, ok := columns[required]; !ok {
			return nil, nil, fmt.Errorf("missing column %q", required)
		}
	}

	result := &model.ImportResult{
		Errors:  []model.ImportRowError{},
		Preview: []model.ImportPreviewRow{},
	}
	candidates := make([]importCandidate, 0, 16)

	line := 1
	for {
		record, readErr := reader.Read()
		if errors.Is(readErr, io.EOF) {
			break
		}
		line++

		if readErr != nil {
			result.Total++
			result.Invalid++
			result.Errors = append(result.Errors, model.ImportRowError{Row: line, Message: "malformed row: " + readErr.Error()})
			continue
		}

		result.Total++
		if result.Total > MaxImportRows {
			return nil, nil, ErrImportTooManyRows
		}

		candidate, rowErr := buildImportCandidate(record, columns, accountIDs, categoryIDs, line)
		if rowErr != nil {
			result.Invalid++
			result.Errors = append(result.Errors, model.ImportRowError{Row: line, Message: rowErr.Error()})
			continue
		}

		candidates = append(candidates, candidate)
		if len(result.Preview) < MaxImportPreviewRows {
			result.Preview = append(result.Preview, candidate.preview)
		}
	}

	result.Valid = len(candidates)
	result.Invalid = result.Total - result.Valid

	return candidates, result, nil
}

func buildImportCandidate(record []string, columns map[string]int, accountIDs map[string]uuid.UUID, categoryIDs map[string]uuid.UUID, line int) (importCandidate, error) {
	value := func(name string) string {
		index, ok := columns[name]
		if !ok || index >= len(record) {
			return ""
		}
		return strings.TrimSpace(record[index])
	}

	dateValue := value("date")
	date, err := time.Parse("2006-01-02", dateValue)
	if err != nil {
		return importCandidate{}, fmt.Errorf("invalid date %q", dateValue)
	}

	typeValue := strings.ToLower(value("type"))
	if !isValidType(typeValue) {
		return importCandidate{}, fmt.Errorf("invalid type %q", value("type"))
	}

	amountValue := value("amount")
	amount, err := strconv.ParseFloat(amountValue, 64)
	if err != nil || amount <= 0 {
		return importCandidate{}, fmt.Errorf("invalid amount %q", amountValue)
	}

	accountName := value("account")
	accountID, ok := accountIDs[strings.ToLower(accountName)]
	if !ok {
		return importCandidate{}, fmt.Errorf("unknown account %q", accountName)
	}

	categoryName := value("category")
	categoryID, ok := categoryIDs[categoryKey(categoryName, typeValue)]
	if !ok {
		return importCandidate{}, fmt.Errorf("unknown category %q for type %q", categoryName, typeValue)
	}

	return importCandidate{
		preview: model.ImportPreviewRow{
			Row:         line,
			Date:        date.Format("2006-01-02"),
			Type:        typeValue,
			Amount:      amount,
			Description: value("description"),
			Account:     accountName,
			Category:    categoryName,
		},
		accountID:  accountID,
		categoryID: categoryID,
		date:       date,
	}, nil
}

func categoryKey(name, type_ string) string {
	return strings.ToLower(strings.TrimSpace(name)) + "|" + strings.ToLower(strings.TrimSpace(type_))
}

package service

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
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
	ErrImportTooManyRows    = errors.New("import has too many rows")
	ErrImportMissingFile    = errors.New("import file is required")
	ErrImportMissingColumns = errors.New("missing required columns")
)

type importCandidate struct {
	preview         model.ImportPreviewRow
	accountID       uuid.UUID
	categoryID      uuid.UUID
	date            time.Time
	duplicateInFile bool
	duplicateInDB   bool
}

func (s *transactionService) PreviewImport(ctx context.Context, userID uuid.UUID, r io.Reader) (*model.ImportResult, error) {
	_, result, err := s.parseImport(ctx, userID, r)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *transactionService) ImportTransactions(ctx context.Context, userID uuid.UUID, r io.Reader, allowDuplicates bool) (*model.ImportResult, error) {
	candidates, result, err := s.parseImport(ctx, userID, r)
	if err != nil {
		return nil, err
	}

	batch := make([]model.Transaction, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.duplicateInFile {
			continue
		}
		if candidate.duplicateInDB && !allowDuplicates {
			continue
		}
		batch = append(batch, model.Transaction{
			UserID:      userID,
			AccountID:   candidate.accountID,
			CategoryID:  candidate.categoryID,
			Type:        candidate.preview.Type,
			Amount:      candidate.preview.Amount,
			Description: candidate.preview.Description,
			Date:        candidate.date,
		})
	}
	result.Imported = len(batch)

	if len(batch) == 0 {
		return result, nil
	}

	if err := s.repo.CreateBatch(ctx, userID, batch); err != nil {
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
			return nil, nil, fmt.Errorf("%w: missing column %q", ErrImportMissingColumns, required)
		}
	}

	result := &model.ImportResult{
		Errors:     []model.ImportRowError{},
		Duplicates: []model.ImportRowError{},
		Preview:    []model.ImportPreviewRow{},
	}
	candidates := make([]importCandidate, 0, 16)

	line := 1
	previousDate := ""
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

		// A blank date means "same as the row above" in many ledger exports.
		// Keep the last explicit, valid date so later rows can inherit it even
		// when this row fails another validation.
		if dateValue := importFieldValue(record, columns, "date"); dateValue != "" {
			if parsed, parseErr := time.Parse("2006-01-02", dateValue); parseErr == nil {
				previousDate = parsed.Format("2006-01-02")
			}
		}

		candidate, rowErr := buildImportCandidate(record, columns, accountIDs, categoryIDs, line, previousDate)
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

	if err := s.markDuplicates(ctx, userID, candidates, result); err != nil {
		return nil, nil, err
	}

	return candidates, result, nil
}

// markDuplicates flags candidates whose fingerprint already exists in the
// ledger or appeared earlier in the same file. The first occurrence in a file
// wins, so later identical rows are marked as in-file duplicates.
func (s *transactionService) markDuplicates(ctx context.Context, userID uuid.UUID, candidates []importCandidate, result *model.ImportResult) error {
	if len(candidates) == 0 {
		return nil
	}

	from, to := candidates[0].date, candidates[0].date
	for _, candidate := range candidates[1:] {
		if candidate.date.Before(from) {
			from = candidate.date
		}
		if candidate.date.After(to) {
			to = candidate.date
		}
	}

	existing, err := s.repo.FingerprintCandidates(ctx, userID, from, to)
	if err != nil {
		return fmt.Errorf("load existing fingerprints: %w", err)
	}

	dbSet := make(map[string]struct{}, len(existing))
	for _, fingerprint := range existing {
		dbSet[fingerprintKey(userID, fingerprint.Date, fingerprint.Type, fingerprint.Amount, fingerprint.Description, fingerprint.AccountName)] = struct{}{}
	}

	seen := make(map[string]struct{}, len(candidates))
	for i := range candidates {
		key := candidateFingerprint(userID, candidates[i])
		_, inFile := seen[key]
		_, inDB := dbSet[key]
		seen[key] = struct{}{}

		message := ""
		if inFile {
			candidates[i].duplicateInFile = true
			message = "duplicate row in file"
		} else if inDB {
			candidates[i].duplicateInDB = true
			message = "duplicate of an existing transaction"
		}
		if message == "" {
			continue
		}

		result.DuplicateCount++
		result.Duplicates = append(result.Duplicates, model.ImportRowError{Row: candidates[i].preview.Row, Message: message})
		if i < len(result.Preview) {
			result.Preview[i].Duplicate = true
		}
	}

	return nil
}

func candidateFingerprint(userID uuid.UUID, candidate importCandidate) string {
	return fingerprintKey(userID, candidate.date, candidate.preview.Type, candidate.preview.Amount, candidate.preview.Description, candidate.preview.Account)
}

func fingerprintKey(userID uuid.UUID, date time.Time, type_ string, amount float64, description, accountName string) string {
	payload := strings.Join([]string{
		userID.String(),
		date.Format("2006-01-02"),
		type_,
		strconv.FormatFloat(amount, 'f', 2, 64),
		normalizeFingerprintText(description),
		normalizeFingerprintText(accountName),
	}, "|")
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func normalizeFingerprintText(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}

func buildImportCandidate(record []string, columns map[string]int, accountIDs map[string]uuid.UUID, categoryIDs map[string]uuid.UUID, line int, previousDate string) (importCandidate, error) {
	value := func(name string) string {
		return importFieldValue(record, columns, name)
	}

	dateValue := value("date")
	if dateValue == "" {
		dateValue = previousDate
	}
	if dateValue == "" {
		return importCandidate{}, errors.New("date is empty and there is no previous date to carry forward")
	}

	date, err := time.Parse("2006-01-02", dateValue)
	if err != nil {
		return importCandidate{}, fmt.Errorf("invalid date %q (expected YYYY-MM-DD)", dateValue)
	}

	typeValue := strings.ToLower(value("type"))
	if !isValidType(typeValue) {
		return importCandidate{}, fmt.Errorf("invalid type %q", value("type"))
	}

	amountValue := value("amount")
	amount, err := strconv.ParseFloat(amountValue, 64)
	if err != nil || math.IsNaN(amount) || math.IsInf(amount, 0) || amount <= 0 {
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

func importFieldValue(record []string, columns map[string]int, name string) string {
	index, ok := columns[name]
	if !ok || index >= len(record) {
		return ""
	}
	return strings.TrimSpace(record[index])
}

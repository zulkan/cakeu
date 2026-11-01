package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"

	"goskeleton/internal/model"
	"goskeleton/internal/spreadsheet"
)

var (
	ErrAttachmentNotFound = errors.New("the attached file is no longer available")
	ErrUnknownSheet       = errors.New("unknown sheet")
)

const (
	chatFileTTL         = 30 * time.Minute
	chatFilePreviewRows = 10
	chatContextRows     = 200
	maxFilesPerUser     = 3
)

type storedSheet struct {
	name    string
	csv     string
	rows    int
	columns []string
}

type storedChatFile struct {
	userID       uuid.UUID
	name         string
	order        []string
	sheets       map[string]storedSheet
	defaultSheet string
	expiresAt    time.Time
}

func (f storedChatFile) pick(sheet string) (storedSheet, bool) {
	name := strings.TrimSpace(sheet)
	if name == "" {
		name = f.defaultSheet
	}

	selected, ok := f.sheets[strings.ToLower(name)]
	return selected, ok
}

func (s *ChatService) AttachFile(ctx context.Context, userID uuid.UUID, filename string, r io.Reader) (*model.ChatFile, error) {
	workbook, err := spreadsheet.Parse(filename, r)
	if err != nil {
		return nil, err
	}

	defaultTable, err := workbook.Pick("")
	if err != nil {
		return nil, err
	}
	if len(defaultTable.Rows) < 2 {
		return nil, fmt.Errorf("the file needs a header row and at least one data row")
	}

	sheets := make(map[string]storedSheet, len(workbook.Sheets))
	for i := range workbook.Sheets {
		table := workbook.Sheets[i]
		csvText, csvErr := table.ToCSV()
		if csvErr != nil {
			return nil, csvErr
		}

		var columns []string
		if len(table.Rows) > 0 {
			columns = table.Rows[0]
		}

		sheets[strings.ToLower(table.Name)] = storedSheet{
			name:    table.Name,
			csv:     csvText,
			rows:    max(len(table.Rows)-1, 0),
			columns: columns,
		}
	}

	file := storedChatFile{
		userID:       userID,
		name:         filename,
		order:        workbook.Names(),
		sheets:       sheets,
		defaultSheet: defaultTable.Name,
		expiresAt:    time.Now().Add(chatFileTTL),
	}

	id := randomToken()

	s.mu.Lock()
	s.pruneFilesLocked()
	s.enforceFileQuotaLocked(userID)
	s.files[id] = file
	s.mu.Unlock()

	selected, _ := file.pick("")

	sheetRows := make(map[string]int, len(sheets))
	for _, sheet := range sheets {
		sheetRows[sheet.name] = sheet.rows
	}

	return &model.ChatFile{
		ID:        id,
		Name:      filename,
		Sheet:     selected.name,
		Sheets:    file.order,
		SheetRows: sheetRows,
		Rows:      selected.rows,
		Columns:   selected.columns,
		Preview:   previewRows(defaultTable.Rows, chatFilePreviewRows),
	}, nil
}

func (s *ChatService) lookupFile(userID uuid.UUID, id string) (storedChatFile, bool) {
	if id == "" {
		return storedChatFile{}, false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.pruneFilesLocked()

	file, ok := s.files[id]
	if !ok || file.userID != userID {
		return storedChatFile{}, false
	}

	return file, true
}

func (s *ChatService) pruneFilesLocked() {
	now := time.Now()
	for id, file := range s.files {
		if now.After(file.expiresAt) {
			delete(s.files, id)
		}
	}
}

func (s *ChatService) enforceFileQuotaLocked(userID uuid.UUID) {
	count := 0
	oldestID := ""
	var oldestExpiry time.Time

	for id, file := range s.files {
		if file.userID != userID {
			continue
		}
		count++
		if oldestID == "" || file.expiresAt.Before(oldestExpiry) {
			oldestID = id
			oldestExpiry = file.expiresAt
		}
	}

	if count >= maxFilesPerUser && oldestID != "" {
		delete(s.files, oldestID)
	}
}

func (s *ChatService) fileContext(file storedChatFile, sheet string) (string, error) {
	selected, ok := file.pick(sheet)
	if !ok {
		return "", fmt.Errorf("%w %q; available sheets: %s", ErrUnknownSheet, sheet, strings.Join(file.order, ", "))
	}

	lines := strings.Split(strings.TrimRight(selected.csv, "\n"), "\n")
	if len(lines) > chatContextRows+1 {
		lines = lines[:chatContextRows+1]
	}

	return fmt.Sprintf(
		"The user attached %q (sheet %q, %d data rows). CSV content:\n%s",
		file.name, selected.name, selected.rows, strings.Join(lines, "\n"),
	), nil
}

func (s *ChatService) requestImportAttachedFile(ctx context.Context, userID uuid.UUID, rawArgs, fileID, sheet string) (string, string, *model.ChatPendingAction, error) {
	file, ok := s.lookupFile(userID, fileID)
	if !ok {
		return "", "", nil, fmt.Errorf("%w; ask the user to upload it again", ErrAttachmentNotFound)
	}

	if err := decodeArgs(rawArgs, &struct{}{}); err != nil {
		return "", "", nil, err
	}

	selected, ok := file.pick(sheet)
	if !ok {
		return "", "", nil, fmt.Errorf("%w %q; available sheets: %s", ErrUnknownSheet, sheet, strings.Join(file.order, ", "))
	}

	preview, err := s.transactions.PreviewImport(ctx, userID, strings.NewReader(selected.csv))
	if err != nil {
		return "", "", nil, err
	}
	if preview.Valid == 0 {
		return "", "", nil, fmt.Errorf("the attached file has no importable rows")
	}

	summary := fmt.Sprintf("Import %d transactions from %q (%s)?", preview.Valid, file.name, selected.name)
	if preview.Invalid > 0 {
		summary = fmt.Sprintf("Import %d transactions from %q (%s, %d rows skipped)?", preview.Valid, file.name, selected.name, preview.Invalid)
	}

	pending := s.storePendingFile(userID, pendingImportFile, fileID, selected.name, summary)

	return "", summary, &pending, nil
}

func (s *ChatService) storePendingFile(userID uuid.UUID, kind, fileID, sheet, summary string) model.ChatPendingAction {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.pruneLocked()

	id := randomToken()
	s.pending[id] = pendingAction{
		userID:    userID,
		kind:      kind,
		fileID:    fileID,
		sheet:     sheet,
		summary:   summary,
		expiresAt: time.Now().Add(pendingActionTTL),
	}

	return model.ChatPendingAction{ID: id, Summary: summary}
}

func previewRows(rows [][]string, limit int) [][]string {
	if len(rows) <= limit {
		return rows
	}

	return rows[:limit]
}

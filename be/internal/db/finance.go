package db

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"goskeleton/internal/model"
	"goskeleton/internal/service"
)

var (
	_ service.AccountRepository     = (*AccountRepo)(nil)
	_ service.CategoryRepository    = (*CategoryRepo)(nil)
	_ service.TransactionRepository = (*TransactionRepo)(nil)
)

type AccountRepo struct {
	pool *pgxpool.Pool
}

func NewAccountRepo(pool *pgxpool.Pool) *AccountRepo {
	return &AccountRepo{pool: pool}
}

func (r *AccountRepo) Create(ctx context.Context, userID uuid.UUID, name string, openingBalance float64, currency string) (*model.Account, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin account: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var accountID uuid.UUID
	err = tx.QueryRow(ctx,
		`INSERT INTO accounts (user_id, name, currency) VALUES ($1, $2, $3) RETURNING id`,
		userID, name, currency,
	).Scan(&accountID)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, service.ErrAccountNameExists
		}
		return nil, fmt.Errorf("insert account: %w", err)
	}

	if openingBalance != 0 {
		entryType := service.TypeIncome
		amount := openingBalance
		if openingBalance < 0 {
			entryType = service.TypeExpense
			amount = -openingBalance
		}

		if _, err := tx.Exec(ctx,
			`INSERT INTO transactions (user_id, account_id, category_id, type, amount, description, date, kind)
			 VALUES ($1, $2, NULL, $3, $4, $5, CURRENT_DATE, $6)`,
			userID, accountID, entryType, amount, "Opening balance", service.EntryKindOpening,
		); err != nil {
			return nil, fmt.Errorf("insert opening balance: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit account: %w", err)
	}

	return r.GetByID(ctx, accountID)
}

func (r *AccountRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Account, error) {
	query := `
		SELECT a.id, a.user_id, a.name, a.currency, a.created_at, a.updated_at,
		       COALESCE(SUM(CASE WHEN t.type = 'income' THEN t.amount ELSE -t.amount END), 0) AS balance,
		       COALESCE(SUM(
		           CASE WHEN t.kind = 'opening'
		                THEN CASE WHEN t.type = 'income' THEN t.amount ELSE -t.amount END
		           END
		       ), 0) AS opening_balance,
		       COUNT(t.id) AS entry_count
		FROM accounts a
		LEFT JOIN transactions t ON t.account_id = a.id
		WHERE a.id = $1
		GROUP BY a.id
	`

	var account model.Account
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&account.ID, &account.UserID, &account.Name, &account.Currency,
		&account.CreatedAt, &account.UpdatedAt,
		&account.Balance, &account.OpeningBalance, &account.EntryCount,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, service.ErrAccountNotFound
		}
		return nil, fmt.Errorf("select account: %w", err)
	}

	return &account, nil
}

func (r *AccountRepo) ListByUser(ctx context.Context, userID uuid.UUID) ([]model.Account, error) {
	query := `
		SELECT a.id, a.user_id, a.name, a.currency, a.created_at, a.updated_at,
		       COALESCE(SUM(CASE WHEN t.type = 'income' THEN t.amount ELSE -t.amount END), 0) AS balance,
		       COALESCE(SUM(
		           CASE WHEN t.kind = 'opening'
		                THEN CASE WHEN t.type = 'income' THEN t.amount ELSE -t.amount END
		           END
		       ), 0) AS opening_balance,
		       COUNT(t.id) AS entry_count
		FROM accounts a
		LEFT JOIN transactions t ON t.account_id = a.id
		WHERE a.user_id = $1
		GROUP BY a.id
		ORDER BY a.created_at ASC, a.name ASC
	`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("select accounts: %w", err)
	}
	defer rows.Close()

	accounts := make([]model.Account, 0)
	for rows.Next() {
		var account model.Account
		if err := rows.Scan(
			&account.ID, &account.UserID, &account.Name, &account.Currency,
			&account.CreatedAt, &account.UpdatedAt,
			&account.Balance, &account.OpeningBalance, &account.EntryCount,
		); err != nil {
			return nil, fmt.Errorf("scan account: %w", err)
		}
		accounts = append(accounts, account)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate accounts: %w", err)
	}

	return accounts, nil
}

func (r *AccountRepo) Update(ctx context.Context, userID uuid.UUID, id uuid.UUID, name *string, currency *string) (*model.Account, error) {
	tag, err := r.pool.Exec(ctx, `
		UPDATE accounts a
		SET name = COALESCE($3::varchar, a.name),
		    currency = COALESCE($4::varchar, a.currency),
		    updated_at = NOW()
		WHERE a.id = $1 AND a.user_id = $2
		  AND ($4::varchar IS NULL OR NOT EXISTS (
		      SELECT 1 FROM transactions t WHERE t.account_id = a.id
		  ))
	`, id, userID, name, currency)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, service.ErrAccountNameExists
		}
		return nil, fmt.Errorf("update account: %w", err)
	}
	if tag.RowsAffected() == 0 {
		var exists bool
		if err := r.pool.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM accounts WHERE id = $1 AND user_id = $2)`,
			id, userID,
		).Scan(&exists); err != nil {
			return nil, fmt.Errorf("check account: %w", err)
		}
		if !exists {
			return nil, service.ErrAccountNotFound
		}
		return nil, service.ErrCurrencyLocked
	}

	return r.GetByID(ctx, id)
}

func (r *AccountRepo) AdjustBalance(ctx context.Context, userID uuid.UUID, id uuid.UUID, targetBalance float64, description string) (*model.Account, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin adjustment: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var accountID uuid.UUID
	err = tx.QueryRow(ctx,
		`SELECT id FROM accounts WHERE id = $1 AND user_id = $2 FOR UPDATE`,
		id, userID,
	).Scan(&accountID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, service.ErrAccountNotFound
		}
		return nil, fmt.Errorf("lock account: %w", err)
	}

	var current float64
	if err := tx.QueryRow(ctx,
		`SELECT COALESCE(SUM(CASE WHEN type = 'income' THEN amount ELSE -amount END), 0)
		 FROM transactions WHERE account_id = $1`,
		id,
	).Scan(&current); err != nil {
		return nil, fmt.Errorf("current balance: %w", err)
	}

	delta := math.Round((targetBalance-current)*100) / 100
	if delta != 0 {
		entryType := service.TypeIncome
		amount := delta
		if delta < 0 {
			entryType = service.TypeExpense
			amount = -delta
		}

		if _, err := tx.Exec(ctx,
			`INSERT INTO transactions (user_id, account_id, category_id, type, amount, description, date, kind)
			 VALUES ($1, $2, NULL, $3, $4, $5, CURRENT_DATE, $6)`,
			userID, id, entryType, amount, description, service.EntryKindAdjustment,
		); err != nil {
			return nil, fmt.Errorf("insert adjustment: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit adjustment: %w", err)
	}

	return r.GetByID(ctx, id)
}

func (r *AccountRepo) Delete(ctx context.Context, userID uuid.UUID, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM accounts WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return fmt.Errorf("delete account: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return service.ErrAccountNotFound
	}

	return nil
}

type CategoryRepo struct {
	pool *pgxpool.Pool
}

func NewCategoryRepo(pool *pgxpool.Pool) *CategoryRepo {
	return &CategoryRepo{pool: pool}
}

func (r *CategoryRepo) Create(ctx context.Context, userID uuid.UUID, name string, type_ string, color string) (*model.Category, error) {
	query := `
		INSERT INTO categories (user_id, name, type, color)
		VALUES ($1, $2, $3, $4)
		RETURNING id, user_id, name, type, color, created_at
	`

	var category model.Category
	err := r.pool.QueryRow(ctx, query, userID, name, type_, color).Scan(
		&category.ID, &category.UserID, &category.Name, &category.Type, &category.Color, &category.CreatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, service.ErrCategoryNameExists
		}
		return nil, fmt.Errorf("insert category: %w", err)
	}

	return &category, nil
}

func (r *CategoryRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Category, error) {
	query := `SELECT id, user_id, name, type, color, created_at FROM categories WHERE id = $1`

	var category model.Category
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&category.ID, &category.UserID, &category.Name, &category.Type, &category.Color, &category.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, service.ErrCategoryNotFound
		}
		return nil, fmt.Errorf("select category: %w", err)
	}

	return &category, nil
}

func (r *CategoryRepo) ListByUser(ctx context.Context, userID uuid.UUID) ([]model.Category, error) {
	query := `
		SELECT id, user_id, name, type, color, created_at
		FROM categories
		WHERE user_id = $1
		ORDER BY type ASC, name ASC
	`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("select categories: %w", err)
	}
	defer rows.Close()

	categories := make([]model.Category, 0)
	for rows.Next() {
		var category model.Category
		if err := rows.Scan(
			&category.ID, &category.UserID, &category.Name, &category.Type, &category.Color, &category.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan category: %w", err)
		}
		categories = append(categories, category)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate categories: %w", err)
	}

	return categories, nil
}

func (r *CategoryRepo) Delete(ctx context.Context, userID uuid.UUID, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM categories WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return fmt.Errorf("delete category: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return service.ErrCategoryNotFound
	}

	return nil
}

type TransactionRepo struct {
	pool *pgxpool.Pool
}

func NewTransactionRepo(pool *pgxpool.Pool) *TransactionRepo {
	return &TransactionRepo{pool: pool}
}

func (r *TransactionRepo) Create(ctx context.Context, transaction model.Transaction) (*model.Transaction, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if err := lockAccounts(ctx, tx, transaction.UserID, []uuid.UUID{transaction.AccountID}); err != nil {
		return nil, err
	}

	kind := transaction.Kind
	if kind == "" {
		kind = service.EntryKindNormal
	}

	query := `
		INSERT INTO transactions (user_id, account_id, category_id, type, amount, description, date, kind)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, user_id, account_id, category_id, type, amount, description, date, created_at, kind
	`

	var created model.Transaction
	err = tx.QueryRow(ctx, query,
		transaction.UserID,
		transaction.AccountID,
		transaction.CategoryID,
		transaction.Type,
		transaction.Amount,
		transaction.Description,
		transaction.Date,
		kind,
	).Scan(
		&created.ID,
		&created.UserID,
		&created.AccountID,
		&created.CategoryID,
		&created.Type,
		&created.Amount,
		&created.Description,
		&created.Date,
		&created.CreatedAt,
		&created.Kind,
	)
	if err != nil {
		return nil, fmt.Errorf("insert transaction: %w", err)
	}

	if err := tx.QueryRow(ctx,
		`SELECT a.name, a.currency, COALESCE(c.name, '')
		 FROM accounts a
		 LEFT JOIN categories c ON c.id = $2
		 WHERE a.id = $1`,
		transaction.AccountID, transaction.CategoryID,
	).Scan(&created.AccountName, &created.Currency, &created.CategoryName); err != nil {
		return nil, fmt.Errorf("select transaction labels: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return &created, nil
}

func (r *TransactionRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Transaction, error) {
	query := `
		SELECT t.id, t.user_id, t.account_id,
		       COALESCE(t.category_id, '00000000-0000-0000-0000-000000000000'::uuid),
		       t.type, t.amount, t.description, t.date, t.created_at, t.kind,
		       a.name, a.currency, COALESCE(c.name, '')
		FROM transactions t
		JOIN accounts a ON a.id = t.account_id
		LEFT JOIN categories c ON c.id = t.category_id
		WHERE t.id = $1
	`

	var transaction model.Transaction
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&transaction.ID,
		&transaction.UserID,
		&transaction.AccountID,
		&transaction.CategoryID,
		&transaction.Type,
		&transaction.Amount,
		&transaction.Description,
		&transaction.Date,
		&transaction.CreatedAt,
		&transaction.Kind,
		&transaction.AccountName,
		&transaction.Currency,
		&transaction.CategoryName,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, service.ErrTransactionNotFound
		}
		return nil, fmt.Errorf("select transaction: %w", err)
	}

	return &transaction, nil
}

func (r *TransactionRepo) Update(ctx context.Context, userID uuid.UUID, transaction model.Transaction) (*model.Transaction, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var (
		oldAccountID uuid.UUID
		oldKind      string
	)
	err = tx.QueryRow(ctx,
		`SELECT account_id, kind FROM transactions WHERE id = $1 AND user_id = $2 FOR UPDATE`,
		transaction.ID, userID,
	).Scan(&oldAccountID, &oldKind)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, service.ErrTransactionNotFound
		}
		return nil, fmt.Errorf("select transaction: %w", err)
	}
	if oldKind != "" && oldKind != service.EntryKindNormal {
		return nil, service.ErrSystemEntryImmutable
	}

	if err := lockAccounts(ctx, tx, userID, []uuid.UUID{oldAccountID, transaction.AccountID}); err != nil {
		return nil, err
	}

	query := `
		UPDATE transactions
		SET account_id = $3,
		    category_id = $4,
		    type = $5,
		    amount = $6,
		    description = $7,
		    date = $8
		WHERE id = $1 AND user_id = $2 AND kind = 'normal'
		RETURNING id, user_id, account_id, category_id, type, amount, description, date, created_at, kind
	`

	var updated model.Transaction
	err = tx.QueryRow(ctx, query,
		transaction.ID,
		userID,
		transaction.AccountID,
		transaction.CategoryID,
		transaction.Type,
		transaction.Amount,
		transaction.Description,
		transaction.Date,
	).Scan(
		&updated.ID,
		&updated.UserID,
		&updated.AccountID,
		&updated.CategoryID,
		&updated.Type,
		&updated.Amount,
		&updated.Description,
		&updated.Date,
		&updated.CreatedAt,
		&updated.Kind,
	)
	if err != nil {
		return nil, fmt.Errorf("update transaction: %w", err)
	}

	if err := tx.QueryRow(ctx,
		`SELECT a.name, a.currency, COALESCE(c.name, '')
		 FROM accounts a
		 LEFT JOIN categories c ON c.id = $2
		 WHERE a.id = $1`,
		updated.AccountID, updated.CategoryID,
	).Scan(&updated.AccountName, &updated.Currency, &updated.CategoryName); err != nil {
		return nil, fmt.Errorf("select transaction labels: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return &updated, nil
}

func (r *TransactionRepo) List(ctx context.Context, filters model.TransactionFilters) ([]model.Transaction, int, error) {
	conditions := make([]string, 0, 6)
	args := make([]interface{}, 0, 6)

	addCondition := func(format string, value interface{}) {
		args = append(args, value)
		conditions = append(conditions, fmt.Sprintf(format, len(args)))
	}

	if filters.UserID != nil {
		addCondition("t.user_id = $%d", *filters.UserID)
	}
	if filters.AccountID != nil {
		addCondition("t.account_id = $%d", *filters.AccountID)
	}
	if filters.CategoryID != nil {
		addCondition("t.category_id = $%d", *filters.CategoryID)
	}
	if filters.Type != "" {
		addCondition("t.type = $%d", filters.Type)
	}
	if filters.Kind != "" {
		addCondition("t.kind = $%d", filters.Kind)
	}
	if filters.DateFrom != nil {
		addCondition("t.date >= $%d", *filters.DateFrom)
	}
	if filters.DateTo != nil {
		addCondition("t.date <= $%d", *filters.DateTo)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " WHERE " + strings.Join(conditions, " AND ")
	}

	var total int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM transactions t"+whereClause, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count transactions: %w", err)
	}

	page := filters.Page
	if page < 1 {
		page = 1
	}
	pageSize := filters.PageSize
	if pageSize < 1 {
		pageSize = service.DefaultPageSize
	}
	if pageSize > service.MaxPageSize {
		pageSize = service.MaxPageSize
	}
	offset := (page - 1) * pageSize

	listArgs := make([]interface{}, 0, len(args)+2)
	listArgs = append(listArgs, args...)
	listArgs = append(listArgs, pageSize, offset)

	query := fmt.Sprintf(`
		SELECT t.id, t.user_id, t.account_id,
		       COALESCE(t.category_id, '00000000-0000-0000-0000-000000000000'::uuid),
		       t.type, t.amount, t.description, t.date, t.created_at, t.kind,
		       a.name, a.currency, COALESCE(c.name, '')
		FROM transactions t
		JOIN accounts a ON a.id = t.account_id
		LEFT JOIN categories c ON c.id = t.category_id%s
		ORDER BY t.date DESC, t.created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, len(args)+1, len(args)+2)

	rows, err := r.pool.Query(ctx, query, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("select transactions: %w", err)
	}
	defer rows.Close()

	transactions := make([]model.Transaction, 0, pageSize)
	for rows.Next() {
		var transaction model.Transaction
		if err := rows.Scan(
			&transaction.ID,
			&transaction.UserID,
			&transaction.AccountID,
			&transaction.CategoryID,
			&transaction.Type,
			&transaction.Amount,
			&transaction.Description,
			&transaction.Date,
			&transaction.CreatedAt,
			&transaction.Kind,
			&transaction.AccountName,
			&transaction.Currency,
			&transaction.CategoryName,
		); err != nil {
			return nil, 0, fmt.Errorf("scan transaction: %w", err)
		}
		transactions = append(transactions, transaction)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate transactions: %w", err)
	}

	return transactions, total, nil
}

func (r *TransactionRepo) ForEachExportRow(ctx context.Context, filters model.TransactionFilters, emit func(model.TransactionExportRow) error) error {
	conditions := []string{"t.kind = 'normal'"}
	args := make([]interface{}, 0, 6)

	addCondition := func(format string, value interface{}) {
		args = append(args, value)
		conditions = append(conditions, fmt.Sprintf(format, len(args)))
	}

	if filters.UserID != nil {
		addCondition("t.user_id = $%d", *filters.UserID)
	}
	if filters.AccountID != nil {
		addCondition("t.account_id = $%d", *filters.AccountID)
	}
	if filters.CategoryID != nil {
		addCondition("t.category_id = $%d", *filters.CategoryID)
	}
	if filters.Type != "" {
		addCondition("t.type = $%d", filters.Type)
	}
	if filters.DateFrom != nil {
		addCondition("t.date >= $%d", *filters.DateFrom)
	}
	if filters.DateTo != nil {
		addCondition("t.date <= $%d", *filters.DateTo)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = " WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT t.date, t.type, t.amount, t.description, a.name, c.name
		FROM transactions t
		JOIN accounts a ON a.id = t.account_id
		JOIN categories c ON c.id = t.category_id%s
		ORDER BY t.date ASC, t.created_at ASC`, whereClause)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("query export rows: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var row model.TransactionExportRow
		if err := rows.Scan(&row.Date, &row.Type, &row.Amount, &row.Description, &row.AccountName, &row.CategoryName); err != nil {
			return fmt.Errorf("scan export row: %w", err)
		}
		if err := emit(row); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate export rows: %w", err)
	}

	return nil
}

func (r *TransactionRepo) CreateBatch(ctx context.Context, userID uuid.UUID, transactions []model.Transaction) error {
	if len(transactions) == 0 {
		return nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	accountIDs := make([]uuid.UUID, 0, len(transactions))
	for _, transaction := range transactions {
		accountIDs = append(accountIDs, transaction.AccountID)
	}
	if err := lockAccounts(ctx, tx, userID, accountIDs); err != nil {
		return err
	}

	const insert = `
		INSERT INTO transactions (user_id, account_id, category_id, type, amount, description, date, kind)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	for _, transaction := range transactions {
		if _, err := tx.Exec(ctx, insert,
			userID,
			transaction.AccountID,
			transaction.CategoryID,
			transaction.Type,
			transaction.Amount,
			transaction.Description,
			transaction.Date,
			service.EntryKindNormal,
		); err != nil {
			return fmt.Errorf("insert transaction: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit batch: %w", err)
	}

	return nil
}

func (r *TransactionRepo) Delete(ctx context.Context, userID uuid.UUID, id uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var (
		accountID uuid.UUID
		kind      string
	)
	err = tx.QueryRow(ctx,
		`SELECT account_id, kind FROM transactions WHERE id = $1 AND user_id = $2 FOR UPDATE`,
		id, userID,
	).Scan(&accountID, &kind)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return service.ErrTransactionNotFound
		}
		return fmt.Errorf("select transaction: %w", err)
	}
	if kind != "" && kind != service.EntryKindNormal {
		return service.ErrSystemEntryImmutable
	}

	if err := lockAccounts(ctx, tx, userID, []uuid.UUID{accountID}); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `DELETE FROM transactions WHERE id = $1 AND user_id = $2`, id, userID); err != nil {
		return fmt.Errorf("delete transaction: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func (r *TransactionRepo) MonthlyTotalsByCurrency(ctx context.Context, userID uuid.UUID, month time.Time) ([]model.MonthlyCurrencyTotal, error) {
	query := `
		SELECT a.currency,
		       COALESCE(SUM(CASE WHEN t.type = 'income' THEN t.amount ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN t.type = 'expense' THEN t.amount ELSE 0 END), 0)
		FROM transactions t
		JOIN accounts a ON a.id = t.account_id
		WHERE t.user_id = $1
		  AND t.kind = 'normal'
		  AND t.date >= DATE_TRUNC('month', $2::date)
		  AND t.date < DATE_TRUNC('month', $2::date) + INTERVAL '1 month'
		GROUP BY a.currency
		ORDER BY a.currency
	`

	rows, err := r.pool.Query(ctx, query, userID, month)
	if err != nil {
		return nil, fmt.Errorf("select monthly totals: %w", err)
	}
	defer rows.Close()

	totals := make([]model.MonthlyCurrencyTotal, 0, 4)
	for rows.Next() {
		var total model.MonthlyCurrencyTotal
		if err := rows.Scan(&total.Currency, &total.Income, &total.Expense); err != nil {
			return nil, fmt.Errorf("scan monthly total: %w", err)
		}
		totals = append(totals, total)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate monthly totals: %w", err)
	}

	return totals, nil
}

func lockAccounts(ctx context.Context, tx pgx.Tx, userID uuid.UUID, ids []uuid.UUID) error {
	sorted := append([]uuid.UUID(nil), ids...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].String() < sorted[j].String() })

	for _, id := range sorted {
		if _, err := tx.Exec(ctx, `SELECT 1 FROM accounts WHERE id = $1 AND user_id = $2 FOR UPDATE`, id, userID); err != nil {
			return fmt.Errorf("lock account: %w", err)
		}
	}

	return nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

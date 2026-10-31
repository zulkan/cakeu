package db

import (
	"context"
	"errors"
	"fmt"
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

func (r *AccountRepo) Create(ctx context.Context, userID uuid.UUID, name string, balance float64, currency string) (*model.Account, error) {
	query := `
		INSERT INTO accounts (user_id, name, balance, currency)
		VALUES ($1, $2, $3, $4)
		RETURNING id, user_id, name, balance, currency, created_at, updated_at
	`

	var account model.Account
	err := r.pool.QueryRow(ctx, query, userID, name, balance, currency).Scan(
		&account.ID, &account.UserID, &account.Name, &account.Balance, &account.Currency, &account.CreatedAt, &account.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, service.ErrAccountNameExists
		}
		return nil, fmt.Errorf("insert account: %w", err)
	}

	return &account, nil
}

func (r *AccountRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Account, error) {
	query := `SELECT id, user_id, name, balance, currency, created_at, updated_at FROM accounts WHERE id = $1`

	var account model.Account
	err := r.pool.QueryRow(ctx, query, id).Scan(
		&account.ID, &account.UserID, &account.Name, &account.Balance, &account.Currency, &account.CreatedAt, &account.UpdatedAt,
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
		SELECT id, user_id, name, balance, currency, created_at, updated_at
		FROM accounts
		WHERE user_id = $1
		ORDER BY created_at ASC, name ASC
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
			&account.ID, &account.UserID, &account.Name, &account.Balance, &account.Currency, &account.CreatedAt, &account.UpdatedAt,
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

func (r *AccountRepo) Update(ctx context.Context, userID uuid.UUID, id uuid.UUID, name *string, balance *float64, currency *string) (*model.Account, error) {
	query := `
		UPDATE accounts
		SET name = COALESCE($3::varchar, name),
		    balance = COALESCE($4::numeric, balance),
		    currency = COALESCE($5::varchar, currency),
		    updated_at = NOW()
		WHERE id = $1 AND user_id = $2
		RETURNING id, user_id, name, balance, currency, created_at, updated_at
	`

	var account model.Account
	err := r.pool.QueryRow(ctx, query, id, userID, name, balance, currency).Scan(
		&account.ID, &account.UserID, &account.Name, &account.Balance, &account.Currency, &account.CreatedAt, &account.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, service.ErrAccountNotFound
		}
		if isUniqueViolation(err) {
			return nil, service.ErrAccountNameExists
		}
		return nil, fmt.Errorf("update account: %w", err)
	}

	return &account, nil
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

	query := `
		INSERT INTO transactions (user_id, account_id, category_id, type, amount, description, date)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, user_id, account_id, category_id, type, amount, description, date, created_at
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
	)
	if err != nil {
		return nil, fmt.Errorf("insert transaction: %w", err)
	}

	if err := tx.QueryRow(ctx, `SELECT name FROM accounts WHERE id = $1`, transaction.AccountID).Scan(&created.AccountName); err != nil {
		return nil, fmt.Errorf("select account name: %w", err)
	}

	if err := tx.QueryRow(ctx, `SELECT name FROM categories WHERE id = $1`, transaction.CategoryID).Scan(&created.CategoryName); err != nil {
		return nil, fmt.Errorf("select category name: %w", err)
	}

	delta := transaction.Amount
	if transaction.Type == service.TypeExpense {
		delta = -delta
	}

	if _, err := tx.Exec(ctx,
		`UPDATE accounts SET balance = balance + $2, updated_at = NOW() WHERE id = $1`,
		transaction.AccountID, delta,
	); err != nil {
		return nil, fmt.Errorf("update account balance: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	return &created, nil
}

func (r *TransactionRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Transaction, error) {
	query := `
		SELECT t.id, t.user_id, t.account_id, t.category_id, t.type, t.amount, t.description, t.date, t.created_at,
			a.name, c.name
		FROM transactions t
		JOIN accounts a ON a.id = t.account_id
		JOIN categories c ON c.id = t.category_id
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
		&transaction.AccountName,
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
		SELECT t.id, t.user_id, t.account_id, t.category_id, t.type, t.amount, t.description, t.date, t.created_at,
			a.name, c.name
		FROM transactions t
		JOIN accounts a ON a.id = t.account_id
		JOIN categories c ON c.id = t.category_id%s
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
			&transaction.AccountName,
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

	const insert = `
		INSERT INTO transactions (user_id, account_id, category_id, type, amount, description, date)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
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
		); err != nil {
			return fmt.Errorf("insert transaction: %w", err)
		}

		delta := transaction.Amount
		if transaction.Type == service.TypeExpense {
			delta = -delta
		}

		if _, err := tx.Exec(ctx,
			`UPDATE accounts SET balance = balance + $2, updated_at = NOW() WHERE id = $1 AND user_id = $3`,
			transaction.AccountID, delta, userID,
		); err != nil {
			return fmt.Errorf("update account balance: %w", err)
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
		accountID       uuid.UUID
		transactionType string
		amount          float64
	)
	err = tx.QueryRow(ctx,
		`SELECT account_id, type, amount FROM transactions WHERE id = $1 AND user_id = $2 FOR UPDATE`,
		id, userID,
	).Scan(&accountID, &transactionType, &amount)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return service.ErrTransactionNotFound
		}
		return fmt.Errorf("select transaction: %w", err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM transactions WHERE id = $1 AND user_id = $2`, id, userID); err != nil {
		return fmt.Errorf("delete transaction: %w", err)
	}

	delta := -amount
	if transactionType == service.TypeExpense {
		delta = amount
	}

	if _, err := tx.Exec(ctx,
		`UPDATE accounts SET balance = balance + $2, updated_at = NOW() WHERE id = $1`,
		accountID, delta,
	); err != nil {
		return fmt.Errorf("restore account balance: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func (r *TransactionRepo) MonthlyTotals(ctx context.Context, userID uuid.UUID, month time.Time) (float64, float64, error) {
	query := `
		SELECT
			COALESCE(SUM(CASE WHEN type = 'income' THEN amount ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN type = 'expense' THEN amount ELSE 0 END), 0)
		FROM transactions
		WHERE user_id = $1
		  AND date >= DATE_TRUNC('month', $2::date)
		  AND date < DATE_TRUNC('month', $2::date) + INTERVAL '1 month'
	`

	var income, expense float64
	if err := r.pool.QueryRow(ctx, query, userID, month).Scan(&income, &expense); err != nil {
		return 0, 0, fmt.Errorf("select monthly totals: %w", err)
	}

	return income, expense, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

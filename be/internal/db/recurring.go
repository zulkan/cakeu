package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"goskeleton/internal/model"
	"goskeleton/internal/service"
)

var _ service.RecurringRepository = (*RecurringRepo)(nil)

const recurringColumns = `id, user_id, account_id, category_id, type, amount, description, frequency, interval, next_run, active, created_at, updated_at`

type RecurringRepo struct {
	pool *pgxpool.Pool
}

func NewRecurringRepo(pool *pgxpool.Pool) *RecurringRepo {
	return &RecurringRepo{pool: pool}
}

func scanRecurring(row pgx.Row) (*model.RecurringRule, error) {
	var rule model.RecurringRule
	err := row.Scan(
		&rule.ID, &rule.UserID, &rule.AccountID, &rule.CategoryID, &rule.Type,
		&rule.Amount, &rule.Description, &rule.Frequency, &rule.Interval,
		&rule.NextRun, &rule.Active, &rule.CreatedAt, &rule.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &rule, nil
}

func (r *RecurringRepo) Create(ctx context.Context, rule model.RecurringRule) (*model.RecurringRule, error) {
	query := `
		INSERT INTO recurring_rules (user_id, account_id, category_id, type, amount, description, frequency, interval, next_run, active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING ` + recurringColumns

	created, err := scanRecurring(r.pool.QueryRow(ctx, query,
		rule.UserID, rule.AccountID, rule.CategoryID, rule.Type, rule.Amount,
		rule.Description, rule.Frequency, rule.Interval, rule.NextRun, rule.Active,
	))
	if err != nil {
		return nil, fmt.Errorf("insert recurring rule: %w", err)
	}

	return created, nil
}

func (r *RecurringRepo) ListByUser(ctx context.Context, userID uuid.UUID) ([]model.RecurringRule, error) {
	query := `SELECT ` + recurringColumns + ` FROM recurring_rules WHERE user_id = $1 ORDER BY next_run, created_at`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("select recurring rules: %w", err)
	}
	defer rows.Close()

	rules := make([]model.RecurringRule, 0, 8)
	for rows.Next() {
		rule, scanErr := scanRecurring(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan recurring rule: %w", scanErr)
		}
		rules = append(rules, *rule)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate recurring rules: %w", err)
	}

	return rules, nil
}

func (r *RecurringRepo) GetByID(ctx context.Context, userID, id uuid.UUID) (*model.RecurringRule, error) {
	query := `SELECT ` + recurringColumns + ` FROM recurring_rules WHERE id = $1 AND user_id = $2`

	rule, err := scanRecurring(r.pool.QueryRow(ctx, query, id, userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, service.ErrRecurringNotFound
		}
		return nil, fmt.Errorf("select recurring rule: %w", err)
	}

	return rule, nil
}

func (r *RecurringRepo) Update(ctx context.Context, userID uuid.UUID, rule model.RecurringRule) (*model.RecurringRule, error) {
	query := `
		UPDATE recurring_rules
		SET amount = $3, description = $4, frequency = $5, interval = $6, next_run = $7, active = $8, updated_at = NOW()
		WHERE id = $1 AND user_id = $2
		RETURNING ` + recurringColumns

	updated, err := scanRecurring(r.pool.QueryRow(ctx, query,
		rule.ID, userID, rule.Amount, rule.Description, rule.Frequency, rule.Interval, rule.NextRun, rule.Active,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, service.ErrRecurringNotFound
		}
		return nil, fmt.Errorf("update recurring rule: %w", err)
	}

	return updated, nil
}

func (r *RecurringRepo) Delete(ctx context.Context, userID, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM recurring_rules WHERE id = $1 AND user_id = $2`, id, userID)
	if err != nil {
		return fmt.Errorf("delete recurring rule: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return service.ErrRecurringNotFound
	}

	return nil
}

func (r *RecurringRepo) ListDue(ctx context.Context, now time.Time, limit int) ([]model.RecurringRule, error) {
	query := `SELECT ` + recurringColumns + ` FROM recurring_rules WHERE active AND next_run <= $1 ORDER BY next_run LIMIT $2`

	rows, err := r.pool.Query(ctx, query, now, limit)
	if err != nil {
		return nil, fmt.Errorf("select due recurring rules: %w", err)
	}
	defer rows.Close()

	rules := make([]model.RecurringRule, 0, 8)
	for rows.Next() {
		rule, scanErr := scanRecurring(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan due rule: %w", scanErr)
		}
		rules = append(rules, *rule)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate due rules: %w", err)
	}

	return rules, nil
}

func (r *RecurringRepo) ClaimRun(ctx context.Context, ruleID uuid.UUID, periodStart time.Time) (bool, error) {
	tag, err := r.pool.Exec(ctx,
		`INSERT INTO recurring_runs (rule_id, period_start) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		ruleID, periodStart,
	)
	if err != nil {
		return false, fmt.Errorf("claim recurring run: %w", err)
	}

	return tag.RowsAffected() == 1, nil
}

func (r *RecurringRepo) AttachRunTransaction(ctx context.Context, ruleID uuid.UUID, periodStart time.Time, transactionID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE recurring_runs SET transaction_id = $3 WHERE rule_id = $1 AND period_start = $2`,
		ruleID, periodStart, transactionID,
	)
	if err != nil {
		return fmt.Errorf("attach recurring transaction: %w", err)
	}

	return nil
}

func (r *RecurringRepo) ReleaseRun(ctx context.Context, ruleID uuid.UUID, periodStart time.Time) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM recurring_runs WHERE rule_id = $1 AND period_start = $2 AND transaction_id IS NULL`,
		ruleID, periodStart,
	)
	if err != nil {
		return fmt.Errorf("release recurring run: %w", err)
	}

	return nil
}

func (r *RecurringRepo) AdvanceNextRun(ctx context.Context, ruleID uuid.UUID, next time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE recurring_rules SET next_run = $2, updated_at = NOW() WHERE id = $1`,
		ruleID, next,
	)
	if err != nil {
		return fmt.Errorf("advance next run: %w", err)
	}

	return nil
}

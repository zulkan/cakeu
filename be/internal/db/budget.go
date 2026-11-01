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

var _ service.BudgetRepository = (*BudgetRepo)(nil)

type BudgetRepo struct {
	pool *pgxpool.Pool
}

func NewBudgetRepo(pool *pgxpool.Pool) *BudgetRepo {
	return &BudgetRepo{pool: pool}
}

func (r *BudgetRepo) Upsert(ctx context.Context, userID, categoryID uuid.UUID, month time.Time, amount float64) (*model.Budget, error) {
	query := `
		INSERT INTO budgets (user_id, category_id, month, amount)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, category_id, month)
		DO UPDATE SET amount = EXCLUDED.amount, updated_at = NOW()
		RETURNING id, user_id, category_id, month, amount, created_at, updated_at
	`

	var budget model.Budget
	err := r.pool.QueryRow(ctx, query, userID, categoryID, month, amount).Scan(
		&budget.ID, &budget.UserID, &budget.CategoryID, &budget.Month,
		&budget.Amount, &budget.CreatedAt, &budget.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("upsert budget: %w", err)
	}

	if err := r.pool.QueryRow(ctx, `SELECT name FROM categories WHERE id = $1 AND user_id = $2`, categoryID, userID).Scan(&budget.CategoryName); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("select category name: %w", err)
		}
	}

	return &budget, nil
}

func (r *BudgetRepo) Delete(ctx context.Context, userID, categoryID uuid.UUID, month time.Time) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM budgets WHERE user_id = $1 AND category_id = $2 AND month = $3`,
		userID, categoryID, month,
	)
	if err != nil {
		return fmt.Errorf("delete budget: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return service.ErrBudgetNotFound
	}

	return nil
}

func (r *BudgetRepo) ListByMonth(ctx context.Context, userID uuid.UUID, month time.Time) ([]model.Budget, error) {
	query := `
		SELECT b.id, b.user_id, b.category_id, c.name, b.month, b.amount, b.created_at, b.updated_at
		FROM budgets b
		JOIN categories c ON c.id = b.category_id
		WHERE b.user_id = $1 AND b.month = $2
		ORDER BY c.name
	`

	rows, err := r.pool.Query(ctx, query, userID, month)
	if err != nil {
		return nil, fmt.Errorf("list budgets: %w", err)
	}
	defer rows.Close()

	budgets := make([]model.Budget, 0, 8)
	for rows.Next() {
		var budget model.Budget
		if err := rows.Scan(
			&budget.ID, &budget.UserID, &budget.CategoryID, &budget.CategoryName,
			&budget.Month, &budget.Amount, &budget.CreatedAt, &budget.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan budget: %w", err)
		}
		budgets = append(budgets, budget)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate budgets: %w", err)
	}

	return budgets, nil
}

func (r *BudgetRepo) SpentByCategory(ctx context.Context, userID uuid.UUID, month time.Time) (map[uuid.UUID]float64, error) {
	query := `
		SELECT category_id, COALESCE(SUM(amount), 0)
		FROM transactions
		WHERE user_id = $1
		  AND type = 'expense'
		  AND date >= $2
		  AND date < ($2::date + INTERVAL '1 month')
		GROUP BY category_id
	`

	rows, err := r.pool.Query(ctx, query, userID, month)
	if err != nil {
		return nil, fmt.Errorf("sum spending by category: %w", err)
	}
	defer rows.Close()

	spent := make(map[uuid.UUID]float64, 8)
	for rows.Next() {
		var categoryID uuid.UUID
		var total float64
		if err := rows.Scan(&categoryID, &total); err != nil {
			return nil, fmt.Errorf("scan spending: %w", err)
		}
		spent[categoryID] = total
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate spending: %w", err)
	}

	return spent, nil
}

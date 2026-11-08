package db

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"goskeleton/internal/model"
	"goskeleton/internal/service"
)

var _ service.ReportRepository = (*ReportRepo)(nil)

type ReportRepo struct {
	pool *pgxpool.Pool
}

func NewReportRepo(pool *pgxpool.Pool) *ReportRepo {
	return &ReportRepo{pool: pool}
}

func (r *ReportRepo) Cashflow(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]model.CashflowPoint, error) {
	query := `
		SELECT a.currency,
		       TO_CHAR(DATE_TRUNC('month', t.date), 'YYYY-MM') AS period,
		       COALESCE(SUM(CASE WHEN t.type = 'income' THEN t.amount ELSE 0 END), 0),
		       COALESCE(SUM(CASE WHEN t.type = 'expense' THEN t.amount ELSE 0 END), 0)
		FROM transactions t
		JOIN accounts a ON a.id = t.account_id
		WHERE t.user_id = $1
		  AND t.kind = 'normal'
		  AND t.date >= $2
		  AND t.date < $3
		GROUP BY a.currency, DATE_TRUNC('month', t.date)
		ORDER BY a.currency, DATE_TRUNC('month', t.date)
	`

	rows, err := r.pool.Query(ctx, query, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("query cashflow: %w", err)
	}
	defer rows.Close()

	points := make([]model.CashflowPoint, 0, 12)
	for rows.Next() {
		var point model.CashflowPoint
		if err := rows.Scan(&point.Currency, &point.Period, &point.Income, &point.Expense); err != nil {
			return nil, fmt.Errorf("scan cashflow: %w", err)
		}
		points = append(points, point)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate cashflow: %w", err)
	}

	return points, nil
}

func (r *ReportRepo) CategoryTotals(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]model.CategoryTotal, error) {
	query := `
		SELECT a.currency, c.id, c.name, t.type, SUM(t.amount)
		FROM transactions t
		JOIN accounts a ON a.id = t.account_id
		JOIN categories c ON c.id = t.category_id
		WHERE t.user_id = $1
		  AND t.kind = 'normal'
		  AND t.date >= $2
		  AND t.date < $3
		GROUP BY a.currency, c.id, c.name, t.type
		ORDER BY a.currency, SUM(t.amount) DESC
	`

	rows, err := r.pool.Query(ctx, query, userID, from, to)
	if err != nil {
		return nil, fmt.Errorf("query category totals: %w", err)
	}
	defer rows.Close()

	totals := make([]model.CategoryTotal, 0, 16)
	for rows.Next() {
		var total model.CategoryTotal
		if err := rows.Scan(&total.Currency, &total.CategoryID, &total.CategoryName, &total.Type, &total.Total); err != nil {
			return nil, fmt.Errorf("scan category total: %w", err)
		}
		totals = append(totals, total)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate category totals: %w", err)
	}

	return totals, nil
}

func (r *ReportRepo) MerchantTotals(ctx context.Context, userID uuid.UUID, from, to time.Time, limit int) ([]model.MerchantTotal, error) {
	query := `
		SELECT currency, description, total
		FROM (
			SELECT a.currency AS currency,
			       t.description AS description,
			       SUM(t.amount) AS total,
			       ROW_NUMBER() OVER (
			           PARTITION BY a.currency
			           ORDER BY SUM(t.amount) DESC, t.description ASC
			       ) AS rn
			FROM transactions t
			JOIN accounts a ON a.id = t.account_id
			WHERE t.user_id = $1
			  AND t.kind = 'normal'
			  AND t.type = 'expense'
			  AND t.date >= $2
			  AND t.date < $3
			  AND t.description <> ''
			GROUP BY a.currency, t.description
		) AS ranked
		WHERE rn <= $4
		ORDER BY currency, total DESC
	`

	rows, err := r.pool.Query(ctx, query, userID, from, to, limit)
	if err != nil {
		return nil, fmt.Errorf("query merchant totals: %w", err)
	}
	defer rows.Close()

	totals := make([]model.MerchantTotal, 0, limit)
	for rows.Next() {
		var total model.MerchantTotal
		if err := rows.Scan(&total.Currency, &total.Description, &total.Total); err != nil {
			return nil, fmt.Errorf("scan merchant total: %w", err)
		}
		totals = append(totals, total)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate merchant totals: %w", err)
	}

	return totals, nil
}

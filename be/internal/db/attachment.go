package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"goskeleton/internal/model"
	"goskeleton/internal/service"
)

var _ service.AttachmentRepository = (*AttachmentRepo)(nil)

// AttachmentRepo stores attachment metadata. Every statement is scoped by
// user_id in SQL, never only by record id.
type AttachmentRepo struct {
	pool *pgxpool.Pool
}

func NewAttachmentRepo(pool *pgxpool.Pool) *AttachmentRepo {
	return &AttachmentRepo{pool: pool}
}

func (r *AttachmentRepo) Create(ctx context.Context, userID uuid.UUID, item model.Attachment) (*model.Attachment, error) {
	query := `
		INSERT INTO transaction_attachments (user_id, transaction_id, filename, content_type, size_bytes, storage_key)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, user_id, transaction_id, filename, content_type, size_bytes, storage_key, created_at
	`

	var created model.Attachment
	err := r.pool.QueryRow(ctx, query,
		userID, item.TransactionID, item.Filename, item.ContentType, item.SizeBytes, item.StorageKey,
	).Scan(
		&created.ID, &created.UserID, &created.TransactionID, &created.Filename,
		&created.ContentType, &created.SizeBytes, &created.StorageKey, &created.CreatedAt,
	)
	if err != nil {
		if isForeignKeyViolation(err) {
			return nil, service.ErrTransactionNotFound
		}
		return nil, fmt.Errorf("insert attachment: %w", err)
	}

	return &created, nil
}

func (r *AttachmentRepo) ListByTransaction(ctx context.Context, userID uuid.UUID, transactionID uuid.UUID) ([]model.Attachment, error) {
	query := `
		SELECT id, user_id, transaction_id, filename, content_type, size_bytes, storage_key, created_at
		FROM transaction_attachments
		WHERE user_id = $1 AND transaction_id = $2
		ORDER BY created_at ASC, id ASC
	`

	rows, err := r.pool.Query(ctx, query, userID, transactionID)
	if err != nil {
		return nil, fmt.Errorf("select attachments: %w", err)
	}
	defer rows.Close()

	items := make([]model.Attachment, 0)
	for rows.Next() {
		var item model.Attachment
		if err := rows.Scan(
			&item.ID, &item.UserID, &item.TransactionID, &item.Filename,
			&item.ContentType, &item.SizeBytes, &item.StorageKey, &item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan attachment: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate attachments: %w", err)
	}

	return items, nil
}

func (r *AttachmentRepo) GetByID(ctx context.Context, userID uuid.UUID, id uuid.UUID) (*model.Attachment, error) {
	query := `
		SELECT id, user_id, transaction_id, filename, content_type, size_bytes, storage_key, created_at
		FROM transaction_attachments
		WHERE id = $1 AND user_id = $2
	`

	var item model.Attachment
	err := r.pool.QueryRow(ctx, query, id, userID).Scan(
		&item.ID, &item.UserID, &item.TransactionID, &item.Filename,
		&item.ContentType, &item.SizeBytes, &item.StorageKey, &item.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, service.ErrAttachmentNotFound
		}
		return nil, fmt.Errorf("select attachment: %w", err)
	}

	return &item, nil
}

func (r *AttachmentRepo) Delete(ctx context.Context, userID uuid.UUID, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM transaction_attachments WHERE id = $1 AND user_id = $2`,
		id, userID,
	)
	if err != nil {
		return fmt.Errorf("delete attachment: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return service.ErrAttachmentNotFound
	}

	return nil
}

func (r *AttachmentRepo) StorageKeysByTransaction(ctx context.Context, userID uuid.UUID, transactionID uuid.UUID) ([]string, error) {
	return r.storageKeys(ctx, `
		SELECT storage_key
		FROM transaction_attachments
		WHERE user_id = $1 AND transaction_id = $2
	`, userID, transactionID)
}

func (r *AttachmentRepo) StorageKeysByAccount(ctx context.Context, userID uuid.UUID, accountID uuid.UUID) ([]string, error) {
	return r.storageKeys(ctx, `
		SELECT ta.storage_key
		FROM transaction_attachments ta
		JOIN transactions t ON t.id = ta.transaction_id
		WHERE ta.user_id = $1 AND t.account_id = $2
	`, userID, accountID)
}

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}

func (r *AttachmentRepo) storageKeys(ctx context.Context, query string, userID uuid.UUID, ownerID uuid.UUID) ([]string, error) {
	rows, err := r.pool.Query(ctx, query, userID, ownerID)
	if err != nil {
		return nil, fmt.Errorf("select attachment storage keys: %w", err)
	}
	defer rows.Close()

	keys := make([]string, 0)
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, fmt.Errorf("scan attachment storage key: %w", err)
		}
		keys = append(keys, key)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate attachment storage keys: %w", err)
	}

	return keys, nil
}

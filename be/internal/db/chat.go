package db

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"goskeleton/internal/model"
	"goskeleton/internal/service"
)

var _ service.ChatHistoryRepository = (*ChatHistoryRepo)(nil)

type ChatHistoryRepo struct {
	pool *pgxpool.Pool
}

func NewChatHistoryRepo(pool *pgxpool.Pool) *ChatHistoryRepo {
	return &ChatHistoryRepo{pool: pool}
}

func (r *ChatHistoryRepo) ListMessages(ctx context.Context, userID uuid.UUID) ([]model.ChatHistoryMessage, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, role, content, created_at FROM chat_messages WHERE user_id = $1 ORDER BY seq`,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("select chat messages: %w", err)
	}
	defer rows.Close()

	messages := make([]model.ChatHistoryMessage, 0, 16)
	for rows.Next() {
		var message model.ChatHistoryMessage
		if err := rows.Scan(&message.ID, &message.Role, &message.Content, &message.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan chat message: %w", err)
		}
		messages = append(messages, message)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chat messages: %w", err)
	}

	return messages, nil
}

func (r *ChatHistoryRepo) TokenTotal(ctx context.Context, userID uuid.UUID) (int, error) {
	var total int
	if err := r.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(token_count), 0) FROM chat_messages WHERE user_id = $1`,
		userID,
	).Scan(&total); err != nil {
		return 0, fmt.Errorf("sum chat tokens: %w", err)
	}

	return total, nil
}

func (r *ChatHistoryRepo) AppendMessages(ctx context.Context, userID uuid.UUID, messages []model.NewChatMessage) error {
	if len(messages) == 0 {
		return nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin chat messages: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	for _, message := range messages {
		if _, err := tx.Exec(ctx,
			`INSERT INTO chat_messages (user_id, role, content, token_count) VALUES ($1, $2, $3, $4)`,
			userID, message.Role, message.Content, message.TokenCount,
		); err != nil {
			return fmt.Errorf("insert chat message: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit chat messages: %w", err)
	}

	return nil
}

func (r *ChatHistoryRepo) Clear(ctx context.Context, userID uuid.UUID) error {
	if _, err := r.pool.Exec(ctx,
		`DELETE FROM chat_messages WHERE user_id = $1`,
		userID,
	); err != nil {
		return fmt.Errorf("clear chat messages: %w", err)
	}

	return nil
}

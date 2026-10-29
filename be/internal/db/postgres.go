package db

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

type pool interface {
	Ping(context.Context) error
	Close()
}

type Client struct {
	pool pool
}

func New(ctx context.Context, databaseURL string) (*Client, error) {
	if databaseURL == "" {
		return nil, errors.New("database url is required")
	}

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}

	return &Client{pool: pool}, nil
}

func NewFromPool(p pool) *Client {
	return &Client{pool: p}
}

func (c *Client) Ping(ctx context.Context) error {
	if c == nil || c.pool == nil {
		return errors.New("database pool is not initialized")
	}

	return c.pool.Ping(ctx)
}

func (c *Client) Pool() *pgxpool.Pool {
	if p, ok := c.pool.(*pgxpool.Pool); ok {
		return p
	}
	return nil
}

func (c *Client) Close() {
	if c == nil || c.pool == nil {
		return
	}

	c.pool.Close()
}

package db_test

import (
	"context"
	"errors"
	"testing"

	"goskeleton/internal/db"
)

type fakePool struct {
	err    error
	closed bool
}

func (f *fakePool) Ping(context.Context) error {
	return f.err
}

func (f *fakePool) Close() {
	f.closed = true
}

func TestClientPingDelegatesToPool(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("database unavailable")
	client := db.NewFromPool(&fakePool{err: expectedErr})

	err := client.Ping(context.Background())
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected delegated ping error %v, got %v", expectedErr, err)
	}
}

func TestClientCloseClosesPool(t *testing.T) {
	t.Parallel()

	pool := &fakePool{}
	client := db.NewFromPool(pool)

	client.Close()

	if !pool.closed {
		t.Fatal("expected client close to close underlying pool")
	}
}

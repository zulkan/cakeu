// Package attachment stores attachment bytes outside Postgres.
package attachment

import (
	"errors"
	"io"
)

// ErrInvalidKey is returned when a storage key could point outside the store.
var ErrInvalidKey = errors.New("invalid storage key")

// Store persists attachment bytes under an opaque key. The metadata lives in
// Postgres; only the bytes live here.
type Store interface {
	Save(key string, r io.Reader) (int64, error)
	Open(key string) (io.ReadCloser, error)
	Remove(key string) error
}

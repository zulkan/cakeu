package attachment

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// LocalStore keeps attachment bytes as files in a single directory.
type LocalStore struct {
	dir string
}

// NewLocalStore creates dir when it does not exist. Startup fails loudly here
// so the first upload does not discover a missing directory.
func NewLocalStore(dir string) (*LocalStore, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, fmt.Errorf("attachments directory is required")
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("create attachments directory %q: %w", dir, err)
	}

	return &LocalStore{dir: dir}, nil
}

// Save writes the reader to <dir>/<key> and returns the number of bytes
// written. Keys are generated UUIDs; user input never reaches this path.
func (s *LocalStore) Save(key string, r io.Reader) (int64, error) {
	path, err := s.path(key)
	if err != nil {
		return 0, err
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o640)
	if err != nil {
		return 0, fmt.Errorf("create attachment file: %w", err)
	}

	written, copyErr := io.Copy(file, r)
	closeErr := file.Close()
	if copyErr != nil {
		_ = os.Remove(path)
		return written, fmt.Errorf("write attachment file: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(path)
		return written, fmt.Errorf("close attachment file: %w", closeErr)
	}

	return written, nil
}

func (s *LocalStore) Open(key string) (io.ReadCloser, error) {
	path, err := s.path(key)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open attachment file: %w", err)
	}

	return file, nil
}

// Remove deletes the file. A missing file already satisfies the contract, so
// cleanup after a failed earlier removal stays quiet.
func (s *LocalStore) Remove(key string) error {
	path, err := s.path(key)
	if err != nil {
		return err
	}

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove attachment file: %w", err)
	}

	return nil
}

func (s *LocalStore) path(key string) (string, error) {
	if key == "" || key == "." || strings.ContainsAny(key, `/\`) || strings.Contains(key, "..") {
		return "", fmt.Errorf("%w: %q", ErrInvalidKey, key)
	}

	return filepath.Join(s.dir, key), nil
}

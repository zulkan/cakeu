package logging

import (
	"log/slog"
	"os"
	"path/filepath"
)

func New(dir string) (*slog.Logger, func() error, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, nil, err
	}

	filePath := filepath.Join(dir, "app.log")
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, nil, err
	}

	options := &slog.HandlerOptions{Level: slog.LevelInfo, AddSource: true}
	handler := slog.NewMultiHandler(
		slog.NewTextHandler(os.Stderr, options),
		slog.NewJSONHandler(file, options),
	)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	return logger, file.Close, nil
}

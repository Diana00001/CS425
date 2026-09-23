package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

func Open(path string) (*slog.Logger, io.Closer, error) {
	dir := filepath.Dir(path)

	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, nil, fmt.Errorf("create log directory %q: %w", dir, err)
	}

	file, err := os.OpenFile(
		path,
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0644,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("open log file %q: %w", path, err)
	}

	writer := io.MultiWriter(os.Stderr, file)
	log := slog.New(slog.NewTextHandler(writer, nil))

	return log, file, nil
}

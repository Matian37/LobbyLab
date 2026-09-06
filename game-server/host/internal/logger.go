package internal

import (
	"io"
	"log/slog"
)

// NewLogger builds a JSON slog.Logger writing to writer, tagging every record
// with the worker ID from cfg.
func NewLogger(writer io.Writer, cfg Config) *slog.Logger {
	return slog.New(
		slog.NewJSONHandler(
			writer,
			&slog.HandlerOptions{Level: cfg.LogLevel},
		).WithAttrs(
			[]slog.Attr{slog.String("workerID", cfg.WorkerID)},
		),
	)
}

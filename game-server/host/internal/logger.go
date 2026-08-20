package internal

import (
	"io"
	"log/slog"
)

func NewLogger(writer io.Writer, cfg Config) *slog.Logger {
	return slog.New(
		slog.NewJSONHandler(
			writer,
			&slog.HandlerOptions{Level: cfg.LogLevel},
		).WithAttrs(
			[]slog.Attr{slog.String("hostname", cfg.Hostname)},
		),
	)
}

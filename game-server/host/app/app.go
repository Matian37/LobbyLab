package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"server/internal"
)

func Run(ctx context.Context, cfg internal.Config) error {
	setupLogger(cfg)
	slog.Info("starting...")

	return run(ctx, NewServer(cfg.BrokerURI, cfg.Hostname, cfg.GameServerArgs))
}

func run(ctx context.Context, server *Server) error {
	if err := server.Open(); err != nil {
		return fmt.Errorf("failed to open server: %w", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			slog.Error("failed to close server", "error", err)
		}
	}()

	slog.Info("successfully opened server")

	err := server.Run(ctx)
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

func setupLogger(cfg internal.Config) {
	setupLoggerWithWriter(os.Stdout, cfg)
}

func setupLoggerWithWriter(writer io.Writer, cfg internal.Config) {
	logger := slog.New(
		slog.NewJSONHandler(
			writer,
			&slog.HandlerOptions{Level: cfg.LogLevel},
		).WithAttrs([]slog.Attr{slog.String("hostname", cfg.Hostname)}),
	)
	slog.SetDefault(logger)
}

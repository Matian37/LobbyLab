package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"server/internal"
)

func Run(ctx context.Context, cfg internal.Config, logger *slog.Logger) error {
	serverLogger := logger.With("component", "server")
	serverLogger.Info("starting...")

	return run(ctx, NewServer(cfg.BrokerURI, cfg.Hostname, cfg.GameServerArgs, logger), serverLogger)
}

func run(ctx context.Context, server *Server, logger *slog.Logger) error {
	logger.Debug("opening server...")

	if err := server.Open(); err != nil {
		return fmt.Errorf("failed to open server: %w", err)
	}
	defer func() {
		if err := server.Close(); err != nil {
			logger.Error("failed to close server", "error", err)
		}
	}()

	logger.Debug("server opened; starting server loop...")

	err := server.Run(ctx)
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

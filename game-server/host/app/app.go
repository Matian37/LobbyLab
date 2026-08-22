package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"server/config"
	"server/internal"
)

func Run(ctx context.Context) error {
	cfg, err := config.ReadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	logger := internal.NewLogger(os.Stdout, cfg)

	serverLogger := logger.With("component", "server")
	serverLogger.Info("starting...")

	server := NewServer(cfg.BrokerURI, cfg.WorkerID, cfg.GameServerArgs, logger)
	return run(ctx, server, serverLogger)
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

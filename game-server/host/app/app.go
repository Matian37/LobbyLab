// Package app orchestrates the life cycle of the game-server application.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/Matian37/LobbyLab/game-server/config"
	"github.com/Matian37/LobbyLab/game-server/internal"
)

// Run runs the full application with configuration, logger, and server.
func Run(ctx context.Context) error {
	cfg, err := config.ReadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	logger := internal.NewLogger(os.Stdout, cfg)

	serverLogger := logger.With("component", "server")
	serverLogger.Info("starting...")

	server := NewServer(cfg.BrokerURI, cfg.WorkerID, cfg.GameServerArgs, logger)
	return runServer(ctx, server, serverLogger)
}

func runServer(ctx context.Context, server *Server, logger *slog.Logger) error {
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

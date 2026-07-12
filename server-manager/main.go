package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"server-manager/app"
	"server-manager/config"
	"server-manager/internal"
	"syscall"
)

func main() {
	baseHandler := slog.NewJSONHandler(
		os.Stdout,
		&slog.HandlerOptions{Level: slog.LevelInfo},
	)
	handler := internal.NewContextErrorHandler(baseHandler)
	logger := slog.New(handler)

	config, err := config.ReadConfig()
	if err != nil {
		logger.Error("failed to read config", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx, config, logger); err != nil {
		logger.Error("application failed", "error", err)
		os.Exit(1)
	}
}

package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"server-manager/app"
	"server-manager/config"
	"syscall"
)

func main() {
	config, err := config.ReadConfig()
	if err != nil {
		slog.Error("failed to read config", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// FIX: don't log context errors
	if err := app.Run(ctx, config); err != nil {
		slog.Error("application failed", "error", err)
		os.Exit(1)
	}
}

package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"server/app"
	"server/config"
	"syscall"
)

func main() {
	cfg, err := config.ReadConfig()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	if err := app.Run(ctx, cfg); err != nil {
		slog.Error("application failed", "error", err)
		os.Exit(1)
	}
}

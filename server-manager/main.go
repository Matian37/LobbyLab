// Package main is the entry point of the server-manager service. It reads the
// configuration, wires logging, and runs the application until it receives a
// termination signal.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Matian37/multiplayer-asset/server-manager/app"
	"github.com/Matian37/multiplayer-asset/server-manager/config"
	"github.com/Matian37/multiplayer-asset/server-manager/internal"
)

func main() {
	config, err := config.ReadConfig()
	if err != nil {
		panic(fmt.Errorf("failed to read config: %w", err))
	}

	baseHandler := slog.NewJSONHandler(
		os.Stdout,
		&slog.HandlerOptions{Level: config.LogLevel},
	)
	handler := internal.NewContextErrorHandler(baseHandler)
	logger := slog.New(handler)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := app.Run(ctx, config, logger); err != nil {
		logger.Error("application failed", "error", err)
		os.Exit(1)
	}
}

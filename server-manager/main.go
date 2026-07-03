package main

import (
	"context"
	"fmt"
	"log/slog"
	"os/signal"
	"server-manager/app"
	"server-manager/config"
	"syscall"
)

func main() {
	if err := run(); err != nil {
		slog.Error("application failed", "error", err)
	}
}

func run() error {
	config, err := config.ReadConfig()
	if err != nil {
		return fmt.Errorf("failed to read config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	wm := app.NewWorkerManager(config)
	defer func() { _ = wm.Shutdown() }()
	if err := wm.Start(ctx); err != nil {
		return err
	}

	<-ctx.Done()

	return nil
}

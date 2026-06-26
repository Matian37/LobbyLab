package main

import (
	"context"
	"fmt"
	"log/slog"
	"os/signal"
	"syscall"
)

func main() {
	if err := run(); err != nil {
		slog.Error("application failed", "error", err)
	}
}

func run() error {
	config, err := ReadConfig()
	if err != nil {
		return fmt.Errorf("failed to read config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	wm := NewWorkerManager(config)

	if err = wm.Init(ctx, config); err != nil {
		wm.Close()
		return err
	}

	wm.Run(ctx)
	wm.Close()

	return nil
}

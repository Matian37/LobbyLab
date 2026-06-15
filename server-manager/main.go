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

	wm := NewWorkerManager()

	if err = wm.Init(ctx, config, config.Workercount); err != nil {
		return err
	}
	defer wm.Close(ctx)

	go wm.HealthLoop(ctx)
	go wm.SaveLoop(ctx)

	<-ctx.Done()

	return nil
}

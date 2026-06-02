package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/shlex"
)

func ParseArgs(args []string) ([]string, error) {
	if len(args) != 2 {
		return []string{}, fmt.Errorf("Expected 1 argument, got %v", len(args)-1)
	}

	cmdArgs, err := shlex.Split(args[1])
	if err != nil {
		return []string{}, fmt.Errorf("failed to parse game server command: %w", err)
	}
	return cmdArgs, nil
}

func main() {
	if err := run(); err != nil {
		slog.Error("application failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	slog.Info("starting...")

	cmdArgs, err := ParseArgs(os.Args)
	if err != nil {
		return fmt.Errorf("%w, \nUsage: %v \"COMMAND\"", err, os.Args[0])
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	containerID, err := os.Hostname()
	if err != nil {
		return fmt.Errorf("failed to get hostname: %w", err)
	}

	// fix incorrect naming of variable
	natsURI := os.Getenv("NATS_URI")
	if natsURI == "" {
		natsURI = "nats://localhost:4222"
	}

	app := NewApp(natsURI, containerID, cmdArgs)

	if err := app.Init(1 * time.Second); err != nil {
		return fmt.Errorf("failed to init app: %w", err)
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- app.Run(ctx)
	}()

	signalCtx, stop := signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	select {
	case err := <-errChan:
		return err
	case <-signalCtx.Done():
		slog.Info("stopping...")
		cancel()
		return nil
	}
}

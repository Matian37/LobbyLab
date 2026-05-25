package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

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
	slog.Info("starting...")

	cmdArgs, err := ParseArgs(os.Args)
	if err != nil {
		wrapErr := fmt.Errorf("%w, \nUsage: %v \"COMMAND\"", err, os.Args[0])
		panic(wrapErr)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	brokerUri := GenBrokerUri(
		os.Getenv("RABBITMQ_DEFAULT_USER"),
		os.Getenv("RABBITMQ_DEFAULT_PASS"),
	)
	app := NewApp(brokerUri, cmdArgs)

	if err := app.Init(ctx); err != nil {
		panic(fmt.Errorf("failed to init app: %w", err))
	}
	go app.Run(ctx)

	signalCtx, stop := signal.NotifyContext(
		ctx, syscall.SIGTERM, syscall.SIGINT,
	)
	defer stop()
	<-signalCtx.Done()

	slog.Info("stopping...")
}

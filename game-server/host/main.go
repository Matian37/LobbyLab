package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/google/shlex"
)

func main() {
	slog.Info("starting...")

	if len(os.Args) != 2 {
		panic(fmt.Errorf(
			"Expected 1 argument, got %v\nUsage: %v \"game server run command\"",
			len(os.Args)-1,
			os.Args[0],
		))
	}

	cmdArgs, err := shlex.Split(os.Args[1])
	if err != nil {
		panic(fmt.Errorf("failed to parse game server command: %w", err))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	brokerUri := GenBrokerUri(
		os.Getenv("RABBITMQ_DEFAULT_USER"),
		os.Getenv("RABBITMQ_DEFAULT_PASS"),
	)
	app, err := NewApp(ctx, brokerUri)
	if err != nil {
		panic(fmt.Errorf("failed to init app: %w", err))
	}

	go app.Run(ctx, cmdArgs)
	app.HandleSignals(ctx)

	slog.Info("stopping...")
}

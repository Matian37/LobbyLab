package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"server/app"
	"syscall"

	"github.com/google/shlex"
)

func setupLogger() {
	setupLoggerWithWriter(os.Stdout)
}

func setupLoggerWithWriter(writer io.Writer) {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	var level slog.Level
	if err := level.UnmarshalText([]byte(os.Getenv("LOG_LEVEL"))); err != nil {
		level = slog.LevelInfo
	}

	logger := slog.New(
		slog.NewJSONHandler(
			writer,
			&slog.HandlerOptions{Level: level},
		).WithAttrs([]slog.Attr{slog.String("hostname", hostname)}),
	)
	slog.SetDefault(logger)
}

func ParseArgs(args []string) ([]string, error) {
	if len(args) != 2 {
		return []string{}, fmt.Errorf("expected 1 argument, got %v", len(args)-1)
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
	setupLogger()
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

	natsURI, ok := os.LookupEnv("NATS_URI")
	if !ok {
		return errors.New("failed to find NATS_URI env")
	}

	app := app.NewServer(natsURI, containerID, cmdArgs)

	if err := app.Open(); err != nil {
		return fmt.Errorf("failed to open app: %w", err)
	}
	slog.Info("successfully opened app")
	defer func() {
		if err := app.Close(); err != nil {
			slog.Error("failed to close app", "error", err)
		}
	}()

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
		<-errChan
		return nil
	}
}

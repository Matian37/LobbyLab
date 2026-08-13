package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/google/shlex"
)

func setupLogger() {
	setupLoggerWithWriter(os.Stdout)
}

func getID() (string, error) {
	id, ok := os.LookupEnv("WORKER_ID")
	if !ok {
		return "", fmt.Errorf("WORKER_ID not set")
	}
	return id, nil
}

func setupLoggerWithWriter(writer io.Writer) {
	workerID, err := getID()
	if err != nil {
		workerID = "unknown"
	}

	var level slog.Level
	if err := level.UnmarshalText([]byte(os.Getenv("LOG_LEVEL"))); err != nil {
		level = slog.LevelInfo
	}

	logger := slog.New(
		slog.NewJSONHandler(
			writer,
			&slog.HandlerOptions{Level: level},
		).WithAttrs([]slog.Attr{slog.String("workerID", workerID)}),
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

	workerID, err := getID()
	if err != nil {
		return err
	}

	natsURI, ok := os.LookupEnv("NATS_URI")
	if !ok {
		return errors.New("failed to find NATS_URI env")
	}

	app := NewApp(natsURI, workerID, cmdArgs)

	if err := app.Init(); err != nil {
		return fmt.Errorf("failed to init app: %w", err)
	}
	slog.Info("successfuly initialized app")

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

package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"server/internal/domain"
	"time"
)

var (
	ErrAppNotInitialized     = errors.New("app not initialized")
	ErrAppAlreadyInitialized = errors.New("app already initialized")
)

type App struct {
	conn        domain.BrokerConnection
	server      domain.Server
	cmdArgs     []string
	initialized bool

	initTimeout       time.Duration
	serverStopTimeout time.Duration
	sendResultTimeout time.Duration
	sendCancelTimeout time.Duration
}

func NewApp(brokerURI string, containerID string, cmdArgs []string) *App {
	return &App{
		conn:              NewConnection(brokerURI, containerID),
		server:            &GameServer{},
		cmdArgs:           cmdArgs,
		initTimeout:       5 * time.Second,
		serverStopTimeout: 5 * time.Second,
		sendResultTimeout: 15 * time.Second,
		sendCancelTimeout: 5 * time.Second,
	}
}

func (app *App) Init() error {
	if app.initialized {
		return ErrAppAlreadyInitialized
	}

	if err := app.conn.Open(app.initTimeout); err != nil {
		return err
	}
	app.initialized = true
	return nil
}

func (app *App) Run(ctx context.Context) error {
	if !app.initialized {
		return ErrAppNotInitialized
	}

	slog.Info("app loop started", "args", app.cmdArgs)

	for ctx.Err() == nil {
		if err := app.runMatch(ctx); err != nil {
			if errors.Is(err, context.Canceled) {
				break
			}
			slog.Error("match execution failed", "error", err)
		}
		slog.Info("match lifecycle complete, ready for next request")
	}
	return ctx.Err()
}

func (app *App) runMatch(ctx context.Context) error {
	config, err := app.conn.GetMatchConfig(ctx)
	if err != nil {
		return fmt.Errorf("get match config failed: %w", err)
	}
	slog.Info("received game server start request", "config_len", len(config))

	result, err := app.runServer(ctx, config)
	if err != nil {
		app.sendCancel()
		return fmt.Errorf("server execution failed: %w", err)
	}

	slog.Info("match result retrieved, sending...")
	if err := app.sendResult(ctx, result); err != nil {
		app.sendCancel()
		return fmt.Errorf("failed to send match result: %w", err)
	}
	return nil
}

func (app *App) runServer(ctx context.Context, config string) ([]byte, error) {
	slog.Info("starting server")
	if err := app.server.Start(config, app.cmdArgs); err != nil {
		return nil, fmt.Errorf("failed to start server: %w", err)
	}
	defer app.stopServer(ctx)

	slog.Info("server running, waiting for result")
	result, err := app.server.GetResult(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve match result: %w", err)
	}
	return result, nil
}

func (app *App) stopServer(ctx context.Context) {
	timeoutCtx, cancel := context.WithTimeout(ctx, app.serverStopTimeout)
	defer cancel()

	err := app.server.Stop(timeoutCtx)

	// Stop errors are only logged since failure to stop does not affect server reuse.
	if err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("failed to stop server", "error", err)
	}
}

func (app *App) sendCancel() {
	ctx, cancel := context.WithTimeout(context.Background(), app.sendCancelTimeout)
	defer cancel()

	if err := app.conn.SendCancel(ctx); err != nil {
		slog.Error("send match cancel failed", "error", err)
	}
}

func (app *App) sendResult(ctx context.Context, result []byte) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, app.sendResultTimeout)
	defer cancel()
	return app.conn.SendResult(timeoutCtx, result)
}

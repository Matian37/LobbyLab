package main

import (
	"context"
	"errors"
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
}

func NewApp(brokerUri string, containerId string, cmdArgs []string) *App {
	return &App{
		conn:    NewConnection(brokerUri, containerId),
		server:  &GameServer{},
		cmdArgs: cmdArgs,
	}
}

func (app *App) Init(timeout time.Duration) error {
	if app.initialized {
		return ErrAppAlreadyInitialized
	}

	if err := app.conn.Open(timeout); err != nil {
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

	// TODO: prevent logging context errors like Canceled, DeadlineExceeded...
	// TODO: handle SendCancel errors and server.Stop?
	// TODO: add timeouts
	for ctx.Err() == nil {
		config, err := app.conn.GetMatchConfig(ctx)
		if err != nil {
			slog.Error("get match config failed", "error", err)
			continue
		}
		slog.Info("received game server start request", "config_len", len(config))

		slog.Info("starting game server")
		if err := app.server.Start(config, app.cmdArgs); err != nil {
			slog.Error("failed to start game server", "error", err)
			_ = app.conn.SendCancel(context.Background())
			continue
		}

		slog.Info("game server running, waiting for result")
		result, err := app.server.GetResult(ctx)
		if err != nil {
			slog.Error("failed to retrieve match result", "error", err)
			_ = app.server.Stop(context.Background())
			_ = app.conn.SendCancel(context.Background())
			continue
		}
		slog.Info("match result retrieved, sending...")

		if err := app.conn.SendResult(ctx, result); err != nil {
			slog.Error("failed to send match result", "error", err)
			_ = app.conn.SendCancel(context.Background())
			continue
		}
		slog.Info("match lifecycle complete, ready for next request")
	}

	return ctx.Err()
}

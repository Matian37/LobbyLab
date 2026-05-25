package main

import (
	"context"
	"errors"
	"log/slog"
	"server/internal/domain"
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

func NewApp(brokerUri string, cmdArgs []string) *App {
	return &App{
		conn:    NewConnection(brokerUri),
		server:  &GameServer{},
		cmdArgs: cmdArgs,
	}
}

func (app *App) Init(ctx context.Context) error {
	if app.initialized {
		return ErrAppAlreadyInitialized
	}

	if err := app.conn.Connect(ctx); err != nil {
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

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		payload, err := app.conn.GetStartRequest(ctx)
		if err != nil {
			return err
		}
		slog.Info("received game server start request", "payload_len", len(payload))
		slog.Debug("request detail", "payload", string(payload))

		slog.Info("starting game server")
		if err = app.server.Start(string(payload), app.cmdArgs); err != nil {
			slog.Error("failed to start game server", "error", err, "action", "skipping_request")
			app.server.Stop(ctx)
			continue
		}

		slog.Info("game server running, waiting for result")
		result, err := app.server.GetResult(ctx)
		if err != nil {
			slog.Error("failed to retrieve match result", "error", err, "action", "stopping_server")
			app.server.Stop(ctx)
			continue
		}
		_ = app.server.Stop(ctx)

		slog.Info("match finished, sending result", "result_len", len(result))
		slog.Debug("result detail", "result", string(result))
		if err = app.conn.SendMatchResult(ctx, result); err != nil {
			slog.Error("failed to send match result back to broker", "error", err)
			continue
		}
		slog.Info("match lifecycle complete, ready for next request")
	}
}

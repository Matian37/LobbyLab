package main

import (
	"context"
	"errors"
	"log/slog"
	"os/signal"
	"server/internal/domain"
	"syscall"
)

type App struct {
	conn        domain.Connection
	server      domain.Server
	initialized bool
}

func NewApp(ctx context.Context, brokerUri string) (*App, error) {
	app := &App{conn: NewConnection(brokerUri), server: &GameServer{}}

	if err := app.conn.Connect(ctx); err != nil {
		return app, err
	}
	app.initialized = true

	return app, nil
}

func (app *App) Run(ctx context.Context, cmdArgs []string) error {
	if !app.initialized {
		return errors.New("app not initialized")
	}

	for {
		payload, err := app.conn.GetStartRequest(ctx)
		if err != nil {
			return err
		}

		if err = app.server.Start(string(payload), cmdArgs); err != nil {
			slog.Error("server start failed", "error", err)
			app.server.Stop(ctx)
			continue
		}

		result, err := app.server.GetResult(ctx)
		if err != nil {
			slog.Error("waiting for match result failed", "error", err)
			app.server.Stop(ctx)
			continue
		}

		app.server.Stop(ctx)

		if err = app.conn.SendMatchResult(ctx, result); err != nil {
			slog.Error("sending match result failed", "error", err)
			continue
		}
	}
}

func (app *App) HandleSignals(ctx context.Context) {
	signalCtx, stop := signal.NotifyContext(
		ctx, syscall.SIGTERM, syscall.SIGINT,
	)
	defer stop()
	<-signalCtx.Done()
}

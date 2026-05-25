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

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		payload, err := app.conn.GetStartRequest(ctx)
		if err != nil {
			return err
		}

		if err = app.server.Start(string(payload), app.cmdArgs); err != nil {
			slog.Error("server start failed", "error", err)
			app.server.Stop(ctx)
			continue
		}
		// TODO: make delivery.Accept() here with some interaface

		result, err := app.server.GetResult(ctx)
		if err != nil {
			slog.Error("waiting for match result failed", "error", err)
			app.server.Stop(ctx)
			continue
		}
		_ = app.server.Stop(ctx)

		if err = app.conn.SendMatchResult(ctx, result); err != nil {
			slog.Error("sending match result failed", "error", err)
			continue
		}
	}
}

package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"server/adapters"
	"server/internal"
	"time"
)

var (
	ErrAppNotInitialized     = errors.New("app not initialized")
	ErrAppAlreadyInitialized = errors.New("app already initialized")
)

type App struct {
	conn        internal.BrokerConnection
	server      internal.Executor
	cmdArgs     []string
	initialized bool

	initTimeout       time.Duration
	serverStopTimeout time.Duration
	sendResultTimeout time.Duration
	sendCancelTimeout time.Duration
}

func NewApp(brokerURI string, containerID string, cmdArgs []string) *App {
	return &App{
		conn:              adapters.NewConnection(brokerURI, containerID),
		server:            adapters.NewExecutor(cmdArgs),
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

	slog.Info("app loop started; ready for requests", "gameServerArgs", app.cmdArgs)

	for ctx.Err() == nil {
		if err := app.runMatch(ctx); err != nil {
			if errors.Is(err, context.Canceled) {
				break
			}
			slog.Error("match execution failed", "error", err)
		} else {
			slog.Info("match execution successful")
		}
	}
	return ctx.Err()
}

func (app *App) runMatch(ctx context.Context) error {
	slog.Debug("waiting for match config...")
	config, err := app.conn.GetMatchConfig(ctx)
	if err != nil {
		return fmt.Errorf("get match config failed: %w", err)
	}
	slog.Info("received match config", "matchID", config.MatchID, "configLen", len(config.Config))
	slog.Debug("match config", "config", config)

	result, err := app.runServer(ctx, string(config.Config))
	if err != nil {
		app.sendCancel(config.MatchID)
		return fmt.Errorf("server execution failed: %w", err)
	}

	slog.Info("match result retrieved; sending to broker", "resultLen", len(result))
	if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		slog.Debug("match result", "result", string(result))
	}

	if err := app.sendResult(ctx, config.MatchID, result); err != nil {
		app.sendCancel(config.MatchID)
		return fmt.Errorf("failed to send result: %w", err)
	}
	return nil
}

func (app *App) runServer(ctx context.Context, config string) ([]byte, error) {
	slog.Info("starting server...")
	if err := app.server.Start(config); err != nil {
		return nil, fmt.Errorf("failed to start server: %w", err)
	}
	defer app.stopServer(ctx)

	slog.Info("server started; waiting for the result...")
	result, err := app.server.GetResult(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get match result: %w", err)
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

func (app *App) sendCancel(matchID int) {
	slog.Warn("sending match cancel...")

	ctx, cancel := context.WithTimeout(context.Background(), app.sendCancelTimeout)
	defer cancel()

	if err := app.conn.SendCancel(ctx, matchID); err != nil {
		slog.Error("send match cancel failed", "error", err)
	}
}

func (app *App) sendResult(ctx context.Context, matchID int, result []byte) error {
	slog.Info("sending match result...")

	timeoutCtx, cancel := context.WithTimeout(ctx, app.sendResultTimeout)
	defer cancel()
	return app.conn.SendResult(timeoutCtx, matchID, result)
}

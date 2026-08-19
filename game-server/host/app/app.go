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
	ErrServerNotInitialized     = errors.New("server not initialized")
	ErrServerAlreadyInitialized = errors.New("server already initialized")
)

type Server struct {
	broker      internal.BrokerConnection
	executor    internal.Executor
	cmdArgs     []string
	initialized bool

	initTimeout       time.Duration
	serverStopTimeout time.Duration
	sendResultTimeout time.Duration
	sendCancelTimeout time.Duration
}

func NewServer(brokerURI string, containerID string, cmdArgs []string) *Server {
	return &Server{
		broker:            adapters.NewConnection(brokerURI, containerID),
		executor:          adapters.NewExecutor(cmdArgs),
		cmdArgs:           cmdArgs,
		initTimeout:       5 * time.Second,
		serverStopTimeout: 5 * time.Second,
		sendResultTimeout: 15 * time.Second,
		sendCancelTimeout: 5 * time.Second,
	}
}

func (server *Server) Init() error {
	if server.initialized {
		return ErrServerAlreadyInitialized
	}

	if err := server.broker.Open(server.initTimeout); err != nil {
		return err
	}
	server.initialized = true
	return nil
}

func (server *Server) Run(ctx context.Context) error {
	if !server.initialized {
		return ErrServerNotInitialized
	}

	slog.Info("server loop started; ready for requests", "gameServerArgs", server.cmdArgs)

	for ctx.Err() == nil {
		if err := server.runMatch(ctx); err != nil {
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

func (server *Server) runMatch(ctx context.Context) error {
	slog.Debug("waiting for match config...")
	config, err := server.broker.GetMatchConfig(ctx)
	if err != nil {
		return fmt.Errorf("get match config failed: %w", err)
	}
	slog.Info("received match config", "matchID", config.MatchID, "configLen", len(config.Config))
	slog.Debug("match config", "config", config)

	result, err := server.runServer(ctx, string(config.Config))
	if err != nil {
		server.sendCancel(config.MatchID)
		return fmt.Errorf("server execution failed: %w", err)
	}

	slog.Info("match result retrieved; sending to broker", "resultLen", len(result))
	if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		slog.Debug("match result", "result", string(result))
	}

	if err := server.sendResult(ctx, config.MatchID, result); err != nil {
		server.sendCancel(config.MatchID)
		return fmt.Errorf("failed to send result: %w", err)
	}
	return nil
}

func (server *Server) runServer(ctx context.Context, config string) ([]byte, error) {
	slog.Info("starting server...")
	if err := server.executor.Start(config); err != nil {
		return nil, fmt.Errorf("failed to start server: %w", err)
	}
	defer server.stopServer(ctx)

	slog.Info("server started; waiting for the result...")
	result, err := server.executor.GetResult(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get match result: %w", err)
	}
	return result, nil
}

func (server *Server) stopServer(ctx context.Context) {
	timeoutCtx, cancel := context.WithTimeout(ctx, server.serverStopTimeout)
	defer cancel()

	err := server.executor.Stop(timeoutCtx)

	// Stop errors are only logged since failure to stop does not affect server reuse.
	if err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("failed to stop server", "error", err)
	}
}

func (server *Server) sendCancel(matchID int) {
	slog.Warn("sending match cancel...")

	ctx, cancel := context.WithTimeout(context.Background(), server.sendCancelTimeout)
	defer cancel()

	if err := server.broker.SendCancel(ctx, matchID); err != nil {
		slog.Error("send match cancel failed", "error", err)
	}
}

func (server *Server) sendResult(ctx context.Context, matchID int, result []byte) error {
	slog.Info("sending match result...")

	timeoutCtx, cancel := context.WithTimeout(ctx, server.sendResultTimeout)
	defer cancel()
	return server.broker.SendResult(timeoutCtx, matchID, result)
}

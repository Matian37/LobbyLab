package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Matian37/multiplayer-asset/game-server/adapters"
	"github.com/Matian37/multiplayer-asset/game-server/internal"

	"github.com/cenkalti/backoff/v6"
)

var (
	ErrServerNotOpened     = errors.New("server not opened")
	ErrServerAlreadyOpened = errors.New("server already opened")
	ErrServerAlreadyClosed = errors.New("server already closed")
)

// Server manages a worker's life cycle. It owns internal.BrokerConnection and
// internal.Executor instances. It runs in loop, handling one match per iteration,
// until the context is canceled.
//
// Server cannot be reused after it is closed.
type Server struct {
	broker   internal.BrokerConnection
	executor internal.Executor
	cmdArgs  []string
	logger   *slog.Logger
	opened   bool
	closed   bool

	initTimeout       time.Duration
	serverStopTimeout time.Duration
	sendResultTimeout time.Duration
	sendCancelTimeout time.Duration
}

func NewServer(brokerURI string, containerID string, cmdArgs []string, logger *slog.Logger) *Server {
	return &Server{
		broker:            adapters.NewConnection(brokerURI, containerID, logger),
		executor:          adapters.NewExecutor(cmdArgs, logger),
		cmdArgs:           cmdArgs,
		logger:            logger.With("component", "server"),
		initTimeout:       5 * time.Second,
		serverStopTimeout: 5 * time.Second,
		sendResultTimeout: 15 * time.Second,
		sendCancelTimeout: 5 * time.Second,
	}
}

func (s *Server) Open() error {
	if s.closed {
		return ErrServerAlreadyClosed
	}
	if s.opened {
		return ErrServerAlreadyOpened
	}

	if err := s.broker.Open(s.initTimeout); err != nil {
		return err
	}
	s.opened = true
	return nil
}

func (s *Server) Close() error {
	if !s.opened {
		return ErrServerNotOpened
	}
	if s.closed {
		return ErrServerAlreadyClosed
	}
	s.closed = true

	s.stopServer(context.Background())
	return s.broker.Close()
}

// Run runs the loop handling one match per iteration until the context is
// canceled. A failed match does not stop the loop; the next iteration is
// retried with exponential backoff.
func (s *Server) Run(ctx context.Context) error {
	if !s.opened {
		return ErrServerNotOpened
	}
	if s.closed {
		return ErrServerAlreadyClosed
	}

	s.logger.Info("server loop started; ready for requests", "command", s.cmdArgs)

	b := backoff.NewExponentialBackOff()

	var iterationErr error
	for ctx.Err() == nil {
		HandleBackoff(ctx, b, iterationErr)

		err := s.runMatch(ctx)
		if errors.Is(err, context.Canceled) {
			break
		} else if err != nil {
			s.logger.Error("match execution failed", "error", err)
		} else {
			s.logger.Info("match execution successful")
		}

		iterationErr = err
	}

	return ctx.Err()
}

// Fetches a match configuration from the broker, runs the game server, and sends the result back
func (s *Server) runMatch(ctx context.Context) error {
	s.logger.Debug("waiting for match config...")
	config, err := s.broker.GetMatchConfig(ctx)
	if err != nil {
		return fmt.Errorf("get match config failed: %w", err)
	}

	s.logger.Info("received match config", "matchID", config.MatchID, "configLen", len(config.Config))
	s.logger.Debug("match config", "config", config)

	result, err := s.runServer(ctx, string(config.Config))
	if err != nil {
		s.sendCancel(config.MatchID)
		return fmt.Errorf("server execution failed: %w", err)
	}

	s.logger.Info("received match result; sending to broker", "resultLen", len(result))
	s.logger.Debug("match result", "result", string(result))

	if err := s.sendResult(ctx, config.MatchID, result); err != nil {
		s.sendCancel(config.MatchID)
		return fmt.Errorf("failed to send result: %w", err)
	}
	return nil
}

// Starts executor and waits for the result, after that it stops the executor
func (s *Server) runServer(ctx context.Context, config string) ([]byte, error) {
	s.logger.Info("starting server...")
	if err := s.executor.Start(config); err != nil {
		return nil, fmt.Errorf("failed to start server: %w", err)
	}
	defer s.stopServer(ctx)

	s.logger.Info("server started; waiting for the result...")
	result, err := s.executor.GetResult(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get match result: %w", err)
	}
	return result, nil
}

// Wraps executor.Stop with logger and timeout
// If the stop fails, the error is logged but not returned.
func (s *Server) stopServer(ctx context.Context) {
	timeoutCtx, cancel := context.WithTimeout(ctx, s.serverStopTimeout)
	defer cancel()

	err := s.executor.Stop(timeoutCtx)
	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, adapters.ErrExecutorNotActive) {
		s.logger.Error("failed to stop server", "error", err)
	}
}

// Wraps broker.SendCancel with logger and timeout
// If send fails, the error is logged but not returned.
func (s *Server) sendCancel(matchID int) {
	s.logger.Warn("sending match cancel...")

	ctx, cancel := context.WithTimeout(context.Background(), s.sendCancelTimeout)
	defer cancel()

	if err := s.broker.SendCancel(ctx, matchID); err != nil {
		s.logger.Error("send match cancel failed", "error", err)
	}
}

// Wraps broker.SendResult with logger and timeout
func (s *Server) sendResult(ctx context.Context, matchID int, result []byte) error {
	s.logger.Info("sending match result...")

	timeoutCtx, cancel := context.WithTimeout(ctx, s.sendResultTimeout)
	defer cancel()
	return s.broker.SendResult(timeoutCtx, matchID, result)
}

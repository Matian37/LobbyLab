package app

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/Matian37/multiplayer-asset/server-manager/adapters"
	"github.com/Matian37/multiplayer-asset/server-manager/internal"

	"github.com/cenkalti/backoff/v6"
)

var (
	ErrNotEnoughUsers        = errors.New("not enough users")
	ErrMatchmakerClosed      = errors.New("matchmaker closed")
	ErrMatchmakerAlreadyOpen = errors.New("matchmaker already open")
)

type Matchmaker struct {
	workerManager internal.WorkerManager
	db            internal.DatabaseConnection
	config        *internal.EnvConfig

	dbPoolTimeout time.Duration

	opened bool
	closed bool

	logger *slog.Logger

	wg sync.WaitGroup
}

func NewMatchmaker(workerManager internal.WorkerManager, config *internal.EnvConfig, logger *slog.Logger) *Matchmaker {
	return &Matchmaker{
		workerManager: workerManager,
		db:            adapters.NewDatabaseConnection(config),
		config:        config,
		dbPoolTimeout: 150 * time.Millisecond,
		logger:        logger.With("service", "matchmaker"),
	}
}

func (m *Matchmaker) Start(ctx context.Context) error {
	if m.closed {
		return ErrMatchmakerClosed
	}
	if m.opened {
		return ErrMatchmakerAlreadyOpen
	}

	if err := m.db.Open(ctx); err != nil {
		return err
	}

	if err := m.db.SetupMatchmaking(ctx); err != nil {
		return err
	}

	m.wg.Go(func() {
		m.logger.Debug("starting core loop")
		err := m.matchmakingLoop(ctx)
		// nil check not required; loop shouldn't return it as error
		// context errors will be hidden by handler accordingly
		m.logger.Error("core loop exited", "error", err)
	})

	m.opened = true
	return nil
}

func (m *Matchmaker) Shutdown() {
	if m.closed {
		return
	}

	if m.db != nil {
		err := m.db.Close()
		if err != nil {
			m.logger.Error("failed to close db", "error", err)
		}
	}

	m.logger.Debug("waiting for core loop to exit")
	m.wg.Wait()
	m.closed = true

	m.logger.Debug("shutdown complete")
}

func (m *Matchmaker) createMatch(ctx context.Context, matchUsers []internal.User) (int, error) {
	// TODO: pass universal seceret, which distinguish players from unauthorized users
	gameConfig, err := json.Marshal(struct {
		Players []internal.User `json:"players"`
	}{Players: matchUsers})
	if err != nil {
		return 0, err
	}

	ctxTimeout, cancelTimeout := context.WithTimeout(ctx, 10*time.Second)
	defer cancelTimeout()

	matchID, err := m.db.GetNextMatchId(ctxTimeout)
	if err != nil {
		return 0, err
	}

	matchConfig := internal.MatchConfig{
		Config:  gameConfig,
		MatchID: matchID,
	}
	serverInfo, err := m.workerManager.AssignMatch(ctxTimeout, matchConfig)
	if err != nil {
		return 0, err
	}

	// FIX: if add match fails then send cancel match job to worker
	// 		this require creating cancel feature in game-server,
	// 		so responsibility of stopping match is on actual game server side
	return matchID, m.db.AddMatch(ctxTimeout, matchUsers, serverInfo, matchID)
}

func (m *Matchmaker) matchmakingLoop(ctx context.Context) error {
	backoff := backoff.NewExponentialBackOff()

	var iterationErr error
	for {
		HandleBackoff(ctx, backoff, iterationErr)

		m.logger.Debug("waiting for a free worker")
		m.workerManager.WaitForFreeWorker(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}

		m.logger.Debug("waiting for enough players")
		users, err := m.waitForEnoughPlayers(ctx)
		if err != nil {
			m.logger.Error("failed to wait for enough players", "error", err)
			iterationErr = err
			continue
		}

		users, err = m.db.GenerateAuthTokens(ctx, users)
		if err != nil {
			m.logger.Error("failed to set match auth tokens for users", "error", err)
			iterationErr = err
			continue
		}

		m.logger.Info("creating a match for users", "users", users)
		matchID, err := m.createMatch(ctx, users)
		if err != nil {
			if errors.Is(err, internal.ErrDBNotEnoughPlayers) {
				m.logger.Info("match creation failed due to decrease in number of players")
				iterationErr = nil
			} else {
				m.logger.Error("failed to matchmake", "error", err)
				iterationErr = err
			}
			continue
		}

		m.logger.Info("match created successfully", "matchID", matchID)
		iterationErr = nil
	}
}

func (m *Matchmaker) waitForEnoughPlayers(ctx context.Context) ([]internal.User, error) {
	ticker := time.NewTicker(m.dbPoolTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return nil, ctx.Err()
		}

		users, err := m.db.GatherMatchPlayers(ctx)
		if err != nil {
			if errors.Is(err, internal.ErrDBNotEnoughPlayers) {
				m.logger.Debug("waiting for players...")
				continue
			}
			return nil, err
		}

		return users, nil
	}
}

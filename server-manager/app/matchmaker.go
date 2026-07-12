package app

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"server-manager/adapters"
	"server-manager/internal"
	"sync"
	"time"

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

// TODO: return id for logs
func (m *Matchmaker) createMatch(ctx context.Context, matchUsers []internal.User) error {
	gameConfig, err := json.Marshal(struct {
		Players []internal.User `json:"players"`
	}{Players: matchUsers})
	if err != nil {
		return err
	}

	ctxTimeout, cancelTimeout := context.WithTimeout(ctx, 10*time.Second)
	defer cancelTimeout()

	matchId, err := m.db.GetNextMatchId(ctxTimeout)
	if err != nil {
		return err
	}

	matchConfig := internal.MatchConfig{
		Config:  gameConfig,
		MatchID: matchId,
	}
	serverInfo, err := m.workerManager.AssignMatch(ctxTimeout, matchConfig)
	if err != nil {
		return err
	}

	// FIX: if add match fails then send cancel match job to worker
	// 		this require creating cancel feature in game-server,
	// 		so responsibility of stopping match is on actual game server side
	return m.db.AddMatch(ctxTimeout, matchUsers, serverInfo, matchId)
}

// TODO: use HandleBackoff
func (m *Matchmaker) matchmakingLoop(ctx context.Context) error {
	backoff := backoff.NewExponentialBackOff()

	for {
		m.logger.Debug("waiting for a free worker")
		m.workerManager.WaitForFreeWorker(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}

		m.logger.Debug("waiting for enough players")
		users, err := m.waitForEnoughPlayers(ctx)
		if err != nil {
			m.logger.Error("failed to wait for enough players", "error", err)
			time.Sleep(backoff.NextBackOff())
			continue
		}

		m.logger.Info("creating a match for users", "users", users)
		if err := m.createMatch(ctx, users); err != nil {
			if errors.Is(err, internal.ErrDBNotEnoughPlayers) {
				m.logger.Info("match creation failed due to decrease in number of players")
				backoff.Reset()
				continue
			}
			m.logger.Error("failed to matchmake", "error", err)
			time.Sleep(backoff.NextBackOff())
			continue
		}
		m.logger.Info("match created successfully")

		backoff.Reset()
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

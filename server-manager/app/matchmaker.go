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
	ErrNotEnoughUsers            = errors.New("not enough users")
	ErrMatchmakerClosed          = errors.New("matchmaker closed")
	ErrMatchmakerAlreadyOpen     = errors.New("matchmaker already open")
	ErrMatchmakerAlreadyShutdown = errors.New("matchmaker already shutdown")
)

type Matchmaker struct {
	workerManager internal.WorkerManager
	db            internal.DatabaseConnection
	config        *internal.EnvConfig

	dbPoolTimeout time.Duration

	opened bool
	closed bool

	wg sync.WaitGroup
}

func NewMatchmaker(workerManager internal.WorkerManager, config *internal.EnvConfig) *Matchmaker {
	return &Matchmaker{
		workerManager: workerManager,
		db:            adapters.NewDatabaseConnection(config),
		config:        config,
		dbPoolTimeout: 150 * time.Millisecond,
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

	m.wg.Go(func() { _ = m.matchmakingLoop(ctx) })

	m.opened = true
	return nil
}

func (m *Matchmaker) Shutdown() error {
	if m.closed {
		return ErrMatchmakerAlreadyShutdown
	}

	var err error
	if m.db != nil {
		err = m.db.Close()
	}
	m.wg.Wait()

	m.closed = true
	return err
}

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

func (m *Matchmaker) matchmakingLoop(ctx context.Context) error {
	backoff := backoff.NewExponentialBackOff()

	for {
		m.workerManager.WaitForFreeWorker(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}

		users, err := m.waitForEnoughPlayers(ctx)
		if err != nil {
			slog.Error("failed to wait for enough players", "error", err)
			time.Sleep(backoff.NextBackOff())
			continue
		}

		if err := m.createMatch(ctx, users); err != nil {
			// FIX: skip sleep and logging when match creation failed due to waiting queue
			// 		then reset backoff
			slog.Error("failed to matchmake", "error", err)
			time.Sleep(backoff.NextBackOff())
			continue
		}

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
				continue
			}
			return nil, err
		}

		return users, nil
	}
}

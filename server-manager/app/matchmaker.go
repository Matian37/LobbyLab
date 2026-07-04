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

	opened bool
	closed bool

	wg sync.WaitGroup
}

func NewMatchmaker(workerManager internal.WorkerManager, config *internal.EnvConfig) *Matchmaker {
	return &Matchmaker{
		workerManager: workerManager,
		db:            adapters.NewDatabaseConnection(config),
		config:        config,
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

	if err := m.db.StartListening(ctx); err != nil {
		return err
	}

	m.wg.Go(func() { m.listenLoop(ctx) })

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

func (m *Matchmaker) createMatches(ctx context.Context, users []internal.User) error {
	if len(users) < m.config.PlayersPerRoom {
		return nil
	}

	for i := m.config.PlayersPerRoom; i <= len(users); i += m.config.PlayersPerRoom {
		matchUsers := users[i-m.config.PlayersPerRoom : i]

		gameConfig, err := json.Marshal(struct{ Players []internal.User }{Players: matchUsers})
		if err != nil {
			return err
		}

		matchId, err := m.db.GetNextMatchId(ctx)
		if err != nil {
			return err
		}

		matchConfig := internal.MatchConfig{
			Config:  gameConfig,
			MatchID: matchId,
		}
		serverInfo, err := m.workerManager.AssignMatch(ctx, matchConfig)
		if err != nil {
			return err
		}

		if err = m.db.AddMatch(ctx, matchUsers, serverInfo, matchId); err != nil {
			return err
		}
	}

	return nil
}

func (m *Matchmaker) listenLoop(ctx context.Context) {
	for {
		_ = m.db.ListenForQueueChange(ctx)
		if ctx.Err() != nil {
			return
		}

		if err := m.runMatchmaking(ctx); err != nil {
			slog.Error("failed to matchmake", "error", err)
		}
	}
}

func (m *Matchmaker) runMatchmaking(ctx context.Context) error {
	ctxTimeout, cancelTimeout := context.WithTimeout(ctx, 10*time.Second)
	defer cancelTimeout()

	users, err := m.db.GetList(ctxTimeout)
	if err != nil {
		return err
	}
	return m.createMatches(ctxTimeout, users)
}

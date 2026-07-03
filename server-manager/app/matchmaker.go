package app

import (
	"context"
	"encoding/json"
	"errors"
	"server-manager/internal"
	"sync"
	"time"
)

var ErrNotEnoughUsers = errors.New("not enough users")

type Matchmaker struct {
	workerManager  internal.WorkerManager
	db             internal.DatabaseConnection
	playersPerRoom int
	config         internal.EnvConfig

	wg sync.WaitGroup
}

func (m *Matchmaker) StartMatchmaking(ctx context.Context) error {
	if err := m.db.Init(ctx, &m.config); err != nil {
		return err
	}

	if err := m.db.StartListening(ctx); err != nil {
		return err
	}

	m.wg.Go(func() { m.listenLoop(ctx) })

	return nil
}

func (m *Matchmaker) CreateMatches(ctx context.Context, users []internal.User) error {
	if len(users) < m.playersPerRoom {
		return ErrNotEnoughUsers
	}

	for i := m.playersPerRoom; i <= len(users); i += m.playersPerRoom {
		matchUsers := users[i-m.playersPerRoom : i]

		gameConfig, err := json.Marshal(struct{ Players []internal.User }{Players: users})
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
		select {
		case <-ctx.Done():
			return
		default:
		}

		_ = m.db.ListenForQueueChange(ctx)
		if ctx.Err() != nil {
			return
		}

		// TODO: handle error somehow
		_ = m.runMatchmaking(ctx)
	}
}

func (m *Matchmaker) runMatchmaking(ctx context.Context) error {
	ctxTimeout, cancelTimeout := context.WithTimeout(ctx, 10*time.Second)
	defer cancelTimeout()

	users, err := m.db.GetList(ctxTimeout)
	if err != nil {
		return err
	}
	return m.CreateMatches(ctxTimeout, users)
}

func (m *Matchmaker) WaitForShutdown() {
	m.wg.Wait()
}

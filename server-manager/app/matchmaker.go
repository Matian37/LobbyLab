package app

import (
	"context"
	"encoding/json"
	"errors"
	"server-manager/internal"
	"sync"
	"time"
)

type Matchmaker struct {
	WorkerManager  internal.WorkerManager
	Db             internal.DatabaseConnection
	PlayersPerRoom int
	EnvConfig      internal.EnvConfig

	wg sync.WaitGroup
}

func (m *Matchmaker) StartMatchmaking(ctx context.Context) error {
	err := m.Db.Init(ctx, &m.EnvConfig)
	if err != nil {
		return errors.New("error while initializing db")
	}

	if err := m.Db.StartListening(ctx); err != nil {
		return err
	}

	m.wg.Go(func() { m.listenLoop(ctx) })

	return nil
}

func (m *Matchmaker) CreateMatches(ctx context.Context, users []internal.User) error {
	if len(users) < m.PlayersPerRoom {
		return errors.New("not enough users")
	}

	for i := 0; i < len(users); i += m.PlayersPerRoom {
		var matchUsers []internal.User
		for j := 0; j < m.PlayersPerRoom; j++ {
			if i+j >= len(users) {
				break
			}
			matchUsers = append(matchUsers, users[i+j])
		}
		if len(matchUsers) < m.PlayersPerRoom {
			continue
		}

		data, err := json.Marshal(struct{ Players []internal.User }{Players: users})
		if err != nil {
			return err
		}
		matchId, err := m.Db.GetNextMatchId(ctx)
		if err != nil {
			return err
		}
		config := internal.MatchConfig{
			Config:  data,
			MatchID: matchId,
		}
		serverInfo, err := m.WorkerManager.AssignMatch(ctx, config)
		if err != nil {
			return err
		}
		err = m.Db.AddMatch(ctx, matchUsers, serverInfo, matchId)
		if err != nil {
			return errors.New("error while creating match")
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

		_ = m.Db.ListenForQueueChange(ctx)
		if ctx.Err() != nil {
			return
		}

		m.runMatchmaking(ctx)
	}
}

func (m *Matchmaker) runMatchmaking(ctx context.Context) {
	ctxTimeout, cancelTimeout := context.WithTimeout(ctx, 10*time.Second)
	defer cancelTimeout()

	users, err2 := m.Db.GetList(ctxTimeout)
	if err2 != nil && len(users) > 1 {
		_ = m.CreateMatches(ctxTimeout, users)
	}
}

func (m *Matchmaker) WaitForShutdown() {
	m.wg.Wait()
}

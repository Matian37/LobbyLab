package app

import (
	"context"
	"encoding/json"
	"errors"
	"server-manager/adapters"
	"server-manager/internal"
)

type Matchmaker struct {
	WorkerManager  internal.WorkerManager
	Db             internal.DatabaseConnection
	PlayersPerRoom int
	EnvConfig      internal.EnvConfig
}

func (m *Matchmaker) StartMatchmaking(ctx context.Context) error {
	dbObj := &adapters.DatabaseConnection{Matchmaker: m}
	err := dbObj.Init(ctx, &m.EnvConfig)
	if err != nil {
		return errors.New("Error while initializing DB")
	}
	m.Db = dbObj

	m.Db.StartListening(ctx)

	return nil
}

func (m *Matchmaker) CreateMatches(ctx context.Context, users []internal.User) error {
	if len(users) < m.PlayersPerRoom {
		return errors.New("Not enough users")
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

package main

import (
	"context"
	"errors"
	"server-manager/internal"
)

type Matchmaker struct {
	WorkerManager  internal.WorkerManager
	Db             internal.DB
	PlayersPerRoom int
}

func NewMatchmaker(workerManager internal.WorkerManager, playersPerRoom int) *Matchmaker {
	m := &Matchmaker{
		WorkerManager:  workerManager,
		PlayersPerRoom: playersPerRoom,
	}

	return m
}

func (m *Matchmaker) StartMatchmaking(ctx context.Context) error {
	dbObj, err := NewDB(ctx, m)
	if err != nil {
		return errors.New("Error while creating DB")
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
		config := internal.NewMatchConfig(matchUsers)

		socket, err := m.WorkerManager.AssignMatch(ctx, config)
		if err != nil {
			return err
		}
		err = m.Db.AddMatch(ctx, matchUsers, socket)
		if err != nil {
			return errors.New("error while creating match")
		}
	}

	return nil
}

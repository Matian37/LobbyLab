package main

import (
	"context"
	"fmt"
	"server-manager/internal"
)

type Matchmaker struct {
	Host, Port, User, Password, Name string
	WorkerManager                    internal.WorkerManager
	Db                               internal.DB
	PlayersPerRoom                   int
}

func NewMatchmaker(workerManager internal.WorkerManager, playersPerRoom int) *Matchmaker {
	return &Matchmaker{
		WorkerManager:  workerManager,
		PlayersPerRoom: playersPerRoom,
	}
}

func (m *Matchmaker) StartMatchmaking(ctx context.Context) error {
	dbObj, err := NewDB(ctx, m)
	if err != nil {
		return nil
	}
	m.Db = dbObj

	m.Db.StartListening()

	return nil
}

func (m *Matchmaker) CreateMatches(ctx context.Context, users []internal.User) error {
	if len(users) < m.PlayersPerRoom {
		return fmt.Errorf("Not enough users")
	}

	var errors int
	for i := 0; i < len(users); i += m.PlayersPerRoom {
		var match_users []internal.User
		for j := 0; j < m.PlayersPerRoom; j++ {
			match_users = append(match_users, users[i+j])
		}
		config := internal.NewMatchConfig(match_users)

		socket, err := m.WorkerManager.AssignMatch(ctx, config)
		if err != nil {
			errors++
			fmt.Println(err)
		}
		err = m.Db.AddMatch(ctx, match_users, socket)
		if err != nil {
			errors++
		}
	}

	if errors > 0 {
		return fmt.Errorf("%d errors when creating matches", errors)
	}
	return nil
}

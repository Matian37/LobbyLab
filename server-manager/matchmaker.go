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

	m.Db.StartListening(ctx)

	return nil
}

func (m *Matchmaker) CreateMatches(ctx context.Context, users []internal.User) error {
	if len(users) < m.PlayersPerRoom {
		return fmt.Errorf("Not enough users")
	}

	for i := 0; i < len(users); i += m.PlayersPerRoom {
		var matchUsers []internal.User
		for j := 0; j < m.PlayersPerRoom; j++ {
			matchUsers = append(matchUsers, users[i+j])
		}
		config := internal.NewMatchConfig(matchUsers)

		socket, err := m.WorkerManager.AssignMatch(ctx, config)
		if err != nil {
			fmt.Println(err)
		}
		err = m.Db.AddMatch(ctx, matchUsers, socket)
		if err != nil {
			fmt.Printf("error while creating %dth match", i)
		}
	}

	return nil
}

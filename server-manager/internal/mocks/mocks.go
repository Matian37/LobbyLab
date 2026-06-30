package internal

import (
	"context"
	"server-manager/internal"
)

type MockWorkerManager struct{}

func (m *MockWorkerManager) Init(ctx context.Context, config *internal.EnvConfig) error {
	return nil
}
func (m *MockWorkerManager) WaitForFreeWorker(ctx context.Context) error { return nil }
func (m *MockWorkerManager) AssignMatch(ctx context.Context, config internal.MatchConfig) (internal.ServerInfo, error) {
	return internal.ServerInfo{Host: "http://jakis/host", Port: "2137"}, nil
}
func (m *MockWorkerManager) Close() error                         { return nil }
func (m *MockWorkerManager) Run(ctx context.Context)              {}
func (m *MockWorkerManager) SaveLoop(ctx context.Context) error   { return nil }
func (m *MockWorkerManager) ResultLoop(ctx context.Context) error { return nil }
func (m *MockWorkerManager) HealthLoop(ctx context.Context) error { return nil }

type MockDB struct{}

func (d *MockDB) Init(ctx context.Context, config *internal.EnvConfig) error {
	return nil
}

func (d *MockDB) Close() error {
	return nil
}

func (d *MockDB) StartListening(ctx context.Context) error {
	return nil
}

func (d *MockDB) GetList(ctx context.Context) (error, []internal.User) {
	return nil, []internal.User{}
}

func (d *MockDB) AddMatch(ctx context.Context, users []internal.User, serverInfo internal.ServerInfo, matchId int) error {
	return nil
}

func (d *MockDB) SaveMatchResults(ctx context.Context, details string, matchId int) error {
	return nil
}

func (d *MockDB) GetNextMatchId(ctx context.Context) (int, error) {
	return 0, nil
}

type MockMatchmaker struct{}

func (m *MockMatchmaker) StartMatchmaking(ctx context.Context) error {
	return nil
}

func (m *MockMatchmaker) CreateMatches(ctx context.Context, users []internal.User) error {
	return nil
}

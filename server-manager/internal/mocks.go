package internal

import (
	"context"
)

type MockWorkerManager struct{}

func GetMockWorkerManager() *MockWorkerManager {
	return &MockWorkerManager{}
}

func (m *MockWorkerManager) Init(ctx context.Context, config *EnvConfig, workerCount int) error {
	return nil
}
func (m *MockWorkerManager) WaitForFreeWorker(ctx context.Context) {}
func (m *MockWorkerManager) AssignMatch(ctx context.Context, config *MatchConfig) (Socket, error) {
	return *NewSocket("http://jakis/host", "2137"), nil
}
func (m *MockWorkerManager) Monitor(ctx context.Context)     {}
func (m *MockWorkerManager) SaveResults(ctx context.Context) {}
func (m *MockWorkerManager) Close(ctx context.Context)       {}

type MockDB struct{}

func GetMockDB() *MockDB {
	return &MockDB{}
}

func (d *MockDB) StartListening(ctx context.Context) error {
	return nil
}

func (d *MockDB) GetList(ctx context.Context) (error, []User) {
	return nil, []User{}
}

func (d *MockDB) AddMatch(ctx context.Context, users []User, socket Socket) error {
	return nil
}

type MockMatchmaker struct{}

func GetMockMatchmaker() *MockMatchmaker {
	return &MockMatchmaker{}
}

func (m *MockMatchmaker) StartMatchmaking(ctx context.Context) error {
	return nil
}

func (m *MockMatchmaker) CreateMatches(ctx context.Context, users []User) error {
	return nil
}

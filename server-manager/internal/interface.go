//go:generate go run go.uber.org/mock/mockgen -source=./interface.go -destination=./mocks/interface.go -package=mocks

package internal

import (
	"context"
	"time"
)

type DockerConnection interface {
	Open(config *EnvConfig) error
	Close() error
	SpawnContainer(ctx context.Context) (string, error)
	RestartContainer(ctx context.Context, id string) error
	KillContainer(ctx context.Context, id string) error
	GetGamePort(ctx context.Context, containerID string) (string, error)
}

type BrokerConnection interface {
	Open(ctx context.Context, config *EnvConfig) error
	Close() error
	AssignJob(ctx context.Context, workerID string, config MatchConfig) error
	// GetWorkersPong broadcasts a ping to all active workers
	// and returns those that respond before the timeout.
	GetWorkersPong(ctx context.Context, pongTimeout time.Duration) (Responders, error)
	GetResult(ctx context.Context) (Message, error)
}

type WorkerManager interface {
	Start(ctx context.Context) error
	Shutdown() error
	WaitForFreeWorker(ctx context.Context)
	AssignMatch(ctx context.Context, config MatchConfig) (ServerInfo, error)
}

type DatabaseConnection interface {
	Open(ctx context.Context) error
	Close() error
	SaveMatchResults(ctx context.Context, results Result) error
	GatherMatchPlayers(ctx context.Context) ([]User, error)
	AddMatch(ctx context.Context, users []User, serverInfo ServerInfo, matchId int) error
	GetNextMatchId(ctx context.Context) (int, error)
}

type Message interface {
	Data() []byte
	Ack() error
}

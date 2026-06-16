package internal

import (
	"context"
	"time"

	"github.com/moby/moby/api/types/network"
)

type DockerClient interface {
	Init(config *EnvConfig) error
	CreateWorkerContainer(ctx context.Context) (string, error)
	IsContainerOK(ctx context.Context, id string) (bool, error)
	RestartContainer(ctx context.Context, id string) error
	KillContainer(ctx context.Context, id string) error
	GetPorts(ctx context.Context, containerID string) (network.PortMap, error)
	Close() error
}

type BrokerConnection interface {
	Open(timeout time.Duration, config *EnvConfig) error
	SendPing(ctx context.Context) error
	GetPong(ctx context.Context) (string, time.Time, error)
	AssignJob(ctx context.Context, workerID string, config string) error
	Close() error
}

type WorkerManager interface {
	Init(ctx context.Context, config *EnvConfig, workerCount int) error
	WaitForFreeWorker(ctx context.Context)
	AssignMatch(ctx context.Context, config *MatchConfig) (Socket, error)
	Monitor(ctx context.Context)
	SaveResults(ctx context.Context)
	Close(ctx context.Context)
}

type Matchmaker interface {
	StartMatchmaking(ctx context.Context) error
	CreateMatches(ctx context.Context, users []User) error
}

type DB interface {
	StartListening() error
	GetList(ctx context.Context) (error, []User)
	AddMatch(ctx context.Context, users []User, socket Socket) error
}

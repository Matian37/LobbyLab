package internal

import (
	"context"

	"github.com/moby/moby/api/types/network"
)

type DockerClient interface {
	Init(config *EnvConfig) error
	CreateWorkerContainer(ctx context.Context) (string, error)
	// TODO: make it concurrent
	RestartContainer(ctx context.Context, id string) error
	KillContainer(ctx context.Context, id string) error
	GetGamePorts(ctx context.Context, containerID string) (network.PortMap, error)
	IsContainerStarted(ctx context.Context, containerID string) (bool, error)
	Close() error
}

type BrokerConnection interface {
	Open(ctx context.Context, config *EnvConfig) error
	AssignJob(ctx context.Context, workerID string, config string) error
	SendPing() (Responders, error)
	GetResult(ctx context.Context) (Message, error)
	GetFinish(ctx context.Context) (string, error)
	Close() error
}

type WorkerManager interface {
	Init(ctx context.Context, config *EnvConfig, workerCount int) error
	WaitForFreeWorker(ctx context.Context) error
	AssignMatch(ctx context.Context, matchID int, config string) (network.PortMap, error)
	HealthLoop(ctx context.Context) error
	Close(ctx context.Context) error
}

type DatabaseConnection interface {
	Init(ctx context.Context, config *EnvConfig) error
	SaveMatchResult(ctx context.Context, success bool, result string) error
	Close() error
}

type Message interface {
	Data() []byte
	Ack() error
}

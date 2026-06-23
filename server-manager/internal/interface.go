package internal

import "context"

type DockerConnection interface {
	Init(config *EnvConfig) error
	SpawnContainer(ctx context.Context) (string, error)
	RestartContainer(ctx context.Context, id string) error
	KillContainer(ctx context.Context, id string) error
	GetGamePort(ctx context.Context, containerID string) (string, error)
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
	Init(ctx context.Context, config *EnvConfig) error
	Close(ctx context.Context) error
	SaveLoop(ctx context.Context) error
	ResultLoop(ctx context.Context) error
	HealthLoop(ctx context.Context) error
	WaitForFreeWorker(ctx context.Context) error
	AssignMatch(ctx context.Context, matchID int, config string) (ServerInfo, error)
}

type DatabaseConnection interface {
	Init(ctx context.Context, config *EnvConfig) error
	SaveMatchResult(ctx context.Context, result Result) error
	Close() error
}

type Message interface {
	Data() []byte
	Ack() error
}

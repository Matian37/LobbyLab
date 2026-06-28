//go:generate go run go.uber.org/mock/mockgen -source=./interface.go -destination=./mocks/interface.go -package=mocks

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
	AssignJob(ctx context.Context, workerID string, config MatchConfig) error
	// GetWorkersPong broadcasts a ping to all active workers
	// and returns those that respond before the timeout.
	GetWorkersPong(ctx context.Context) (Responders, error)
	GetResult(ctx context.Context) (Message, error)
	Close() error
}

type WorkerManager interface {
	Init(ctx context.Context, config *EnvConfig) error
	Run(ctx context.Context)
	Close() error
	SaveLoop(ctx context.Context) error
	ResultLoop(ctx context.Context) error
	HealthLoop(ctx context.Context) error
	WaitForFreeWorker(ctx context.Context) error
	AssignMatch(ctx context.Context, config MatchConfig) (ServerInfo, error)
}

type Message interface {
	Data() []byte
	Ack() error
}

type Matchmaker interface {
	StartMatchmaking(ctx context.Context) error
	CreateMatches(ctx context.Context, users []User) error
}

type DB interface {
	StartListening(ctx context.Context) error
	GetList(ctx context.Context) (error, []User)
	AddMatch(ctx context.Context, users []User, socket ServerInfo) error
	SaveMatchResults(ctx context.Context, details string, match_id int) error
}

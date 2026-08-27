//go:generate go run go.uber.org/mock/mockgen -source=./interface.go -destination=./mocks/interface.go -package=mocks

package internal

import (
	"context"
	"time"
)

// Responsibility of handling timeouts per operations is on implementation side.
// Caller should not worry about it.

// Sends requests to the Docker daemon to manage worker containers.
type DockerConnection interface {
	Open(config *EnvConfig) error
	Close() error

	// Spawns worker container with the given workerID as environment var.
	// Implementation must ensure container is not removed automatically
	// after stopping/exiting, so that it can be restarted later.
	SpawnContainer(ctx context.Context, workerID string) (string, error)
	RestartContainer(ctx context.Context, containerID string) error
	RemoveContainer(ctx context.Context, containerID string) error
	// Returns external port for player to connect to container.
	GetGamePort(ctx context.Context, containerID string) (string, error)
	// Removes workers which are left over from previous server-manager run.
	RemoveZombieWorkers(ctx context.Context) error
}

// Handles communication with broker.
type BrokerConnection interface {
	Open(ctx context.Context, config *EnvConfig) error
	Close() error

	AssignJob(ctx context.Context, workerID string, config MatchConfig) error
	GetResult(ctx context.Context) (Message, error)

	// Broadcasts a ping to all active workers
	// and returns those that respond before the timeout.
	GetWorkersPong(ctx context.Context, pongTimeout time.Duration) (Responders, error)
}

// Message abstracts a single message received from the broker.
type Message interface {
	Data() []byte
	// Sends an acknowledgment to the broker that the message has been processed.
	Ack() error
}

// Manages worker containers and assigns matches to them.
type WorkerManager interface {
	Start(ctx context.Context) error
	Shutdown()
	WaitForFreeWorker(ctx context.Context)
	AssignMatch(ctx context.Context, config MatchConfig) (ServerInfo, error)
}

// Handles connection to the database.
// Each database operation here must be atomic.
type DatabaseConnection interface {
	Open(ctx context.Context) error
	Close() error

	// Sets all users to not active (not waiting and not in any match),
	// and sets all active matches in database to be canceled.
	SetupMatchmaking(ctx context.Context) error

	// Returns users waiting for a match of number PLAYERS_PER_ROOM.
	// If there are not enough users, it returns internal.ErrDBNotEnoughPlayers
	// It does not update or hold anything in database
	GatherMatchPlayers(ctx context.Context) ([]User, error)
	// Sets matchAuthToken in database to random string
	// for each user in this given slice
	// and returns the updated slice.
	GenerateAuthTokens(ctx context.Context, users []User) ([]User, error)

	// Returns the next match ID to be used for a new match.
	GetNextMatchId(ctx context.Context) (int, error)
	AddMatch(ctx context.Context, users []User, serverInfo ServerInfo, matchId int) error

	// Removes match status from users, which belonged to the match with the given matchID.
	RemoveMatchStatus(ctx context.Context, matchID int) error
	SaveMatchResults(ctx context.Context, results Result) error
}

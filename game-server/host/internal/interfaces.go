//go:generate go run go.uber.org/mock/mockgen -source=./interfaces.go -destination=./mocks/mocks.go -package=mocks

package internal

import (
	"context"
	"time"
)

// BrokerConnection handles connection and communication with the message
// broker. A connection must not allow reopening after being closed.
type BrokerConnection interface {
	// Note: timeout is used here instead of context due to broker's library implementation.
	Open(timeout time.Duration) error
	Close() error
	GetMatchConfig(ctx context.Context) (MatchConfig, error)
	SendCancel(ctx context.Context, matchID int) error
	SendResult(ctx context.Context, matchID int, result []byte) error
}

// Executor handles the runtime of the game-server process.
// The implementation must allow the executor to start again after stopping.
type Executor interface {
	Start(config string) error
	Stop(ctx context.Context) error
	GetResult(ctx context.Context) ([]byte, error)
}

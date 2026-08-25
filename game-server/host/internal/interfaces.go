//go:generate go run go.uber.org/mock/mockgen -source=./interfaces.go -destination=./mocks/mocks.go -package=mocks

package internal

import (
	"context"
	"time"
)

// Handles connection and communication with the message broker.
// Connection must not allow for reopening after closing.
type BrokerConnection interface {
	// Note: timeout is used here instead of context due to broker's library implementation.
	Open(timeout time.Duration) error
	Close() error
	GetMatchConfig(ctx context.Context) (MatchConfig, error)
	SendCancel(ctx context.Context, matchID int) error
	SendResult(ctx context.Context, matchID int, result []byte) error
}

// Handles runtime of game server process.
// Implementation allow executor to start again after stopping.
type Executor interface {
	Start(config string) error
	Stop(ctx context.Context) error
	GetResult(ctx context.Context) ([]byte, error)
}

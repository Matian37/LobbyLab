package internal

import (
	"context"
	"time"
)

//go:generate go run go.uber.org/mock/mockgen -source=./interfaces.go -destination=./mocks/mocks.go -package=mocks

type BrokerConnection interface {
	Open(timeout time.Duration) error
	Close() error
	GetMatchConfig(ctx context.Context) (MatchConfig, error)
	SendCancel(ctx context.Context, matchID int) error
	SendResult(ctx context.Context, matchID int, result []byte) error
}

type Server interface {
	Start(config string, command []string) error

	// After Stop returns, the server must be ready to start again.
	// This is a required invariant.
	Stop(ctx context.Context) error

	GetResult(ctx context.Context) ([]byte, error)
}

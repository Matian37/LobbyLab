package domain

import (
	"context"
	"time"
)

//go:generate go run go.uber.org/mock/mockgen -source=./interfaces.go -destination=../mocks/mocks.go -package=mocks

type BrokerConnection interface {
	Open(timeout time.Duration) error
	Close() error
	GetMatchConfig(ctx context.Context) (string, error)
	SendCancel() error
	SendResult(result []byte) error
}

type Server interface {
	Start(config string, command []string) error
	Stop(ctx context.Context) error
	GetResult(ctx context.Context) ([]byte, error)
}

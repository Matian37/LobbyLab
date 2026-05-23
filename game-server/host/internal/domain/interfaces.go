package domain

import "context"

type Connection interface {
	Connect(ctx context.Context) error
	Close(ctx context.Context) error
	GetStartRequest(ctx context.Context) ([]byte, error)
	SendMatchResult(ctx context.Context, payload []byte) error
}

type Server interface {
	Start(config string, command []string) error
	Stop(ctx context.Context) error
	GetResult(ctx context.Context) ([]byte, error)
}

package adapters

import (
	"context"
	"server-manager/internal"
)

type DatabaseConnection struct{}

func (db *DatabaseConnection) Init(ctx context.Context, config *internal.EnvConfig) error {
	return nil
}

func (db *DatabaseConnection) SaveMatchResults(ctx context.Context, details string, matchID int) error {
	return nil
}

func (db *DatabaseConnection) Close() error {
	return nil
}

func NewDBConnection() internal.DatabaseConnection {
	return &DatabaseConnection{}
}

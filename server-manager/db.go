package main

import (
	"context"
	"server-manager/internal"
)

type DatabaseConnection struct{}

func (db *DatabaseConnection) Init(ctx context.Context, config *internal.EnvConfig) error {
	return nil
}

func (db *DatabaseConnection) SaveMatchResult(ctx context.Context, result internal.Result) error {
	return nil
}
func (db *DatabaseConnection) Close() error {
	return nil
}

func NewDBConnection() internal.DatabaseConnection {
	return &DatabaseConnection{}
}

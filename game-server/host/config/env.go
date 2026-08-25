package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/Matian37/multiplayer-asset/game-server/internal"

	"github.com/caarlos0/env/v11"
	"github.com/google/shlex"
)

var ErrInvalidLogLevel = errors.New("invalid log level")

type parsedConfig struct {
	BrokerURI string `env:"NATS_URI,required,notEmpty"`
	LogLevel  string `env:"LOG_LEVEL,required,notEmpty"`
	WorkerID  string `env:"WORKER_ID,required,notEmpty"`
}

func parseLogLevel(value string) (slog.Level, error) {
	var level slog.Level
	if err := level.UnmarshalText([]byte(value)); err != nil {
		return 0, err
	}
	return level, nil
}

func parseArgs(args []string) ([]string, error) {
	if len(args) != 2 {
		return []string{}, fmt.Errorf("expected 1 argument, got %v", len(args)-1)
	}

	cmdArgs, err := shlex.Split(args[1])
	if err != nil {
		return []string{}, fmt.Errorf("failed to parse command: %w", err)
	}
	return cmdArgs, nil
}

func ReadConfig() (internal.Config, error) {
	parsed := &parsedConfig{}
	if err := env.Parse(parsed); err != nil {
		return internal.Config{}, err
	}

	gameServerArgs, err := parseArgs(os.Args)
	if err != nil {
		return internal.Config{}, fmt.Errorf("%w, \nUsage: %v \"COMMAND\"", err, os.Args[0])
	}

	logLevel, err := parseLogLevel(parsed.LogLevel)
	if err != nil {
		return internal.Config{}, fmt.Errorf("%w: %w", ErrInvalidLogLevel, err)
	}

	return internal.Config{
		BrokerURI:      parsed.BrokerURI,
		WorkerID:       parsed.WorkerID,
		GameServerArgs: gameServerArgs,
		LogLevel:       logLevel,
	}, nil
}

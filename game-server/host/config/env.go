// Package config loads the game-server configuration.
//
// Configuration comes from two sources: required environment variables
// and the single command-line argument that holds the game server command.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/Matian37/LobbyLab/game-server/internal"

	"github.com/caarlos0/env/v11"
	"github.com/google/shlex"
)

// ErrInvalidLogLevel is returned when the LOG_LEVEL environment variable
// cannot be parsed.
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

// Extracts the game server command from the command-line arguments.
// Exactly one argument is expected: the shell command passed after the binary
// name.
//
// Returns the parsed command arguments or an error.
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

// ReadConfig parses the configuration from environment variables and the
// command line, and validates it.
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

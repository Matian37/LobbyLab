package config

import (
	"errors"
	"fmt"
	"log/slog"
	"github.com/Matian37/multiplayer-asset/server-manager/internal"

	"github.com/caarlos0/env/v11"
	"github.com/moby/moby/api/types/network"
)

var (
	ErrWorkerCountNotPositive    = errors.New("worker count not positive")
	ErrClientPortNotInExposed    = errors.New("client port not in expose ports")
	ErrInvalidPortString         = errors.New("invalid port string")
	ErrTooFewPlayersPerRoom      = errors.New("too few players per room; must be atleast two")
	ErrInvalidLogLevel           = errors.New("invalid log level")
	ErrInvalidGameServerLogLevel = errors.New("invalid game server log level")
)

type parsedConfig struct {
	Image              string   `env:"GAME_SERVER_IMAGE,required,notEmpty"`
	WorkerCount        int      `env:"GAME_SERVER_COUNT,required"`
	ExposePorts        []string `env:"GAME_SERVER_EXPOSE_PORTS,required,notEmpty"`
	ClientPort         string   `env:"GAME_SERVER_CLIENT_PORT,required,notEmpty"`
	BrokerURI          string   `env:"NATS_URI,required,notEmpty"`
	BrokerNetworkName  string   `env:"NATS_NETWORK_NAME,required,notEmpty"`
	PublicHost         string   `env:"PUBLIC_HOST,required,notEmpty"`
	PlayersPerRoom     int      `env:"PLAYERS_PER_ROOM,required"`
	DatabaseURI        string   `env:"DATABASE_URI,required,notEmpty"`
	LogLevel           string   `env:"LOG_LEVEL,required,notEmpty"`
	GameServerLogLevel string   `env:"GAME_SERVER_LOG_LEVEL,required,notEmpty"`
}

func parsePorts(ports []string) (network.PortSet, error) {
	parsedPorts := make(network.PortSet)

	for _, portString := range ports {
		port, err := network.ParsePort(portString)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrInvalidPortString, err)
		}
		parsedPorts[port] = struct{}{}
	}

	return parsedPorts, nil
}

func parseLogLevel(value string) (slog.Level, error) {
	var level slog.Level
	if err := level.UnmarshalText([]byte(value)); err != nil {
		return 0, err
	}
	return level, nil
}

func ReadConfig() (*internal.EnvConfig, error) {
	config := &parsedConfig{}

	if err := env.Parse(config); err != nil {
		return nil, err
	}

	if config.WorkerCount < 1 {
		return nil, ErrWorkerCountNotPositive
	}

	if config.PlayersPerRoom < 2 {
		return nil, ErrTooFewPlayersPerRoom
	}

	exposePorts, err := parsePorts(config.ExposePorts)
	if err != nil {
		return nil, err
	}

	clientPort, err := network.ParsePort(config.ClientPort)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidPortString, err)
	}

	if _, ok := exposePorts[clientPort]; !ok {
		return nil, ErrClientPortNotInExposed
	}

	logLevel, err := parseLogLevel(config.LogLevel)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidLogLevel, err)
	}

	gameServerLogLevel, err := parseLogLevel(config.GameServerLogLevel)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidGameServerLogLevel, err)
	}

	return &internal.EnvConfig{
		Image:              config.Image,
		Workercount:        config.WorkerCount,
		ExposePorts:        exposePorts,
		ClientPort:         clientPort,
		BrokerURI:          config.BrokerURI,
		BrokerNetworkName:  config.BrokerNetworkName,
		PublicHost:         config.PublicHost,
		PlayersPerRoom:     config.PlayersPerRoom,
		DatabaseURI:        config.DatabaseURI,
		LogLevel:           logLevel,
		GameServerLogLevel: gameServerLogLevel,
	}, nil
}

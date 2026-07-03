package config

import (
	"errors"
	"fmt"
	"server-manager/internal"

	"github.com/caarlos0/env/v11"
	"github.com/moby/moby/api/types/network"
)

var (
	ErrWorkerCountNotPositive = errors.New("worker count not positive")
	ErrClientPortNotInExposed = errors.New("client port not in expose ports")
	ErrInvalidPortString      = errors.New("invalid port string")
	ErrTooFewPlayersPerRoom   = errors.New("too few players per room; must be atleast two")
)

type parsedConfig struct {
	Image          string   `env:"GAME_SERVER_IMAGE,required,notEmpty"`
	WorkerCount    int      `env:"GAME_SERVER_COUNT,required"`
	ExposePorts    []string `env:"GAME_SERVER_EXPOSE_PORTS,required,notEmpty"`
	ClientPort     string   `env:"GAME_SERVER_CLIENT_PORT,required,notEmpty"`
	BrokerURI      string   `env:"NATS_URI,required,notEmpty"`
	PublicHost     string   `env:"PUBLIC_HOST,required,notEmpty"`
	PlayersPerRoom int      `env:"PLAYERS_PER_ROOM,required"`
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

	return &internal.EnvConfig{
		Image:       config.Image,
		Workercount: config.WorkerCount,
		ExposePorts: exposePorts,
		ClientPort:  clientPort,
		BrokerURI:   config.BrokerURI,
		PublicHost:  config.PublicHost,
	}, nil
}

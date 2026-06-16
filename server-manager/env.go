package main

import (
	"errors"
	"fmt"
	"server-manager/internal"

	"github.com/caarlos0/env/v11"
	"github.com/moby/moby/api/types/network"
)

var ErrWorkerCountNotPositive = errors.New("worker count not positive")

type parsedConfig struct {
	Image       string   `env:"GAME_SERVER_IMAGE,required,notEmpty"`
	WorkerCount int      `env:"GAME_SERVER_COUNT,required"`
	ExposePorts []string `env:"GAME_SERVER_EXPOSE_PORTS,required,notEmpty"`
	BrokerURI   string   `env:"NATS_URI,required,notEmpty"`
}

func parsePorts(config *parsedConfig) (map[network.Port]struct{}, error) {
	ports := make(map[network.Port]struct{})

	for _, portString := range config.ExposePorts {
		port, err := network.ParsePort(portString)
		if err != nil {
			return nil, fmt.Errorf("invalid port: %w", err)
		}
		ports[port] = struct{}{}
	}

	return ports, nil
}

func ReadConfig() (*internal.EnvConfig, error) {
	config := &parsedConfig{}

	if err := env.Parse(config); err != nil {
		return nil, err
	}

	ports, err := parsePorts(config)
	if err != nil {
		return nil, err
	}

	if config.WorkerCount < 1 {
		return nil, ErrWorkerCountNotPositive
	}

	return &internal.EnvConfig{
		Image:       config.Image,
		ExposePorts: ports,
		BrokerURI:   config.BrokerURI,
		Workercount: config.WorkerCount,
	}, nil
}

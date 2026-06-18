package main

import (
	"errors"
	"fmt"
	"server-manager/internal"

	"github.com/caarlos0/env/v11"
	"github.com/moby/moby/api/types/network"
)

var (
	ErrWorkerCountNotPositive = errors.New("worker count not positive")
	ErrClientPortsNotSubset   = errors.New("client ports must be a subset of expose ports")
)

type parsedConfig struct {
	Image       string   `env:"GAME_SERVER_IMAGE,required,notEmpty"`
	WorkerCount int      `env:"GAME_SERVER_COUNT,required"`
	ExposePorts []string `env:"GAME_SERVER_EXPOSE_PORTS,required,notEmpty"`
	ClientPorts []string `env:"GAME_SERVER_CLIENT_PORTS,required,notEmpty"`
	BrokerURI   string   `env:"NATS_URI,required,notEmpty"`
	PublicHost  string   `env:"PUBLIC_HOST,required,notEmpty"`
}

func parsePorts(ports []string) (map[network.Port]struct{}, error) {
	parsedPorts := make(map[network.Port]struct{})

	for _, portString := range ports {
		port, err := network.ParsePort(portString)
		if err != nil {
			return nil, fmt.Errorf("invalid port: %w", err)
		}
		parsedPorts[port] = struct{}{}
	}

	return parsedPorts, nil
}

func isSubset[K comparable](sub, super map[K]struct{}) bool {
	for k := range sub {
		if _, ok := super[k]; !ok {
			return false
		}
	}
	return true
}

func ReadConfig() (*internal.EnvConfig, error) {
	config := &parsedConfig{}

	if err := env.Parse(config); err != nil {
		return nil, err
	}

	if config.WorkerCount < 1 {
		return nil, ErrWorkerCountNotPositive
	}

	exposePorts, err := parsePorts(config.ExposePorts)
	if err != nil {
		return nil, err
	}

	clientPorts, err := parsePorts(config.ClientPorts)
	if err != nil {
		return nil, err
	}

	if !isSubset(clientPorts, exposePorts) {
		return nil, ErrClientPortsNotSubset
	}

	return &internal.EnvConfig{
		Image:       config.Image,
		Workercount: config.WorkerCount,
		ExposePorts: exposePorts,
		ClientPorts: clientPorts,
		BrokerURI:   config.BrokerURI,
		PublicHost:  config.PublicHost,
	}, nil
}

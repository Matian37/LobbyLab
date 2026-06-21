package main

import (
	"errors"
	"fmt"
	"server-manager/internal"
	"strings"

	"github.com/caarlos0/env/v11"
	"github.com/moby/moby/api/types/network"
)

var (
	ErrWorkerCountNotPositive = errors.New("worker count not positive")
	ErrClientPortsNotSubset   = errors.New("client ports must be a subset of expose ports")
	ErrInvalidPortString      = errors.New("invalid port string")
	ErrPortNameMissing        = errors.New("port name missing")
)

type parsedConfig struct {
	Image       string   `env:"GAME_SERVER_IMAGE,required,notEmpty"`
	WorkerCount int      `env:"GAME_SERVER_COUNT,required"`
	ExposePorts []string `env:"GAME_SERVER_EXPOSE_PORTS,required,notEmpty"`
	ClientPorts []string `env:"GAME_SERVER_CLIENT_PORTS,required,notEmpty"`
	BrokerURI   string   `env:"NATS_URI,required,notEmpty"`
	PublicHost  string   `env:"PUBLIC_HOST,required,notEmpty"`
}

func parsePorts(ports []string) (network.PortSet, error) {
	parsedPorts := make(network.PortSet)

	for _, str := range ports {
		port, err := network.ParsePort(str)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrInvalidPortString, err)
		}
		parsedPorts[port] = struct{}{}
	}

	return parsedPorts, nil
}

func parseNamedPorts(ports []string) (internal.NamedPortSet, error) {
	parsedPorts := make(internal.NamedPortSet)

	for _, str := range ports {
		name, portString, found := strings.Cut(str, ":")
		if !found {
			return nil, fmt.Errorf("%w for port '%v'", ErrPortNameMissing, str)
		}
		port, err := network.ParsePort(portString)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrInvalidPortString, err)
		}
		parsedPorts[internal.NamedPort{Name: name, Port: port}] = struct{}{}
	}

	return parsedPorts, nil
}

func isSubset(sub internal.NamedPortSet, super network.PortSet) bool {
	for k := range sub {
		if _, ok := super[k.Port]; !ok {
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

	clientPorts, err := parseNamedPorts(config.ClientPorts)
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

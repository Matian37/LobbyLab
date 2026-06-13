package internal

import "github.com/moby/moby/api/types/network"

type EnvConfig struct {
	Image       string
	ExposePorts []network.Port
	BrokerURI   string
}

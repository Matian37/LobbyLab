package internal

import (
	"encoding/json"

	"github.com/moby/moby/api/types/network"
)

type EnvConfig struct {
	Image       string
	Workercount int
	ExposePorts map[network.Port]struct{}
	ClientPorts map[network.Port]struct{}
	BrokerURI   string
	PublicHost  string
}

type Result struct {
	Success bool            `json:"success"`
	Details json.RawMessage `json:"details"`
}

type ServerInfo struct {
	Host    string
	PortMap network.PortMap
}

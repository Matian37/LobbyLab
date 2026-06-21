package internal

import (
	"encoding/json"

	"github.com/moby/moby/api/types/network"
)

type EnvConfig struct {
	Image       string
	Workercount int
	ExposePorts network.PortSet
	ClientPorts NamedPortSet
	BrokerURI   string
	PublicHost  string
}

type Result struct {
	Success bool            `json:"success"`
	Details json.RawMessage `json:"details"`
}

type ServerEndpoints struct {
	Host  string       `json:"host"`
	Ports NamedPortMap `json:"portNames"`
}

type NamedPort struct {
	Name string
	Port network.Port
}

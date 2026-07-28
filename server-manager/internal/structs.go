package internal

import (
	"encoding/json"

	"github.com/moby/moby/api/types/network"
)

type EnvConfig struct {
	Image          string
	Workercount    int
	ExposePorts    network.PortSet
	ClientPort     network.Port
	BrokerURI      string
	PublicHost     string
	DatabaseURI    string
	PlayersPerRoom int

	// must not be exposed for production use
	TestMakeContainerDummy bool
}

type MatchConfig struct {
	MatchID int             `json:"matchID"`
	Config  json.RawMessage `json:"config"`
}

type Result struct {
	Success bool            `json:"success"`
	MatchID int             `json:"matchID"`
	Details json.RawMessage `json:"details"`
}

type User struct {
	Login          string `json:"login"`
	MatchAuthToken string `json:"matchAuthToken"`
}

type ServerInfo struct {
	Host, Port string
}

package internal

import (
	"encoding/json"

	"github.com/moby/moby/api/types/network"
)

type EnvConfig struct {
	Image       string
	ExposePorts []network.Port
	BrokerURI   string
	DatabaseURI string
	PublicHost  string

	// must not be exposed for production use
	TestMakeContainerDummy bool
}

type MatchConfig struct {
	MatchID int             `json:"matchID"`
	Config  json.RawMessage `json:"config"`
}

type User struct {
	Login string
}

type ServerInfo struct {
	Host, Port string
}

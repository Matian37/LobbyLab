package internal

import (
	"encoding/json"

	"github.com/moby/moby/api/types/network"
)

type EnvConfig struct {
	Image       string
	ExposePorts map[network.Port]struct{}
	BrokerURI   string
	Workercount int
}

type Result struct {
	Success bool            `json:"success"`
	Details json.RawMessage `json:"details"`
}

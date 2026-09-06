package internal

import (
	"encoding/json"
	"log/slog"

	"github.com/moby/moby/api/types/network"
)

// EnvConfig is the runtime configuration for server-manager.
type EnvConfig struct {
	Image              string
	Workercount        int
	ExposePorts        network.PortSet
	ClientPort         network.Port
	BrokerURI          string
	BrokerNetworkName  string
	PublicHost         string
	DatabaseURI        string
	PlayersPerRoom     int
	LogLevel           slog.Level
	GameServerLogLevel slog.Level

	// Must not be exposed for production use
	TestMakeContainerDummy bool
}

// MatchConfig is the configuration which is sent to a worker.
type MatchConfig struct {
	MatchID int             `json:"matchID"`
	Config  json.RawMessage `json:"config"`
}

// Result is the outcome of a finished match reported by a worker.
type Result struct {
	Success bool            `json:"success"`
	MatchID int             `json:"matchID"`
	Details json.RawMessage `json:"details"`
}

// User is a player participating in matchmaking.
type User struct {
	Login string `json:"login"`
	// Secret which is used to authenticate the player
	// when connecting to match.
	MatchAuthToken string `json:"matchAuthToken"`
}

// ServerInfo is the address of a game-server which is running a match.
type ServerInfo struct {
	Host, Port string
}

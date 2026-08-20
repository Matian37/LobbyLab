package internal

import (
	"encoding/json"
	"log/slog"
)

type Config struct {
	BrokerURI      string
	WorkerID       string
	GameServerArgs []string
	LogLevel       slog.Level
}

type Result struct {
	Success bool            `json:"success"`
	MatchID int             `json:"matchID"`
	Details json.RawMessage `json:"details"`
}

type MatchConfig struct {
	MatchID int             `json:"matchID"`
	Config  json.RawMessage `json:"config"`
}

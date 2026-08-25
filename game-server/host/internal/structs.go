package internal

import (
	"encoding/json"
	"log/slog"
)

// Configuration for the game-server
type Config struct {
	BrokerURI string
	WorkerID  string
	LogLevel  slog.Level

	// Command-line arguments used for running actual game server
	GameServerArgs []string
}

// Match result published to the results stream.
type Result struct {
	MatchID int `json:"matchID"`
	// Holds whether the match was completed or canceled
	Success bool `json:"success"`
	// Holds result data provided by actual game server.
	// On cancellation, this is an empty object
	Details json.RawMessage `json:"details"`
}

// Match configuration given on assignment
type MatchConfig struct {
	MatchID int `json:"matchID"`
	// Match configuration addressed to the actual game server
	Config json.RawMessage `json:"config"`
}

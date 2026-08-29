package internal

import "errors"

// Errors returned by the matchmaking and database operations.
var (
	ErrDBNotEnoughPlayers        = errors.New("not enough players to create a room")
	ErrDBWaitingUserDisconnected = errors.New("waiting user disconnected")
)

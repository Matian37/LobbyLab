package internal

import "errors"

var (
	ErrDBNotEnoughPlayers        = errors.New("not enough players to create a room")
	ErrDBWaitingUserDisconnected = errors.New("waiting user disconnected")
)

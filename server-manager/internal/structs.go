package internal

import (
	"encoding/json"

	"github.com/moby/moby/api/types/network"
)

type EnvConfig struct {
	Image       string
	Workercount int
	ExposePorts network.PortSet
	ClientPort  network.Port
	BrokerURI   string
	PublicHost  string
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

type ServerInfo struct {
	Host, Port string
}

type MatchConfig struct {
	Players []User
}

func NewMatchConfig(_players []User) *MatchConfig {
	return &MatchConfig{
		Players: _players,
	}
}

type User struct {
	Login string
}

func NewUser(_login string) *User {
	return &User{
		Login: _login,
	}
}

type ServerInfo struct {
	Host, Port string
}

func NewServerInfo(host string, port string) *ServerInfo {
	return &ServerInfo{
		Host: host,
		Port: port,
	}
}

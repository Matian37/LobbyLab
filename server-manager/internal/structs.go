package internal

import "github.com/moby/moby/api/types/network"

type EnvConfig struct {
	Image       string
	ExposePorts []network.Port
	BrokerURI   string
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

type Socket struct {
	Host, Port string
}

func NewSocket(host string, port string) *Socket {
	return &Socket{
		Host: host,
		Port: port,
	}
}

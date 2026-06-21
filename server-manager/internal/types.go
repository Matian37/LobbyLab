package internal

import "github.com/moby/moby/api/types/network"

type Responders map[string]struct{}

type NamedPortSet = map[NamedPort]struct{}
type NamedPortMap = map[string]network.Port

package algorithm

import (
	"CloudBalancer/internal/load_balancer/backend"
)

type Strategy interface {
	NextBackend(backends []*backend.Backend) (*backend.Backend, error)
	Name() string
}

func GetStrategy(name string) (Strategy, error) {
	switch name {
	case "RoundRobin":
		return NewRoundRobinStrategy(), nil
	default:
		return nil, backend.ErrUnknownStrategy(name)
	}
}

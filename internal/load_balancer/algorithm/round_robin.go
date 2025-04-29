package algorithm

import (
	"fmt"
	"sync"

	"CloudBalancer/internal/load_balancer/backend"
)

type RoundRobinStrategy struct {
	mtx     sync.Mutex
	current int
}

func NewRoundRobinStrategy() *RoundRobinStrategy {
	return &RoundRobinStrategy{
		current: 0,
	}
}

func (s *RoundRobinStrategy) NextBackend(backends []*backend.Backend) (*backend.Backend, error) {
	if len(backends) == 0 {
		return nil, fmt.Errorf("no backends available")
	}

	s.mtx.Lock()
	defer s.mtx.Unlock()

	start := s.current
	for {
		backendItem := backends[s.current]
		s.current = (s.current + 1) % len(backends)

		if backendItem.IsHealthy() {
			return backendItem, nil
		}
		if s.current == start {
			return nil, fmt.Errorf("no healthy backends available")
		}
	}
}

func (s *RoundRobinStrategy) Name() string {
	return "RoundRobin"
}

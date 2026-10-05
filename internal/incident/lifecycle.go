package incident

import "sync"

type RepeaterLifecycle struct {
	mu         sync.RWMutex
	active     map[string]uint64
	resolution map[string]uint64
}

func NewRepeaterLifecycle() *RepeaterLifecycle {
	return &RepeaterLifecycle{active: make(map[string]uint64), resolution: make(map[string]uint64)}
}

func (s *RepeaterLifecycle) OpenRepeaterIncident(repeaterID string, generation uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if generation >= s.active[repeaterID] {
		s.active[repeaterID] = generation
	}
	return nil
}

func (s *RepeaterLifecycle) ResolveRepeaterIncident(repeaterID string, generation uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if active := s.active[repeaterID]; active != 0 && generation >= active {
		delete(s.active, repeaterID)
		s.resolution[repeaterID] = generation
	}
	return nil
}

func (s *RepeaterLifecycle) Active(repeaterID string) (uint64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	generation, exists := s.active[repeaterID]
	return generation, exists
}

func (s *RepeaterLifecycle) LastResolution(repeaterID string) uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.resolution[repeaterID]
}

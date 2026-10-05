package landing

import (
	"errors"
	"sync"
	"time"
)

type StationState string

const (
	StationOnline   StationState = "online"
	StationDegraded StationState = "degraded"
	StationOffline  StationState = "offline"
)

type StationSnapshot struct {
	ID         string       `json:"id"`
	Name       string       `json:"name"`
	State      StationState `json:"state"`
	LastSeenAt time.Time    `json:"last_seen_at"`
	Sequence   uint64       `json:"sequence"`
	UpdatedAt  time.Time    `json:"updated_at"`
}

type Station struct {
	mu       sync.RWMutex
	snapshot StationSnapshot
}

func NewStation(id, name string, at time.Time) (*Station, error) {
	if id == "" || name == "" {
		return nil, errors.New("station identity is required")
	}
	return &Station{snapshot: StationSnapshot{
		ID: id, Name: name, State: StationOnline, LastSeenAt: at.UTC(), UpdatedAt: at.UTC(),
	}}, nil
}

func (s *Station) Heartbeat(sequence uint64, at time.Time) StationSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	if sequence > s.snapshot.Sequence {
		s.snapshot.Sequence = sequence
		s.snapshot.LastSeenAt = at.UTC()
		s.snapshot.State = StationOnline
		s.snapshot.UpdatedAt = at.UTC()
	}
	return s.snapshot
}

func (s *Station) SetState(state StationState, at time.Time) StationSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshot.State = state
	s.snapshot.UpdatedAt = at.UTC()
	return s.snapshot
}

func (s *Station) Snapshot() StationSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshot
}

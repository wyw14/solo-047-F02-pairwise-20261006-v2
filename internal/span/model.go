package span

import (
	"errors"
	"sync"
	"time"
)

type State string

const (
	StateNormal    State = "normal"
	StateSuspect   State = "suspect"
	StateCut       State = "cut"
	StateRepairing State = "repairing"
	StateProving   State = "proving"
	StateRecovered State = "recovered"
)

type Snapshot struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	LengthKM         float64   `json:"length_km"`
	State            State     `json:"state"`
	RepairGeneration uint64    `json:"repair_generation"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type Span struct {
	mu       sync.RWMutex
	snapshot Snapshot
}

func New(id, name string, lengthKM float64, at time.Time) (*Span, error) {
	if id == "" || name == "" || lengthKM <= 0 {
		return nil, errors.New("span identity and positive length are required")
	}
	return &Span{snapshot: Snapshot{
		ID: id, Name: name, LengthKM: lengthKM, State: StateNormal, UpdatedAt: at.UTC(),
	}}, nil
}

func (s *Span) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshot
}

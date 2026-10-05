package repeater

import (
	"fmt"
	"sync"
	"time"
)

type HealthState string

const (
	HealthNormal  HealthState = "healthy"
	HealthHot     HealthState = "hot"
	HealthTripped HealthState = "latched-trip"
)

type Snapshot struct {
	ID             string      `json:"id"`
	Health         HealthState `json:"health"`
	TemperatureC   float64     `json:"temperature_c"`
	LastSequence   uint64      `json:"last_sequence"`
	TripGeneration uint64      `json:"trip_generation"`
	StableStep     int         `json:"stable_step"`
	UpdatedAt      time.Time   `json:"updated_at"`
}

type Repeater struct {
	mu       sync.RWMutex
	snapshot Snapshot
}

func New(id string, at time.Time) (*Repeater, error) {
	if id == "" {
		return nil, fmt.Errorf("repeater ID is required")
	}
	return &Repeater{snapshot: Snapshot{ID: id, Health: HealthNormal, UpdatedAt: at.UTC()}}, nil
}

func (r *Repeater) Snapshot() Snapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.snapshot
}

func (r *Repeater) update(mutator func(*Snapshot)) Snapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	mutator(&r.snapshot)
	return r.snapshot
}

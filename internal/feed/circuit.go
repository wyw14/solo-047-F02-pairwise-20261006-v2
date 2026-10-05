package feed

import (
	"fmt"
	"sync"
	"time"
)

type State string

const (
	StateEnergized State = "energized"
	StateIsolating State = "isolating"
	StateIsolated  State = "isolated"
	StateGrounded  State = "grounded"
	StateReleased  State = "released"
)

type CircuitSnapshot struct {
	ID          string    `json:"id"`
	StationID   string    `json:"station_id"`
	State       State     `json:"state"`
	OperationID string    `json:"operation_id"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Circuit struct {
	mu       sync.RWMutex
	snapshot CircuitSnapshot
}

func NewCircuit(id, stationID string, at time.Time) (*Circuit, error) {
	if id == "" || stationID == "" {
		return nil, fmt.Errorf("circuit identity is required")
	}
	return &Circuit{snapshot: CircuitSnapshot{
		ID: id, StationID: stationID, State: StateEnergized, UpdatedAt: at.UTC(),
	}}, nil
}

func (c *Circuit) Snapshot() CircuitSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.snapshot
}

func (c *Circuit) update(state State, operationID string, at time.Time) CircuitSnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.snapshot.State = state
	c.snapshot.OperationID = operationID
	c.snapshot.UpdatedAt = at.UTC()
	return c.snapshot
}

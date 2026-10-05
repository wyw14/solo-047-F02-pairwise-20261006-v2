package incident

import (
	"sync"
	"time"
)

type State string

const (
	StateCollecting   State = "collecting"
	StateWaitingPeer  State = "waiting-peer"
	StateCableCut     State = "cable-cut"
	StateStationFault State = "station-fault"
	StateResolved     State = "resolved"
)

type Snapshot struct {
	ID            string    `json:"id"`
	SpanID        string    `json:"span_id"`
	State         State     `json:"state"`
	LocalComplete bool      `json:"local_complete"`
	PeerComplete  bool      `json:"peer_complete"`
	Deadline      time.Time `json:"deadline"`
	Reason        string    `json:"reason"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type Incident struct {
	mu       sync.RWMutex
	snapshot Snapshot
}

func New(id, spanID string, deadline, at time.Time) *Incident {
	return &Incident{snapshot: Snapshot{
		ID: id, SpanID: spanID, State: StateCollecting, Deadline: deadline.UTC(), UpdatedAt: at.UTC(),
	}}
}

func (i *Incident) Snapshot() Snapshot {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.snapshot
}

func (i *Incident) update(mutator func(*Snapshot)) Snapshot {
	i.mu.Lock()
	defer i.mu.Unlock()
	mutator(&i.snapshot)
	return i.snapshot
}

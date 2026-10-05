package otdr

import (
	"sync"
	"time"
)

type Evidence struct {
	IncidentID string    `json:"incident_id"`
	StationID  string    `json:"station_id"`
	TraceID    string    `json:"trace_id"`
	DistanceKM float64   `json:"distance_km"`
	ObservedAt time.Time `json:"observed_at"`
}

type EvidencePair struct {
	Local    Evidence `json:"local"`
	Peer     Evidence `json:"peer"`
	Complete bool     `json:"complete"`
}

type Correlator struct {
	mu       sync.RWMutex
	evidence map[string]map[string]Evidence
}

func NewCorrelator() *Correlator {
	return &Correlator{evidence: make(map[string]map[string]Evidence)}
}

func (c *Correlator) Add(evidence Evidence) EvidencePair {
	c.mu.Lock()
	defer c.mu.Unlock()
	byStation := c.evidence[evidence.IncidentID]
	if byStation == nil {
		byStation = make(map[string]Evidence)
		c.evidence[evidence.IncidentID] = byStation
	}
	byStation[evidence.StationID] = evidence
	result := EvidencePair{}
	for _, value := range byStation {
		if result.Local.StationID == "" {
			result.Local = value
		} else if value.StationID != result.Local.StationID {
			result.Peer = value
		}
	}
	result.Complete = result.Local.StationID != "" && result.Peer.StationID != ""
	return result
}

func (c *Correlator) Pair(incidentID string) EvidencePair {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := EvidencePair{}
	for _, value := range c.evidence[incidentID] {
		if result.Local.StationID == "" {
			result.Local = value
		} else if value.StationID != result.Local.StationID {
			result.Peer = value
		}
	}
	result.Complete = result.Local.StationID != "" && result.Peer.StationID != ""
	return result
}

package landing

import (
	"fmt"
	"time"

	"github.com/wyw14/cry-103/internal/store"
	"github.com/wyw14/cry-103/internal/telemetry"
)

type HealthService struct {
	evaluator  *telemetry.AgeEvaluator
	heartbeats *store.HeartbeatStore
}

func NewHealthService(evaluator *telemetry.AgeEvaluator, heartbeats *store.HeartbeatStore) *HealthService {
	return &HealthService{evaluator: evaluator, heartbeats: heartbeats}
}

func (s *HealthService) Record(station *Station, sequence uint64, at time.Time) (StationSnapshot, error) {
	snapshot := station.Heartbeat(sequence, at)
	if err := s.heartbeats.Record(snapshot.ID, snapshot.Sequence, snapshot.LastSeenAt); err != nil {
		return snapshot, fmt.Errorf("persist landing heartbeat: %w", err)
	}
	return snapshot, nil
}

func (s *HealthService) Recover(station *Station) (StationSnapshot, error) {
	stored, found, err := s.heartbeats.Load(station.Snapshot().ID)
	if err != nil {
		return station.Snapshot(), err
	}
	if !found {
		return station.Snapshot(), nil
	}
	return station.Heartbeat(stored.Sequence, stored.SeenAt), nil
}

func (s *HealthService) Evaluate(station *Station, at time.Time) StationSnapshot {
	snapshot := station.Snapshot()
	if s.evaluator.Healthy(snapshot.LastSeenAt) {
		return station.SetState(StationOnline, at)
	}
	return station.SetState(StationOffline, at)
}

package landing

import (
	"context"
	"fmt"
	"time"

	"github.com/wyw14/cry-103/internal/topology"
)

type TraceChunk struct {
	StationID  string        `json:"station_id"`
	Index      int           `json:"index"`
	Path       topology.Path `json:"path"`
	Generation uint64        `json:"generation"`
	DistanceKM float64       `json:"distance_km"`
	LossDB     float64       `json:"loss_db"`
}

type Scanner struct {
	topology *topology.Registry
	delay    time.Duration
}

func NewScanner(registry *topology.Registry, delay time.Duration) *Scanner {
	return &Scanner{topology: registry, delay: delay}
}

func (s *Scanner) ScanPinned(ctx context.Context, snapshot topology.Snapshot, stationID string, chunks int) ([]TraceChunk, error) {
	if chunks < 1 {
		return nil, fmt.Errorf("chunk count must be positive")
	}
	result := make([]TraceChunk, 0, chunks)
	for index := 0; index < chunks; index++ {
		if err := wait(ctx, s.delay); err != nil {
			return nil, err
		}
		result = append(result, synthesizeChunk(snapshot, stationID, index))
	}
	if !s.topology.Matches(snapshot.SpanID, snapshot.Generation) {
		return nil, fmt.Errorf("topology generation changed during pinned scan")
	}
	return result, nil
}

func synthesizeChunk(snapshot topology.Snapshot, stationID string, index int) TraceChunk {
	distance := 18.0 + float64(index)*3.5
	if snapshot.ActivePath == topology.PathProtection {
		distance += 42
	}
	return TraceChunk{
		StationID: stationID, Index: index, Path: snapshot.ActivePath,
		Generation: snapshot.Generation, DistanceKM: distance, LossDB: 2.5 + float64(index)/2,
	}
}

func wait(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

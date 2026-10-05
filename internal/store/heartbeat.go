package store

import "time"

type HeartbeatStore struct {
	checkpoints *Checkpoints
}

func NewHeartbeatStore(checkpoints *Checkpoints) *HeartbeatStore {
	return &HeartbeatStore{checkpoints: checkpoints}
}

func (s *HeartbeatStore) Record(stationID string, sequence uint64, seenAt time.Time) error {
	return s.checkpoints.SaveHeartbeat(HeartbeatSnapshot{
		StationID: stationID,
		SeenAt:    seenAt.UTC(),
		Sequence:  sequence,
	})
}

func (s *HeartbeatStore) Load(stationID string) (HeartbeatSnapshot, bool, error) {
	value, found, err := s.checkpoints.LoadHeartbeat(stationID)
	if err != nil || !found {
		return value, found, err
	}
	value.SeenAt = value.SeenAt.UTC()
	return value, true, nil
}

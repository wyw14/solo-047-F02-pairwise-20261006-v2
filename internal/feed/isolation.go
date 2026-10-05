package feed

import (
	"errors"
	"time"
)

type IsolationCommand struct {
	StationID   string    `json:"station_id"`
	CircuitID   string    `json:"circuit_id"`
	OperationID string    `json:"operation_id"`
	IssuedAt    time.Time `json:"issued_at"`
}

type IsolationService struct{}

func NewIsolationService() *IsolationService {
	return &IsolationService{}
}

func (s *IsolationService) Begin(circuit *Circuit, operationID string, at time.Time) (IsolationCommand, error) {
	if operationID == "" {
		return IsolationCommand{}, errors.New("isolation operation ID is required")
	}
	snapshot := circuit.Snapshot()
	if snapshot.State != StateEnergized && snapshot.State != StateReleased {
		return IsolationCommand{}, errors.New("circuit is not ready for isolation")
	}
	circuit.update(StateIsolating, operationID, at)
	return IsolationCommand{
		StationID: snapshot.StationID, CircuitID: snapshot.ID, OperationID: operationID, IssuedAt: at.UTC(),
	}, nil
}

func (s *IsolationService) Confirm(circuit *Circuit, stationID, circuitID, operationID string, matched bool, at time.Time) (CircuitSnapshot, error) {
	snapshot := circuit.Snapshot()
	if snapshot.StationID != stationID || snapshot.ID != circuitID || snapshot.OperationID != operationID {
		return snapshot, errors.New("isolation receipt scope does not match active operation")
	}
	if !matched {
		return snapshot, errors.New("matching isolated receipt not available")
	}
	return circuit.update(StateIsolated, operationID, at), nil
}

func (s *IsolationService) Ground(circuit *Circuit, operationID string, at time.Time) (CircuitSnapshot, error) {
	snapshot := circuit.Snapshot()
	if snapshot.State != StateIsolated || snapshot.OperationID != operationID {
		return snapshot, errors.New("circuit must be isolated by the current operation before grounding")
	}
	return circuit.update(StateGrounded, operationID, at), nil
}

func (s *IsolationService) Release(circuit *Circuit, operationID string, at time.Time) (CircuitSnapshot, error) {
	snapshot := circuit.Snapshot()
	if snapshot.State != StateGrounded || snapshot.OperationID != operationID {
		return snapshot, errors.New("grounded operation does not match release")
	}
	return circuit.update(StateReleased, operationID, at), nil
}

package span

import (
	"fmt"
	"time"
)

var transitions = map[State]map[State]bool{
	StateNormal:    {StateSuspect: true},
	StateSuspect:   {StateNormal: true, StateCut: true},
	StateCut:       {StateRepairing: true},
	StateRepairing: {StateProving: true},
	StateProving:   {StateRepairing: true, StateRecovered: true},
	StateRecovered: {StateNormal: true, StateSuspect: true},
}

func (s *Span) Transition(next State, at time.Time) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if next == s.snapshot.State {
		return s.snapshot, nil
	}
	if !transitions[s.snapshot.State][next] {
		return s.snapshot, fmt.Errorf("span %s cannot transition from %s to %s", s.snapshot.ID, s.snapshot.State, next)
	}
	if next == StateRepairing && s.snapshot.State == StateCut {
		s.snapshot.RepairGeneration++
	}
	s.snapshot.State = next
	s.snapshot.UpdatedAt = at.UTC()
	return s.snapshot, nil
}

func (s *Span) MarkCut(at time.Time) (Snapshot, error) {
	if _, err := s.Transition(StateSuspect, at); err != nil {
		return s.Snapshot(), err
	}
	return s.Transition(StateCut, at)
}

func (s *Span) StartRepair(at time.Time) (Snapshot, error) {
	return s.Transition(StateRepairing, at)
}

func (s *Span) BeginProof(at time.Time) (Snapshot, error) {
	return s.Transition(StateProving, at)
}

func (s *Span) RejectProof(at time.Time) (Snapshot, error) {
	return s.Transition(StateRepairing, at)
}

func (s *Span) Recover(at time.Time) (Snapshot, error) {
	return s.Transition(StateRecovered, at)
}

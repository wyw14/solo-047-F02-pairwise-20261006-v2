package otdr

import (
	"fmt"
	"sync"
	"time"
)

type Direction string

const (
	DirectionEastToWest Direction = "east-to-west"
	DirectionWestToEast Direction = "west-to-east"
)

type Proof struct {
	ID         string    `json:"id"`
	Direction  Direction `json:"direction"`
	Passed     bool      `json:"passed"`
	LossDB     float64   `json:"loss_db"`
	MeasuredAt time.Time `json:"measured_at"`
	Message    string    `json:"message"`
}

type ProofResult struct {
	Complete bool    `json:"complete"`
	Passed   bool    `json:"passed"`
	Proofs   []Proof `json:"proofs"`
}

type ProofSet struct {
	mu     sync.RWMutex
	proofs map[Direction]Proof
}

func NewProofSet() *ProofSet {
	return &ProofSet{proofs: make(map[Direction]Proof)}
}

func (s *ProofSet) Add(proof Proof) (ProofResult, error) {
	if proof.Direction != DirectionEastToWest && proof.Direction != DirectionWestToEast {
		return ProofResult{}, fmt.Errorf("unsupported proof direction %q", proof.Direction)
	}
	s.mu.Lock()
	s.proofs[proof.Direction] = proof
	s.mu.Unlock()
	return s.Result(), nil
}

func (s *ProofSet) Result() ProofResult {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := ProofResult{Passed: true}
	for _, direction := range []Direction{DirectionEastToWest, DirectionWestToEast} {
		proof, exists := s.proofs[direction]
		if !exists {
			result.Passed = false
			continue
		}
		result.Proofs = append(result.Proofs, proof)
		if !proof.Passed {
			result.Passed = false
		}
	}
	result.Complete = len(result.Proofs) == 2
	return result
}

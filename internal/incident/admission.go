package incident

import (
	"sync"
	"time"

	"github.com/wyw14/cry-103/internal/telemetry"
)

type Alarm struct {
	ID         string    `json:"id"`
	SpanID     string    `json:"span_id"`
	Kind       string    `json:"kind"`
	ObservedAt time.Time `json:"observed_at"`
}

type AdmissionService struct {
	mu       sync.RWMutex
	mutes    *telemetry.MuteManager
	accepted []Alarm
}

func NewAdmissionService(mutes *telemetry.MuteManager) *AdmissionService {
	return &AdmissionService{mutes: mutes}
}

func (s *AdmissionService) Accept(alarm Alarm) bool {
	if s.mutes.Muted(alarm.SpanID) {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accepted = append(s.accepted, alarm)
	return true
}

func (s *AdmissionService) Accepted(spanID string) []Alarm {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []Alarm
	for _, alarm := range s.accepted {
		if alarm.SpanID == spanID {
			result = append(result, alarm)
		}
	}
	return result
}

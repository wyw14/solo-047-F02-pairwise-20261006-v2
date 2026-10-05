package telemetry

import (
	"context"
	"sync"
	"time"
)

type MuteLease struct {
	ID        string    `json:"id"`
	SpanID    string    `json:"span_id"`
	ExpiresAt time.Time `json:"expires_at"`
}

type MuteManager struct {
	mu     sync.RWMutex
	leases map[string]MuteLease
	now    func() time.Time
}

func NewMuteManager(now func() time.Time) *MuteManager {
	return &MuteManager{leases: make(map[string]MuteLease), now: now}
}

func (m *MuteManager) Acquire(id, spanID string, ttl time.Duration) MuteLease {
	m.mu.Lock()
	defer m.mu.Unlock()
	lease := MuteLease{ID: id, SpanID: spanID, ExpiresAt: m.now().Add(ttl).UTC()}
	m.leases[id] = lease
	return lease
}

func (m *MuteManager) Renew(id string, ttl time.Duration) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	lease, exists := m.leases[id]
	if !exists {
		return false
	}
	lease.ExpiresAt = m.now().Add(ttl).UTC()
	m.leases[id] = lease
	return true
}

func (m *MuteManager) Release(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.leases, id)
}

func (m *MuteManager) Muted(spanID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	now := m.now()
	for _, lease := range m.leases {
		if lease.SpanID == spanID && now.Before(lease.ExpiresAt) {
			return true
		}
	}
	return false
}

func (m *MuteManager) Run(ctx context.Context, id string, ttl time.Duration) {
	interval := ttl / 3
	if interval <= 0 {
		interval = time.Millisecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	defer m.Release(id)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !m.Renew(id, ttl) {
				return
			}
		}
	}
}

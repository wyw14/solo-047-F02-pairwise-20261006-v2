package topology

import (
	"errors"
	"sync"
	"time"
)

type Path string

const (
	PathPrimary    Path = "primary"
	PathProtection Path = "protection"
)

type Snapshot struct {
	SpanID      string    `json:"span_id"`
	Generation  uint64    `json:"generation"`
	ActivePath  Path      `json:"active_path"`
	ChangedAt   time.Time `json:"changed_at"`
	PrimaryOpen bool      `json:"primary_open"`
	ProtectOpen bool      `json:"protection_open"`
}

func (s Snapshot) Validate() error {
	if s.SpanID == "" || s.Generation == 0 {
		return errors.New("topology span and generation are required")
	}
	if s.ActivePath != PathPrimary && s.ActivePath != PathProtection {
		return errors.New("active path is invalid")
	}
	return nil
}

type Registry struct {
	mu    sync.RWMutex
	spans map[string]Snapshot
}

func NewRegistry() *Registry {
	return &Registry{spans: make(map[string]Snapshot)}
}

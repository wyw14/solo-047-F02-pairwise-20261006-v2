package topology

import (
	"fmt"
	"time"
)

func (r *Registry) Switch(spanID string, target Path, at time.Time) (Snapshot, error) {
	if target != PathPrimary && target != PathProtection {
		return Snapshot{}, fmt.Errorf("unsupported target path %q", target)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	snapshot, exists := r.spans[spanID]
	if !exists {
		return Snapshot{}, fmt.Errorf("topology for span %s not found", spanID)
	}
	if snapshot.ActivePath == target {
		return snapshot, nil
	}
	snapshot.Generation++
	snapshot.ActivePath = target
	snapshot.ChangedAt = at.UTC()
	snapshot.PrimaryOpen = target != PathPrimary
	snapshot.ProtectOpen = target != PathProtection
	r.spans[spanID] = snapshot
	return snapshot, nil
}

func (r *Registry) Open(spanID string, path Path, at time.Time) (Snapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	snapshot, exists := r.spans[spanID]
	if !exists {
		return Snapshot{}, fmt.Errorf("topology for span %s not found", spanID)
	}
	if path == PathPrimary {
		snapshot.PrimaryOpen = true
	} else if path == PathProtection {
		snapshot.ProtectOpen = true
	} else {
		return Snapshot{}, fmt.Errorf("unsupported path %q", path)
	}
	snapshot.Generation++
	snapshot.ChangedAt = at.UTC()
	r.spans[spanID] = snapshot
	return snapshot, nil
}

func (r *Registry) Connect(spanID string, path Path, at time.Time) (Snapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	snapshot, exists := r.spans[spanID]
	if !exists {
		return Snapshot{}, fmt.Errorf("topology for span %s not found", spanID)
	}
	if path == PathPrimary {
		if !snapshot.ProtectOpen {
			return Snapshot{}, fmt.Errorf("protection path must be open before primary connects")
		}
		snapshot.PrimaryOpen = false
	} else if path == PathProtection {
		if !snapshot.PrimaryOpen {
			return Snapshot{}, fmt.Errorf("primary path must be open before protection connects")
		}
		snapshot.ProtectOpen = false
	} else {
		return Snapshot{}, fmt.Errorf("unsupported path %q", path)
	}
	snapshot.ActivePath = path
	snapshot.Generation++
	snapshot.ChangedAt = at.UTC()
	r.spans[spanID] = snapshot
	return snapshot, nil
}

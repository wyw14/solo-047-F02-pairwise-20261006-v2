package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type SnapshotStore struct {
	mu   sync.RWMutex
	dir  string
	perm os.FileMode
}

func NewSnapshotStore(dir string) *SnapshotStore {
	return &SnapshotStore{dir: dir, perm: 0o600}
}

func (s *SnapshotStore) Save(name string, value any) error {
	if name == "" {
		return errors.New("snapshot name is required")
	}
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode snapshot %s: %w", name, err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return fmt.Errorf("create snapshot directory: %w", err)
	}
	target := filepath.Join(s.dir, name+".json")
	temporary := target + ".next"
	if err := os.WriteFile(temporary, append(data, '\n'), s.perm); err != nil {
		return fmt.Errorf("write snapshot %s: %w", name, err)
	}
	if err := os.Rename(temporary, target); err != nil {
		return fmt.Errorf("publish snapshot %s: %w", name, err)
	}
	return nil
}

func (s *SnapshotStore) Load(name string, target any) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, err := os.ReadFile(filepath.Join(s.dir, name+".json"))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read snapshot %s: %w", name, err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		return false, fmt.Errorf("decode snapshot %s: %w", name, err)
	}
	return true, nil
}

func (s *SnapshotStore) Remove(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := os.Remove(filepath.Join(s.dir, name+".json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

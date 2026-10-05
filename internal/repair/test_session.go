package repair

import (
	"context"
	"sync"
	"time"

	"github.com/wyw14/cry-103/internal/platform"
	"github.com/wyw14/cry-103/internal/telemetry"
)

type TestState string

const (
	TestRunning   TestState = "running"
	TestCompleted TestState = "completed"
	TestCanceled  TestState = "canceled"
)

type TestSnapshot struct {
	ID        string    `json:"id"`
	SpanID    string    `json:"span_id"`
	State     TestState `json:"state"`
	MuteID    string    `json:"mute_id"`
	UpdatedAt time.Time `json:"updated_at"`
}

type TestSession struct {
	mu     sync.RWMutex
	state  TestSnapshot
	cancel context.CancelFunc
	done   chan struct{}
}

type TestService struct {
	mutes *telemetry.MuteManager
	ids   platform.IDGenerator
	now   func() time.Time
	ttl   time.Duration
}

func NewTestService(mutes *telemetry.MuteManager, ids platform.IDGenerator, now func() time.Time, ttl time.Duration) *TestService {
	return &TestService{mutes: mutes, ids: ids, now: now, ttl: ttl}
}

func (s *TestService) Start(parent context.Context, spanID string) *TestSession {
	ctx, cancel := context.WithCancel(parent)
	muteID := s.ids.New("mute")
	s.mutes.Acquire(muteID, spanID, s.ttl)
	session := &TestSession{
		state:  TestSnapshot{ID: s.ids.New("wet-test"), SpanID: spanID, State: TestRunning, MuteID: muteID, UpdatedAt: s.now().UTC()},
		cancel: cancel, done: make(chan struct{}),
	}
	go func() {
		defer close(session.done)
		s.mutes.Run(ctx, muteID, s.ttl)
	}()
	return session
}

func (s *TestService) Cancel(session *TestSession) TestSnapshot {
	session.cancel()
	<-session.done
	session.mu.Lock()
	defer session.mu.Unlock()
	session.state.State = TestCanceled
	session.state.UpdatedAt = s.now().UTC()
	return session.state
}

func (s *TestService) Complete(session *TestSession) TestSnapshot {
	session.cancel()
	<-session.done
	session.mu.Lock()
	defer session.mu.Unlock()
	session.state.State = TestCompleted
	session.state.UpdatedAt = s.now().UTC()
	return session.state
}

func (s *TestService) Snapshot(session *TestSession) TestSnapshot {
	session.mu.RLock()
	defer session.mu.RUnlock()
	return session.state
}

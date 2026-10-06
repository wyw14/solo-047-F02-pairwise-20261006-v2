package repeater

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/wyw14/cry-103/internal/feed"
	"github.com/wyw14/cry-103/internal/store"
	"github.com/wyw14/cry-103/internal/telemetry"
)

func newTestRamp(t *testing.T, now func() time.Time, rule telemetry.StabilityRule) (*RampController, *Repeater, *telemetry.Repository) {
	t.Helper()
	repository := telemetry.NewRepository()
	policy, err := telemetry.NewStabilityPolicy(now, rule)
	if err != nil {
		t.Fatal(err)
	}
	checkpoints := store.NewCheckpoints(store.NewSnapshotStore(filepath.Join(t.TempDir(), "snapshots")))
	controller := NewRampController(feed.NewActuator(), repository, policy, checkpoints, now)
	target, err := New("R07", now())
	if err != nil {
		t.Fatal(err)
	}
	return controller, target, repository
}

func TestRampStepFailsWithConcreteReasonWhenSamplesMissing(t *testing.T) {
	base := time.Date(2026, 10, 5, 10, 0, 0, 0, time.UTC)
	now := func() time.Time { return base }
	controller, target, _ := newTestRamp(t, now, telemetry.StabilityRule{
		MinimumSamples: 2, MinimumPeriod: time.Second, MaximumSpread: 1.5, MaximumAge: 5 * time.Second,
	})

	_, err := controller.ApplyStep(context.Background(), target, RampStep{Number: 1, Voltage: 100}, 0)
	if err == nil {
		t.Fatal("ramp step without current samples should fail")
	}
	var stabilityErr *telemetry.StabilityError
	if !errors.As(err, &stabilityErr) {
		t.Fatalf("error should wrap telemetry.StabilityError, got %v", err)
	}
	if len(stabilityErr.Result.Reasons) != 1 || stabilityErr.Result.Reasons[0].Code != telemetry.ReasonMissing {
		t.Fatalf("expected missing reason, got %+v", stabilityErr.Result.Reasons)
	}
	if stabilityErr.Result.Scope.AssetID != "R07" || stabilityErr.Result.Scope.Kind != telemetry.KindCurrent {
		t.Fatalf("reason should carry the repeater/current scope, got %+v", stabilityErr.Result.Scope)
	}
	if target.Snapshot().StableStep != 0 {
		t.Fatal("unstable step must not be recorded")
	}
}

func TestRampStepSucceedsWhenScopedWindowStable(t *testing.T) {
	base := time.Date(2026, 10, 5, 10, 0, 0, 0, time.UTC)
	now := func() time.Time { return base.Add(2 * time.Second) }
	controller, target, repository := newTestRamp(t, now, telemetry.StabilityRule{
		MinimumSamples: 2, MinimumPeriod: time.Second, MaximumSpread: 1.5, MaximumAge: 5 * time.Second,
	})
	for index, value := range []float64{10, 10.5} {
		sample := telemetry.Sample{
			ID: "sample", AssetID: "R07", Kind: telemetry.KindCurrent, Value: value,
			Sequence: uint64(index + 1), Generation: 7, ObservedAt: base.Add(time.Duration(index) * time.Second),
		}
		if err := repository.Add(sample); err != nil {
			t.Fatal(err)
		}
	}

	snapshot, err := controller.ApplyStep(context.Background(), target, RampStep{Number: 3, Voltage: 100}, 0)
	if err != nil {
		t.Fatalf("stable window should allow ramp step: %v", err)
	}
	if snapshot.StableStep != 3 {
		t.Fatalf("stable step should be recorded, got %d", snapshot.StableStep)
	}
}

func TestRampStepRejectsStaleSamplesFromOldGeneration(t *testing.T) {
	base := time.Date(2026, 10, 5, 10, 0, 0, 0, time.UTC)
	clock := base.Add(30 * time.Second)
	controller, target, repository := newTestRamp(t, func() time.Time { return clock }, telemetry.StabilityRule{
		MinimumSamples: 2, MinimumPeriod: time.Second, MaximumSpread: 1.5, MaximumAge: 5 * time.Second,
	})
	// Fresh samples belong to the old generation; a newer generation exists
	// with no samples yet, so the latest window must be missing.
	for index, value := range []float64{10, 10.5} {
		sample := telemetry.Sample{
			ID: "sample", AssetID: "R07", Kind: telemetry.KindCurrent, Value: value,
			Sequence: uint64(index + 1), Generation: 6, ObservedAt: clock.Add(-time.Second),
		}
		if err := repository.Add(sample); err != nil {
			t.Fatal(err)
		}
	}
	if err := repository.Add(telemetry.Sample{
		ID: "marker", AssetID: "R07", Kind: telemetry.KindTemperature, Value: 40,
		Sequence: 3, Generation: 7, ObservedAt: clock,
	}); err != nil {
		t.Fatal(err)
	}

	_, err := controller.ApplyStep(context.Background(), target, RampStep{Number: 1, Voltage: 100}, 0)
	var stabilityErr *telemetry.StabilityError
	if !errors.As(err, &stabilityErr) {
		t.Fatalf("expected stability error for generation 7, got %v", err)
	}
	if stabilityErr.Result.Scope.Generation != 7 {
		t.Fatalf("scope should pin to newest generation 7, got %d", stabilityErr.Result.Scope.Generation)
	}
	if stabilityErr.Result.Reasons[0].Code != telemetry.ReasonMissing {
		t.Fatalf("expected missing current samples for gen 7, got %+v", stabilityErr.Result.Reasons)
	}
}

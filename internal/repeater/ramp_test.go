package repeater

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/wyw14/cry-103/internal/feed"
	"github.com/wyw14/cry-103/internal/store"
	"github.com/wyw14/cry-103/internal/telemetry"
)

func TestRampControllerReportsStabilityReasons(t *testing.T) {
	base := time.Now().UTC()
	repository := telemetry.NewRepository()
	checkpoints := store.NewCheckpoints(store.NewSnapshotStore(t.TempDir()))
	controller := NewRampController(feed.NewActuator(), repository,
		telemetry.StabilityWindow{MinimumSamples: 2, MaximumSpread: 1.5, MinimumPeriod: time.Second, MaximumAge: 5 * time.Second},
		checkpoints, func() time.Time { return base.Add(time.Minute) })
	target, err := New("R07", base)
	if err != nil {
		t.Fatal(err)
	}
	for index, value := range []float64{10, 10.2} {
		if err := repository.Add(telemetry.Sample{
			ID: "sample", AssetID: "R07", Kind: telemetry.KindCurrent, Value: value,
			Sequence: uint64(index + 1), Generation: 1, ObservedAt: base.Add(time.Duration(index) * time.Second),
		}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := controller.ApplyStep(context.Background(), target, RampStep{Number: 1, Voltage: 12}, 0); err == nil ||
		!strings.Contains(err.Error(), "stale samples: newest is 59s old, maximum age is 5s") {
		t.Fatalf("expected stale reason in error, got %v", err)
	}
}

package telemetry

import (
	"context"
	"testing"
	"time"
)

func TestRepositoryOrdersSamplesBySequence(t *testing.T) {
	repository := NewRepository()
	now := time.Now().UTC()
	for _, sequence := range []uint64{3, 1, 2} {
		if err := repository.Add(Sample{
			ID: "sample", AssetID: "R07", Kind: KindCurrent, Value: float64(sequence),
			Sequence: sequence, ObservedAt: now.Add(time.Duration(sequence) * time.Second),
		}); err != nil {
			t.Fatal(err)
		}
	}
	history := repository.History("R07")
	if len(history) != 3 || history[0].Sequence != 1 || history[2].Sequence != 3 {
		t.Fatalf("samples not ordered by sequence: %+v", history)
	}
}

func TestMuteManagerReleasesOnContextCancellation(t *testing.T) {
	manager := NewMuteManager(time.Now)
	manager.Acquire("mute-1", "S3", time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		manager.Run(ctx, "mute-1", time.Second)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("mute renewal did not stop")
	}
	if manager.Muted("S3") {
		t.Fatal("mute remained active after cancellation")
	}
}

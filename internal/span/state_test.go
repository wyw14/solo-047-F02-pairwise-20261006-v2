package span

import (
	"testing"
	"time"
)

func TestRepairLifecycle(t *testing.T) {
	now := time.Date(2026, 8, 24, 2, 0, 0, 0, time.UTC)
	target, err := New("S3", "South China Sea S3", 126.4, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := target.MarkCut(now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := target.StartRepair(now.Add(2 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := target.BeginProof(now.Add(3 * time.Minute)); err != nil {
		t.Fatal(err)
	}
	snapshot, err := target.Recover(now.Add(4 * time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.State != StateRecovered || snapshot.RepairGeneration != 1 {
		t.Fatalf("unexpected recovered snapshot: %+v", snapshot)
	}
}

func TestInvalidSpanTransitionIsRejected(t *testing.T) {
	target, err := New("S4", "South China Sea S4", 95, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := target.Recover(time.Now()); err == nil {
		t.Fatal("normal span must not transition directly to recovered")
	}
}

package topology

import (
	"testing"
	"time"
)

func TestCrossConnectUsesBreakBeforeMake(t *testing.T) {
	registry := NewRegistry()
	if _, err := registry.Register("S3", PathPrimary, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Connect("S3", PathProtection, time.Now()); err == nil {
		t.Fatal("protection must not connect while primary remains connected")
	}
	if _, err := registry.Open("S3", PathPrimary, time.Now()); err != nil {
		t.Fatal(err)
	}
	snapshot, err := registry.Connect("S3", PathProtection, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.ActivePath != PathProtection || !snapshot.PrimaryOpen || snapshot.ProtectOpen {
		t.Fatalf("unexpected topology snapshot: %+v", snapshot)
	}
}

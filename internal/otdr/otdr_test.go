package otdr

import (
	"testing"
	"time"
)

func TestProofSetWaitsForConfiguredDirections(t *testing.T) {
	set := NewProofSet()
	result, err := set.Add(Proof{ID: "east", Direction: DirectionEastToWest, Passed: true, MeasuredAt: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if result.Complete {
		t.Fatal("one proof must leave the set incomplete")
	}
	result, err = set.Add(Proof{ID: "west", Direction: DirectionWestToEast, Passed: true, MeasuredAt: time.Now()})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Complete || !result.Passed {
		t.Fatalf("unexpected proof result: %+v", result)
	}
}

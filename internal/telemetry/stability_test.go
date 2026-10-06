package telemetry

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func stabilitySample(asset string, kind Kind, sequence uint64, generation uint64, value float64, at time.Time) Sample {
	return Sample{
		ID: "sample", AssetID: asset, Kind: kind, Value: value, Unit: "A",
		Sequence: sequence, Generation: generation, ObservedAt: at,
	}
}

func TestStabilityPolicyPassesWhenAllCriteriaMet(t *testing.T) {
	base := time.Date(2026, 10, 5, 10, 0, 0, 0, time.UTC)
	now := base.Add(2 * time.Second)
	policy, err := NewStabilityPolicy(func() time.Time { return now }, StabilityRule{
		MinimumSamples: 2, MinimumPeriod: time.Second, MaximumSpread: 1.5, MaximumAge: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	repository := NewRepository()
	for index, value := range []float64{10, 10.5} {
		if err := repository.Add(stabilitySample("R07", KindCurrent, uint64(index+1), 3, value, base.Add(time.Duration(index)*time.Second))); err != nil {
			t.Fatal(err)
		}
	}
	result, err := policy.Evaluate(repository, StabilityScope{AssetID: "R07", Kind: KindCurrent}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Stable || len(result.Reasons) != 0 {
		t.Fatalf("expected stable result, got %+v", result)
	}
	if result.Scope.Generation != 3 {
		t.Fatalf("scope should resolve to newest generation, got %d", result.Scope.Generation)
	}
	if result.Metrics.Samples != 2 || result.Metrics.LastSeq != 2 {
		t.Fatalf("unexpected metrics: %+v", result.Metrics)
	}
}

func TestStabilityPolicyReportsMissingWhenNoSamples(t *testing.T) {
	now := time.Date(2026, 10, 5, 10, 0, 0, 0, time.UTC)
	policy, _ := NewStabilityPolicy(func() time.Time { return now }, StabilityRule{MinimumSamples: 2, MaximumSpread: 1})
	result, err := policy.Evaluate(NewRepository(), StabilityScope{AssetID: "R07", Kind: KindCurrent}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if result.Stable || len(result.Reasons) != 1 || result.Reasons[0].Code != ReasonMissing {
		t.Fatalf("expected single missing reason, got %+v", result.Reasons)
	}
}

func TestStabilityPolicyReportsConcreteReasons(t *testing.T) {
	base := time.Date(2026, 10, 5, 10, 0, 0, 0, time.UTC)
	clock := base.Add(30 * time.Second)
	policy, _ := NewStabilityPolicy(func() time.Time { return clock }, StabilityRule{
		MinimumSamples: 3, MinimumPeriod: 10 * time.Second, MaximumSpread: 0.1, MaximumAge: 5 * time.Second,
	})
	repository := NewRepository()
	// Two samples (need three), 1s apart (need 10s), spread 2 (limit 0.1),
	// last one 29s old (limit 5s): every criterion fails at once.
	for index, value := range []float64{10.0, 12.0} {
		at := base.Add(time.Duration(index) * time.Second)
		if err := repository.Add(stabilitySample("R07", KindCurrent, uint64(index+1), 1, value, at)); err != nil {
			t.Fatal(err)
		}
	}
	result, err := policy.Evaluate(repository, StabilityScope{AssetID: "R07", Kind: KindCurrent, Generation: 1}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if result.Stable {
		t.Fatal("expected unstable result")
	}
	codes := map[ReasonCode]bool{}
	for _, reason := range result.Reasons {
		codes[reason.Code] = true
		if reason.Scope.AssetID != "R07" {
			t.Fatalf("reason missing scope: %+v", reason)
		}
		if reason.Message == "" {
			t.Fatalf("reason %s has no message", reason.Code)
		}
	}
	for _, expected := range []ReasonCode{ReasonInsufficientSamples, ReasonPeriodTooShort, ReasonUnstableSpread, ReasonStale} {
		if !codes[expected] {
			t.Fatalf("missing reason %s, got %+v", expected, result.Reasons)
		}
	}
	stale := result.Reasons[0]
	for _, reason := range result.Reasons {
		if reason.Code == ReasonStale {
			stale = reason
		}
	}
	if stale.Age != 29*time.Second || stale.LimitAge != 5*time.Second {
		t.Fatalf("stale reason should carry age details, got %+v", stale)
	}

	stabilityErr := &StabilityError{Result: result}
	message := stabilityErr.Error()
	if !strings.Contains(message, "insufficient_samples") || !strings.Contains(message, "stale") {
		t.Fatalf("error message should list reasons: %s", message)
	}
	var failure StabilityFailure
	if !errors.As(stabilityErr, &failure) {
		t.Fatal("errors.As should expose a StabilityFailure")
	}
}

func TestStabilityPolicyResolvesRulesBySpecificity(t *testing.T) {
	now := time.Date(2026, 10, 5, 10, 0, 0, 0, time.UTC)
	policy, _ := NewStabilityPolicy(func() time.Time { return now }, StabilityRule{MinimumSamples: 2, MaximumSpread: 1})

	kindRule := StabilityRule{MinimumSamples: 4, MaximumSpread: 1}
	assetRule := StabilityRule{MinimumSamples: 6, MaximumSpread: 1}
	exactRule := StabilityRule{MinimumSamples: 8, MaximumSpread: 1}
	if err := policy.SetRule(StabilityScope{Kind: KindTemperature}, kindRule); err != nil {
		t.Fatal(err)
	}
	if err := policy.SetRule(StabilityScope{AssetID: "R07", Kind: KindCurrent}, assetRule); err != nil {
		t.Fatal(err)
	}
	if err := policy.SetRule(StabilityScope{AssetID: "R07", Kind: KindCurrent, Generation: 9}, exactRule); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		scope StabilityScope
		want  int
	}{
		{StabilityScope{AssetID: "R09", Kind: KindOpticalPower}, 2},           // default
		{StabilityScope{AssetID: "R09", Kind: KindTemperature}, 4},            // kind-only
		{StabilityScope{AssetID: "R07", Kind: KindCurrent, Generation: 3}, 6}, // asset+kind
		{StabilityScope{AssetID: "R07", Kind: KindCurrent, Generation: 9}, 8}, // exact
	}
	for _, tc := range cases {
		if got := policy.Rule(tc.scope).MinimumSamples; got != tc.want {
			t.Fatalf("Rule(%s) samples = %d, want %d", tc.scope, got, tc.want)
		}
	}

	if err := policy.SetRule(StabilityScope{AssetID: "R07"}, StabilityRule{MinimumSamples: -1}); err == nil {
		t.Fatal("invalid override rule should be rejected")
	}
}

func TestWindowSamplesFiltersByGenerationAndCursor(t *testing.T) {
	base := time.Date(2026, 10, 5, 10, 0, 0, 0, time.UTC)
	repository := NewRepository()
	samples := []Sample{
		stabilitySample("R07", KindCurrent, 1, 1, 5, base),
		stabilitySample("R07", KindCurrent, 2, 1, 5, base.Add(time.Second)),
		stabilitySample("R07", KindCurrent, 3, 2, 5, base.Add(2*time.Second)),
		stabilitySample("R07", KindTemperature, 4, 2, 40, base.Add(3*time.Second)),
		stabilitySample("R08", KindCurrent, 5, 2, 5, base.Add(4*time.Second)),
	}
	for _, sample := range samples {
		if err := repository.Add(sample); err != nil {
			t.Fatal(err)
		}
	}

	latest := WindowSamples(repository, StabilityScope{AssetID: "R07", Kind: KindCurrent}, 0)
	if len(latest) != 1 || latest[0].Sequence != 3 {
		t.Fatalf("generation 0 should select newest generation only, got %+v", latest)
	}

	pinned := WindowSamples(repository, StabilityScope{AssetID: "R07", Kind: KindCurrent, Generation: 1}, 1)
	if len(pinned) != 1 || pinned[0].Sequence != 2 {
		t.Fatalf("pinned generation with cursor should return seq 2, got %+v", pinned)
	}

	legacy := CurrentSamples(repository, "R07", 0)
	if len(legacy) != 1 || legacy[0].Generation != 2 {
		t.Fatalf("CurrentSamples should delegate to newest generation window, got %+v", legacy)
	}
}

func TestWindowSamplesAcceptsLegacySamplesWithoutGeneration(t *testing.T) {
	base := time.Date(2026, 10, 5, 10, 0, 0, 0, time.UTC)
	repository := NewRepository()
	for index := 0; index < 2; index++ {
		if err := repository.Add(stabilitySample("R07", KindCurrent, uint64(index+1), 0, 5, base.Add(time.Duration(index)*time.Second))); err != nil {
			t.Fatal(err)
		}
	}
	legacy := WindowSamples(repository, StabilityScope{AssetID: "R07", Kind: KindCurrent}, 0)
	if len(legacy) != 2 {
		t.Fatalf("samples without generation should not be filtered out, got %+v", legacy)
	}
	now := base.Add(2 * time.Second)
	policy, _ := NewStabilityPolicy(func() time.Time { return now }, StabilityRule{
		MinimumSamples: 2, MinimumPeriod: time.Second, MaximumSpread: 1.5,
	})
	result, err := policy.Evaluate(repository, StabilityScope{AssetID: "R07", Kind: KindCurrent}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Stable {
		t.Fatalf("legacy samples should evaluate stably, got reasons %+v", result.Reasons)
	}
}

func TestUniformStabilityWindowKeepsLegacyBehavior(t *testing.T) {
	base := time.Date(2026, 10, 5, 10, 0, 0, 0, time.UTC)
	window := StabilityWindow{MinimumSamples: 2, MaximumSpread: 1.5, MinimumPeriod: time.Second}

	if result, err := window.Evaluate(nil); err != nil || result.Stable || result.Samples != 0 {
		t.Fatalf("empty window should return zero result, got %+v err=%v", result, err)
	}
	one, err := window.Evaluate([]Sample{stabilitySample("R07", KindCurrent, 1, 1, 1, base)})
	if err != nil || one.Stable || one.Samples != 1 {
		t.Fatalf("single sample should be unstable, not rejected: %+v err=%v", one, err)
	}
	if _, err := (StabilityWindow{MinimumSamples: 1}).Evaluate(nil); err == nil {
		t.Fatal("invalid window should be rejected")
	}

	samples := []Sample{
		stabilitySample("R07", KindCurrent, 1, 1, 10, base),
		stabilitySample("R07", KindCurrent, 2, 1, 11, base.Add(time.Second)),
	}
	result, err := window.Evaluate(samples)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Stable || result.Spread != 1 || result.Period != time.Second || result.LastSeq != 2 {
		t.Fatalf("unexpected uniform result: %+v", result)
	}
}

func TestStabilityPolicyValidatesScopeAndClock(t *testing.T) {
	if _, err := NewStabilityPolicy(nil, StabilityRule{MinimumSamples: 2}); err == nil {
		t.Fatal("policy without a clock should be rejected")
	}
	now := time.Now
	policy, _ := NewStabilityPolicy(now, StabilityRule{MinimumSamples: 2})
	if _, err := policy.Evaluate(NewRepository(), StabilityScope{AssetID: "R07"}, 0); err == nil {
		t.Fatal("evaluation without kind should be rejected")
	}
}

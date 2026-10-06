package telemetry

import (
	"strings"
	"testing"
	"time"
)

func currentSamples(base time.Time, generation uint64, values ...float64) []Sample {
	samples := make([]Sample, 0, len(values))
	for index, value := range values {
		samples = append(samples, Sample{
			ID: "sample", AssetID: "R07", Kind: KindCurrent, Value: value,
			Sequence: uint64(index + 1), Generation: generation,
			ObservedAt: base.Add(time.Duration(index) * time.Second),
		})
	}
	return samples
}

func hasReason(result StabilityResult, code StabilityIssue) bool {
	for _, reason := range result.Reasons {
		if reason.Code == code {
			return true
		}
	}
	return false
}

func TestStabilityWindowStable(t *testing.T) {
	base := time.Now().UTC()
	window := StabilityWindow{MinimumSamples: 2, MaximumSpread: 1.5, MinimumPeriod: time.Second, MaximumAge: 5 * time.Second}
	result, err := window.Evaluate(currentSamples(base, 1, 10, 10.5, 10.2), base.Add(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if !result.Stable || len(result.Reasons) != 0 {
		t.Fatalf("expected stable without reasons: %+v", result)
	}
	if result.Samples != 3 || result.LastSeq != 3 || result.Age != time.Second {
		t.Fatalf("unexpected result fields: %+v", result)
	}
}

func TestStabilityWindowReportsMissing(t *testing.T) {
	base := time.Now().UTC()
	window := StabilityWindow{MinimumSamples: 3, MaximumSpread: 1.5, MinimumPeriod: 5 * time.Second}
	result, err := window.Evaluate(currentSamples(base, 1, 10, 10.2), base.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if result.Stable || !hasReason(result, StabilityMissing) {
		t.Fatalf("expected missing reason: %+v", result)
	}
	summary := result.ReasonSummary()
	if !strings.Contains(summary, "missing samples: have 2, need at least 3") ||
		!strings.Contains(summary, "missing time span: have 1s, need at least 5s") {
		t.Fatalf("summary lacks detail: %q", summary)
	}
}

func TestStabilityWindowReportsMissingForEmptyInput(t *testing.T) {
	window := StabilityWindow{MinimumSamples: 2, MaximumSpread: 1.5, MinimumPeriod: time.Second}
	result, err := window.Evaluate(nil, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if result.Stable || !hasReason(result, StabilityMissing) {
		t.Fatalf("expected missing reason for empty input: %+v", result)
	}
}

func TestStabilityWindowReportsStale(t *testing.T) {
	base := time.Now().UTC()
	window := StabilityWindow{MinimumSamples: 2, MaximumSpread: 1.5, MinimumPeriod: time.Second, MaximumAge: 2 * time.Second}
	result, err := window.Evaluate(currentSamples(base, 1, 10, 10.1), base.Add(10*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if result.Stable || !hasReason(result, StabilityStale) {
		t.Fatalf("expected stale reason: %+v", result)
	}
	if result.Age != 9*time.Second {
		t.Fatalf("unexpected age: %s", result.Age)
	}
}

func TestStabilityWindowSkipsFreshnessWithoutMaximumAge(t *testing.T) {
	base := time.Now().UTC()
	window := StabilityWindow{MinimumSamples: 2, MaximumSpread: 1.5, MinimumPeriod: time.Second}
	result, err := window.Evaluate(currentSamples(base, 1, 10, 10.1), base.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if !result.Stable || hasReason(result, StabilityStale) {
		t.Fatalf("zero maximum age must disable the freshness check: %+v", result)
	}
}

func TestStabilityWindowReportsUnstable(t *testing.T) {
	base := time.Now().UTC()
	window := StabilityWindow{MinimumSamples: 2, MaximumSpread: 1.5, MinimumPeriod: time.Second}
	result, err := window.Evaluate(currentSamples(base, 1, 10, 13), base.Add(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if result.Stable || !hasReason(result, StabilityUnstable) {
		t.Fatalf("expected unstable reason: %+v", result)
	}
}

func TestStabilityWindowRejectsInvalidConfiguration(t *testing.T) {
	window := StabilityWindow{MinimumSamples: 1, MaximumSpread: 1.5}
	if _, err := window.Evaluate(nil, time.Now().UTC()); err == nil {
		t.Fatal("expected invalid window error")
	}
}

func TestStabilityPolicyResolvesMostSpecificRule(t *testing.T) {
	unified := StabilityWindow{MinimumSamples: 2, MaximumSpread: 1.5, MinimumPeriod: time.Second}
	kindRule := StabilityRule{Kind: KindCurrent, Window: StabilityWindow{MinimumSamples: 2, MaximumSpread: 2, MinimumPeriod: time.Second}}
	assetRule := StabilityRule{AssetID: "R07", Kind: KindCurrent, Window: StabilityWindow{MinimumSamples: 2, MaximumSpread: 3, MinimumPeriod: time.Second}}
	generationRule := StabilityRule{AssetID: "R07", Kind: KindCurrent, Generation: 4, Window: StabilityWindow{MinimumSamples: 2, MaximumSpread: 4, MinimumPeriod: time.Second}}
	policy := NewStabilityPolicy(unified, kindRule, assetRule, generationRule)

	if got := policy.WindowFor("R09", KindOpticalPower, 1); got != unified {
		t.Fatalf("expected unified fallback, got %+v", got)
	}
	if got := policy.WindowFor("R09", KindCurrent, 1); got != kindRule.Window {
		t.Fatalf("expected kind rule, got %+v", got)
	}
	if got := policy.WindowFor("R07", KindCurrent, 1); got != assetRule.Window {
		t.Fatalf("expected asset+kind rule, got %+v", got)
	}
	if got := policy.WindowFor("R07", KindCurrent, 4); got != generationRule.Window {
		t.Fatalf("expected generation rule, got %+v", got)
	}
}

func TestStabilityPolicyEvaluateScopesToNewestGeneration(t *testing.T) {
	base := time.Now().UTC()
	policy := NewStabilityPolicy(
		StabilityWindow{MinimumSamples: 2, MaximumSpread: 1.5, MinimumPeriod: time.Second},
		StabilityRule{AssetID: "R07", Kind: KindCurrent, Generation: 2, Window: StabilityWindow{MinimumSamples: 2, MaximumSpread: 0.5, MinimumPeriod: time.Second}},
	)
	samples := append(currentSamples(base, 1, 10, 10.1, 10.2), currentSamples(base.Add(3*time.Second), 2, 20, 20.4)...)
	result, err := policy.Evaluate(samples, base.Add(4*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if !result.Stable {
		t.Fatalf("expected generation-2 window to judge only its own samples: %+v", result)
	}
	if result.Samples != 2 || result.Window.MaximumSpread != 0.5 {
		t.Fatalf("evaluation used wrong scope or window: %+v", result)
	}
}

func TestStabilityPolicyEvaluateEmptyInputUsesUnifiedWindow(t *testing.T) {
	policy := NewStabilityPolicy(StabilityWindow{MinimumSamples: 4, MaximumSpread: 1.5, MinimumPeriod: time.Second})
	result, err := policy.Evaluate(nil, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if result.Stable || !hasReason(result, StabilityMissing) || result.Window.MinimumSamples != 4 {
		t.Fatalf("expected unified missing judgment: %+v", result)
	}
}

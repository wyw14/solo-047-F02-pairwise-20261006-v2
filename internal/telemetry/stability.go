package telemetry

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

// StabilityWindow describes the thresholds a metric stream must meet before
// it is considered stable. A zero MaximumAge disables the freshness check.
type StabilityWindow struct {
	MinimumSamples int
	MaximumSpread  float64
	MinimumPeriod  time.Duration
	MaximumAge     time.Duration
}

// StabilityIssue identifies why a window evaluation failed.
type StabilityIssue string

const (
	StabilityMissing  StabilityIssue = "missing"
	StabilityStale    StabilityIssue = "stale"
	StabilityUnstable StabilityIssue = "unstable"
)

// StabilityReason explains one failed stability check.
type StabilityReason struct {
	Code    StabilityIssue `json:"code"`
	Message string         `json:"message"`
}

type StabilityResult struct {
	Stable  bool              `json:"stable"`
	Spread  float64           `json:"spread"`
	Period  time.Duration     `json:"period"`
	Samples int               `json:"samples"`
	LastSeq uint64            `json:"last_sequence"`
	Age     time.Duration     `json:"age"`
	Window  StabilityWindow   `json:"window"`
	Reasons []StabilityReason `json:"reasons,omitempty"`
}

// ReasonSummary flattens the failure reasons for error messages.
func (r StabilityResult) ReasonSummary() string {
	if len(r.Reasons) == 0 {
		return "unknown"
	}
	messages := make([]string, 0, len(r.Reasons))
	for _, reason := range r.Reasons {
		messages = append(messages, reason.Message)
	}
	return strings.Join(messages, "; ")
}

func (w StabilityWindow) validate() error {
	if w.MinimumSamples < 2 || w.MaximumSpread < 0 || w.MinimumPeriod < 0 || w.MaximumAge < 0 {
		return errors.New("invalid stability window")
	}
	return nil
}

// Evaluate applies the unified window to samples. asOf is the reference time
// for the freshness check; a zero asOf skips it.
func (w StabilityWindow) Evaluate(samples []Sample, asOf time.Time) (StabilityResult, error) {
	if err := w.validate(); err != nil {
		return StabilityResult{}, err
	}
	result := StabilityResult{Window: w}
	if len(samples) == 0 {
		result.Reasons = append(result.Reasons, StabilityReason{
			Code:    StabilityMissing,
			Message: fmt.Sprintf("missing samples: have 0, need at least %d", w.MinimumSamples),
		})
		return result, nil
	}
	minimum, maximum := samples[0].Value, samples[0].Value
	first, last := samples[0].ObservedAt, samples[0].ObservedAt
	var lastSequence uint64
	for _, sample := range samples {
		minimum = math.Min(minimum, sample.Value)
		maximum = math.Max(maximum, sample.Value)
		if sample.ObservedAt.Before(first) {
			first = sample.ObservedAt
		}
		if sample.ObservedAt.After(last) {
			last = sample.ObservedAt
		}
		if sample.Sequence > lastSequence {
			lastSequence = sample.Sequence
		}
	}
	result.Spread = maximum - minimum
	result.Period = last.Sub(first)
	result.Samples = len(samples)
	result.LastSeq = lastSequence
	if !asOf.IsZero() {
		result.Age = asOf.Sub(last)
		if result.Age < 0 {
			result.Age = 0
		}
	}
	if result.Samples < w.MinimumSamples {
		result.Reasons = append(result.Reasons, StabilityReason{
			Code:    StabilityMissing,
			Message: fmt.Sprintf("missing samples: have %d, need at least %d", result.Samples, w.MinimumSamples),
		})
	}
	if result.Period < w.MinimumPeriod {
		result.Reasons = append(result.Reasons, StabilityReason{
			Code:    StabilityMissing,
			Message: fmt.Sprintf("missing time span: have %s, need at least %s", result.Period, w.MinimumPeriod),
		})
	}
	if w.MaximumAge > 0 && !asOf.IsZero() && result.Age > w.MaximumAge {
		result.Reasons = append(result.Reasons, StabilityReason{
			Code:    StabilityStale,
			Message: fmt.Sprintf("stale samples: newest is %s old, maximum age is %s", result.Age, w.MaximumAge),
		})
	}
	if result.Spread > w.MaximumSpread {
		result.Reasons = append(result.Reasons, StabilityReason{
			Code:    StabilityUnstable,
			Message: fmt.Sprintf("unstable spread: %g exceeds maximum %g", result.Spread, w.MaximumSpread),
		})
	}
	result.Stable = len(result.Reasons) == 0
	return result, nil
}

// StabilityRule overrides the unified window for a matching scope. An empty
// AssetID, empty Kind or zero Generation each match every value.
type StabilityRule struct {
	AssetID    string
	Kind       Kind
	Generation uint64
	Window     StabilityWindow
}

func (r StabilityRule) matches(assetID string, kind Kind, generation uint64) bool {
	if r.AssetID != "" && r.AssetID != assetID {
		return false
	}
	if r.Kind != "" && r.Kind != kind {
		return false
	}
	if r.Generation != 0 && r.Generation != generation {
		return false
	}
	return true
}

func (r StabilityRule) specificity() int {
	score := 0
	if r.AssetID != "" {
		score++
	}
	if r.Kind != "" {
		score++
	}
	if r.Generation != 0 {
		score++
	}
	return score
}

// StabilityPolicy resolves a StabilityWindow per asset, metric kind and
// generation, falling back to the unified window when no rule matches.
type StabilityPolicy struct {
	unified StabilityWindow
	rules   []StabilityRule
}

func NewStabilityPolicy(unified StabilityWindow, rules ...StabilityRule) StabilityPolicy {
	return StabilityPolicy{unified: unified, rules: append([]StabilityRule(nil), rules...)}
}

// WindowFor returns the most specific matching rule window, or the unified
// window when no rule matches.
func (p StabilityPolicy) WindowFor(assetID string, kind Kind, generation uint64) StabilityWindow {
	window, specificity := p.unified, 0
	for _, rule := range p.rules {
		if !rule.matches(assetID, kind, generation) {
			continue
		}
		if score := rule.specificity(); score > specificity {
			window, specificity = rule.Window, score
		}
	}
	return window
}

// Evaluate resolves the window for the newest sample's asset, kind and
// generation, then evaluates only the samples from that same scope. Empty
// input is judged against the unified window.
func (p StabilityPolicy) Evaluate(samples []Sample, asOf time.Time) (StabilityResult, error) {
	if len(samples) == 0 {
		return p.unified.Evaluate(nil, asOf)
	}
	newest := samples[0]
	for _, sample := range samples[1:] {
		if sample.ObservedAt.After(newest.ObservedAt) ||
			(sample.ObservedAt.Equal(newest.ObservedAt) && sample.Sequence > newest.Sequence) {
			newest = sample
		}
	}
	window := p.WindowFor(newest.AssetID, newest.Kind, newest.Generation)
	scoped := make([]Sample, 0, len(samples))
	for _, sample := range samples {
		if sample.AssetID == newest.AssetID && sample.Kind == newest.Kind && sample.Generation == newest.Generation {
			scoped = append(scoped, sample)
		}
	}
	return window.Evaluate(scoped, asOf)
}

func CurrentSamples(repository *Repository, repeaterID string, after uint64) []Sample {
	var result []Sample
	for _, sample := range repository.History(repeaterID) {
		if sample.Kind == KindCurrent && sample.Sequence > after {
			result = append(result, sample)
		}
	}
	return result
}

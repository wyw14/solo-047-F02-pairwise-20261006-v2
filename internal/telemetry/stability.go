package telemetry

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
)

// StabilityRule carries the thresholds a set of samples must satisfy for a
// scope (asset, metric kind and generation) to be considered stable.
type StabilityRule struct {
	MinimumSamples int           `json:"minimum_samples"`
	MinimumPeriod  time.Duration `json:"minimum_period"`
	MaximumSpread  float64       `json:"maximum_spread"`
	// MaximumAge bounds how old the newest sample may be. Zero disables the
	// freshness check.
	MaximumAge time.Duration `json:"maximum_age"`
}

// Validate reports invalid thresholds. MinimumSamples is allowed to be 0 or
// 1: an empty window then means "missing" rather than "insufficient".
func (r StabilityRule) Validate() error {
	var problems []string
	if r.MinimumSamples < 0 {
		problems = append(problems, "minimum samples must not be negative")
	}
	if r.MinimumPeriod < 0 {
		problems = append(problems, "minimum period must not be negative")
	}
	if r.MaximumSpread < 0 {
		problems = append(problems, "maximum spread must not be negative")
	}
	if r.MaximumAge < 0 {
		problems = append(problems, "maximum age must not be negative")
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

// StabilityScope identifies the asset, metric kind and generation an
// evaluation runs for. Generation 0 means "the newest generation present for
// the asset/kind".
type StabilityScope struct {
	AssetID    string `json:"asset_id"`
	Kind       Kind   `json:"kind"`
	Generation uint64 `json:"generation,omitempty"`
}

func (s StabilityScope) String() string {
	if s.Generation == 0 {
		return fmt.Sprintf("asset=%s kind=%s generation=latest", s.AssetID, s.Kind)
	}
	return fmt.Sprintf("asset=%s kind=%s generation=%d", s.AssetID, s.Kind, s.Generation)
}

// ReasonCode explains why a stability evaluation did not pass.
type ReasonCode string

const (
	// ReasonMissing means no samples exist for the scope (or sequence
	// cursor).
	ReasonMissing ReasonCode = "missing"
	// ReasonInsufficientSamples means samples exist but fewer than the rule
	// requires.
	ReasonInsufficientSamples ReasonCode = "insufficient_samples"
	// ReasonPeriodTooShort means the samples do not span the required period.
	ReasonPeriodTooShort ReasonCode = "period_too_short"
	// ReasonUnstableSpread means value spread exceeds the allowed maximum.
	ReasonUnstableSpread ReasonCode = "unstable_spread"
	// ReasonStale means the newest sample is older than MaximumAge.
	ReasonStale ReasonCode = "stale"
)

// StabilityFailure describes a single failed criterion with the observed and
// required values, so callers can report a concrete reason.
type StabilityFailure struct {
	Code     ReasonCode     `json:"code"`
	Message  string         `json:"message"`
	Scope    StabilityScope `json:"scope"`
	Have     float64        `json:"have,omitempty"`
	Want     float64        `json:"want,omitempty"`
	Duration time.Duration  `json:"duration,omitempty"`
	Limit    time.Duration  `json:"limit,omitempty"`
	Age      time.Duration  `json:"age,omitempty"`
	LimitAge time.Duration  `json:"limit_age,omitempty"`
}

func (f StabilityFailure) Error() string {
	return fmt.Sprintf("%s: %s", f.Code, f.Message)
}

// StabilityError wraps a non-stable evaluation so callers can surface the
// concrete missing/stale/unstable reasons programmatically.
type StabilityError struct {
	Result ConfiguredStabilityResult
}

func (e *StabilityError) Error() string {
	reasons := make([]string, 0, len(e.Result.Reasons))
	for _, reason := range e.Result.Reasons {
		reasons = append(reasons, reason.Error())
	}
	return fmt.Sprintf("%s is not stable: %s", e.Result.Scope, strings.Join(reasons, "; "))
}

// AsFailure exposes the (single) failure when callers only need one, e.g.
// errors.As(err, new(*telemetry.StabilityFailure)).
func (e *StabilityError) As(target any) bool {
	failure, ok := target.(*StabilityFailure)
	if !ok || len(e.Result.Reasons) == 0 {
		return false
	}
	*failure = e.Result.Reasons[0]
	return true
}

// ConfiguredStabilityResult is the outcome of a per-scope rule evaluation,
// including every failed criterion.
type ConfiguredStabilityResult struct {
	Stable  bool               `json:"stable"`
	Scope   StabilityScope     `json:"scope"`
	Rule    StabilityRule      `json:"rule"`
	Metrics StabilityResult    `json:"metrics"`
	Reasons []StabilityFailure `json:"reasons,omitempty"`
}

// StabilityPolicy resolves a rule per asset, metric kind and generation, and
// evaluates samples against it. The default rule applies to every scope that
// has no explicit override.
type StabilityPolicy struct {
	now         func() time.Time
	defaultRule StabilityRule
	overrides   map[scopeKey]StabilityRule
}

type scopeKey struct {
	assetID    string
	kind       Kind
	generation uint64
}

// NewStabilityPolicy builds a policy with a single uniform rule.
func NewStabilityPolicy(now func() time.Time, defaultRule StabilityRule) (*StabilityPolicy, error) {
	if now == nil {
		return nil, errors.New("clock is required")
	}
	if err := defaultRule.Validate(); err != nil {
		return nil, fmt.Errorf("invalid default stability rule: %w", err)
	}
	return &StabilityPolicy{
		now:         now,
		defaultRule: defaultRule,
		overrides:   make(map[scopeKey]StabilityRule),
	}, nil
}

// SetRule registers a rule for a scope. Blank fields broaden the override:
// exact (asset+kind+generation), asset+kind, kind-only, then the default rule
// apply in that order during lookup.
func (p *StabilityPolicy) SetRule(scope StabilityScope, rule StabilityRule) error {
	if err := rule.Validate(); err != nil {
		return fmt.Errorf("invalid rule for %s: %w", scope, err)
	}
	p.overrides[scopeKey{assetID: scope.AssetID, kind: scope.Kind, generation: scope.Generation}] = rule
	return nil
}

// Rule resolves the configured rule for a scope, falling back from exact
// generation overrides to asset+kind, kind-only and finally the default rule.
func (p *StabilityPolicy) Rule(scope StabilityScope) StabilityRule {
	if rule, ok := p.overrides[scopeKey{assetID: scope.AssetID, kind: scope.Kind, generation: scope.Generation}]; ok {
		return rule
	}
	if rule, ok := p.overrides[scopeKey{assetID: scope.AssetID, kind: scope.Kind}]; ok {
		return rule
	}
	if rule, ok := p.overrides[scopeKey{kind: scope.Kind}]; ok {
		return rule
	}
	return p.defaultRule
}

// Evaluate collects the samples matching the scope (optionally restricted to
// sequences after the cursor) and applies the resolved rule. A scope with
// generation 0 is pinned to the asset's newest generation across all metric
// kinds, since generation is the asset's epoch.
func (p *StabilityPolicy) Evaluate(repository *Repository, scope StabilityScope, after uint64) (ConfiguredStabilityResult, error) {
	if scope.AssetID == "" || scope.Kind == "" {
		return ConfiguredStabilityResult{}, errors.New("stability scope requires asset and kind")
	}
	scope = p.resolveGeneration(repository, scope)
	samples := WindowSamples(repository, scope, after)
	return p.evaluate(scope, p.Rule(scope), samples), nil
}

func (p *StabilityPolicy) resolveGeneration(repository *Repository, scope StabilityScope) StabilityScope {
	if scope.Generation != 0 {
		return scope
	}
	for _, sample := range repository.History(scope.AssetID) {
		if sample.Generation > scope.Generation {
			scope.Generation = sample.Generation
		}
	}
	return scope
}

func (p *StabilityPolicy) evaluate(scope StabilityScope, rule StabilityRule, samples []Sample) ConfiguredStabilityResult {
	result := ConfiguredStabilityResult{Stable: true, Scope: scope, Rule: rule, Metrics: computeMetrics(samples)}

	if len(samples) == 0 {
		result.Stable = false
		result.Reasons = append(result.Reasons, StabilityFailure{
			Code: ReasonMissing, Scope: scope,
			Message: fmt.Sprintf("no %s samples for asset %s", scope.Kind, scope.AssetID),
		})
		return result
	}

	if rule.MinimumSamples > 0 && result.Metrics.Samples < rule.MinimumSamples {
		result.Stable = false
		result.Reasons = append(result.Reasons, StabilityFailure{
			Code: ReasonInsufficientSamples, Scope: scope,
			Message: fmt.Sprintf("need %d samples, have %d", rule.MinimumSamples, result.Metrics.Samples),
			Have:    float64(result.Metrics.Samples), Want: float64(rule.MinimumSamples),
		})
	}
	if result.Metrics.Period < rule.MinimumPeriod {
		result.Stable = false
		result.Reasons = append(result.Reasons, StabilityFailure{
			Code: ReasonPeriodTooShort, Scope: scope,
			Message:  fmt.Sprintf("samples span %s, need %s", result.Metrics.Period, rule.MinimumPeriod),
			Duration: result.Metrics.Period, Limit: rule.MinimumPeriod,
		})
	}
	if result.Metrics.Spread > rule.MaximumSpread {
		result.Stable = false
		result.Reasons = append(result.Reasons, StabilityFailure{
			Code: ReasonUnstableSpread, Scope: scope,
			Message: fmt.Sprintf("spread %.3f exceeds %.3f", result.Metrics.Spread, rule.MaximumSpread),
			Have:    result.Metrics.Spread, Want: rule.MaximumSpread,
		})
	}
	if rule.MaximumAge > 0 {
		age := p.freshness(samples)
		if age > rule.MaximumAge {
			result.Stable = false
			result.Reasons = append(result.Reasons, StabilityFailure{
				Code: ReasonStale, Scope: scope,
				Message: fmt.Sprintf("newest sample is %s old, limit %s", age, rule.MaximumAge),
				Age:     age, LimitAge: rule.MaximumAge,
			})
		}
	}
	return result
}

func (p *StabilityPolicy) freshness(samples []Sample) time.Duration {
	last := samples[0].ObservedAt
	for _, sample := range samples[1:] {
		if sample.ObservedAt.After(last) {
			last = sample.ObservedAt
		}
	}
	age := p.now().UTC().Sub(last.UTC())
	if age < 0 {
		return 0
	}
	return age
}

// WindowSamples returns repository samples matching the scope: asset, kind
// and generation (0 selects the asset's newest generation across all kinds),
// with sequence greater than after. When no sample carries a generation the
// data predates generation tracking and every matching kind is returned,
// preserving the legacy behavior.
func WindowSamples(repository *Repository, scope StabilityScope, after uint64) []Sample {
	history := repository.History(scope.AssetID)
	generation := scope.Generation
	if generation == 0 {
		for _, sample := range history {
			if sample.Generation > generation {
				generation = sample.Generation
			}
		}
	}
	var result []Sample
	for _, sample := range history {
		if sample.Kind != scope.Kind || sample.Sequence <= after {
			continue
		}
		if generation != 0 && sample.Generation != generation {
			continue
		}
		result = append(result, sample)
	}
	return result
}

// --- uniform window (existing health projection path) ---------------------

type StabilityWindow struct {
	MinimumSamples int
	MaximumSpread  float64
	MinimumPeriod  time.Duration
}

type StabilityResult struct {
	Stable  bool          `json:"stable"`
	Spread  float64       `json:"spread"`
	Period  time.Duration `json:"period"`
	Samples int           `json:"samples"`
	LastSeq uint64        `json:"last_sequence"`
}

func (w StabilityWindow) Evaluate(samples []Sample) (StabilityResult, error) {
	if w.MinimumSamples < 2 || w.MaximumSpread < 0 || w.MinimumPeriod < 0 {
		return StabilityResult{}, errors.New("invalid stability window")
	}
	if len(samples) == 0 {
		return StabilityResult{}, nil
	}
	result := computeMetrics(samples)
	result.Stable = result.Samples >= w.MinimumSamples && result.Spread <= w.MaximumSpread && result.Period >= w.MinimumPeriod
	return result, nil
}

func computeMetrics(samples []Sample) StabilityResult {
	if len(samples) == 0 {
		return StabilityResult{}
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
	return StabilityResult{
		Spread: maximum - minimum, Period: last.Sub(first), Samples: len(samples), LastSeq: lastSequence,
	}
}

func CurrentSamples(repository *Repository, repeaterID string, after uint64) []Sample {
	return WindowSamples(repository, StabilityScope{AssetID: repeaterID, Kind: KindCurrent}, after)
}

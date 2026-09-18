package health

import (
	"fmt"
	"time"
)

// RecencyClass describes how representative evidence is for a requested
// evaluation time. These are BeeBase evaluation-policy categories, not
// biological expiration states.
type RecencyClass string

const (
	RecencyCurrent RecencyClass = "CURRENT"
	RecencyRecent  RecencyClass = "RECENT"
	RecencyStale   RecencyClass = "STALE"
)

// DimensionRecencyPolicy defines the elapsed-time boundaries for one health
// dimension. Both boundaries are inclusive at their upper edge.
type DimensionRecencyPolicy struct {
	CurrentFor time.Duration
	RecentFor  time.Duration
}

// RecencyPolicy is a versioned, year-round evaluation policy.
type RecencyPolicy struct {
	Version    string
	Dimensions map[HealthDimension]DimensionRecencyPolicy
}

const recencyPolicyV1Version = "v1"

var recencyDimensions = [...]HealthDimension{
	DimensionStrength,
	DimensionQueen,
	DimensionBrood,
	DimensionNutrition,
	DimensionPestsAndDisease,
	DimensionOverall,
}

// DefaultRecencyPolicyV1 returns an independent copy of BeeBase's canonical
// v1 policy. The returned map may be modified by the caller without changing
// subsequent policy instances.
func DefaultRecencyPolicyV1() RecencyPolicy {
	return RecencyPolicy{
		Version: recencyPolicyV1Version,
		Dimensions: map[HealthDimension]DimensionRecencyPolicy{
			DimensionStrength:        {CurrentFor: 14 * 24 * time.Hour, RecentFor: 30 * 24 * time.Hour},
			DimensionQueen:           {CurrentFor: 14 * 24 * time.Hour, RecentFor: 30 * 24 * time.Hour},
			DimensionBrood:           {CurrentFor: 14 * 24 * time.Hour, RecentFor: 30 * 24 * time.Hour},
			DimensionNutrition:       {CurrentFor: 10 * 24 * time.Hour, RecentFor: 30 * 24 * time.Hour},
			DimensionPestsAndDisease: {CurrentFor: 14 * 24 * time.Hour, RecentFor: 30 * 24 * time.Hour},
			DimensionOverall:         {CurrentFor: 14 * 24 * time.Hour, RecentFor: 30 * 24 * time.Hour},
		},
	}
}

// Validate checks that a policy is complete and internally consistent.
func (p RecencyPolicy) Validate() error {
	if p.Version == "" {
		return fmt.Errorf("recency policy version is required")
	}
	if p.Dimensions == nil {
		return fmt.Errorf("recency policy dimensions are required")
	}

	known := make(map[HealthDimension]struct{}, len(recencyDimensions))
	for _, dimension := range recencyDimensions {
		known[dimension] = struct{}{}
		limits, ok := p.Dimensions[dimension]
		if !ok {
			return fmt.Errorf("recency policy is missing dimension %q", dimension)
		}
		if limits.CurrentFor < 0 {
			return fmt.Errorf("recency policy dimension %q has negative current boundary", dimension)
		}
		if limits.RecentFor < 0 {
			return fmt.Errorf("recency policy dimension %q has negative recent boundary", dimension)
		}
		if limits.RecentFor < limits.CurrentFor {
			return fmt.Errorf("recency policy dimension %q has recent boundary before current boundary", dimension)
		}
	}
	for dimension := range p.Dimensions {
		if _, ok := known[dimension]; !ok {
			return fmt.Errorf("recency policy contains unknown dimension %q", dimension)
		}
	}
	return nil
}

// ClassifyRecency classifies evidence relative to the explicitly supplied
// AsOf instant. The bool is false when the evidence is future-dated and is
// therefore ineligible for that evaluation. No system clock is consulted.
func ClassifyRecency(evidence HealthEvidence, asOf time.Time, policy RecencyPolicy) (RecencyClass, bool, error) {
	if err := policy.Validate(); err != nil {
		return "", false, err
	}
	if asOf.IsZero() {
		return "", false, fmt.Errorf("recency evaluation AsOf is required")
	}
	if evidence.Source.OccurredAt.IsZero() {
		return "", false, fmt.Errorf("health evidence occurrence time is required")
	}
	limits, ok := policy.Dimensions[evidence.Dimension]
	if !ok {
		return "", false, fmt.Errorf("recency policy has no dimension %q", evidence.Dimension)
	}
	if evidence.Source.OccurredAt.After(asOf) {
		return "", false, nil
	}

	age := asOf.Sub(evidence.Source.OccurredAt)
	switch {
	case age <= limits.CurrentFor:
		return RecencyCurrent, true, nil
	case age <= limits.RecentFor:
		return RecencyRecent, true, nil
	default:
		return RecencyStale, true, nil
	}
}

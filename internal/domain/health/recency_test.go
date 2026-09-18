package health

import (
	"strings"
	"testing"
	"time"
)

func TestDefaultRecencyPolicyV1(t *testing.T) {
	policy := DefaultRecencyPolicyV1()
	if policy.Version != "v1" {
		t.Fatalf("version = %q, want v1", policy.Version)
	}

	want := map[HealthDimension]DimensionRecencyPolicy{
		DimensionStrength:        {CurrentFor: 14 * 24 * time.Hour, RecentFor: 30 * 24 * time.Hour},
		DimensionQueen:           {CurrentFor: 14 * 24 * time.Hour, RecentFor: 30 * 24 * time.Hour},
		DimensionBrood:           {CurrentFor: 14 * 24 * time.Hour, RecentFor: 30 * 24 * time.Hour},
		DimensionNutrition:       {CurrentFor: 10 * 24 * time.Hour, RecentFor: 30 * 24 * time.Hour},
		DimensionPestsAndDisease: {CurrentFor: 14 * 24 * time.Hour, RecentFor: 30 * 24 * time.Hour},
		DimensionOverall:         {CurrentFor: 14 * 24 * time.Hour, RecentFor: 30 * 24 * time.Hour},
	}
	if len(policy.Dimensions) != len(want) {
		t.Fatalf("dimension count = %d, want %d", len(policy.Dimensions), len(want))
	}
	for dimension, expected := range want {
		if got := policy.Dimensions[dimension]; got != expected {
			t.Errorf("%s policy = %#v, want %#v", dimension, got, expected)
		}
	}
	if err := policy.Validate(); err != nil {
		t.Fatalf("canonical policy is invalid: %v", err)
	}
}

func TestDefaultRecencyPolicyV1ReturnsIndependentMap(t *testing.T) {
	first := DefaultRecencyPolicyV1()
	first.Dimensions[DimensionStrength] = DimensionRecencyPolicy{}
	second := DefaultRecencyPolicyV1()
	if second.Dimensions[DimensionStrength].CurrentFor != 14*24*time.Hour {
		t.Fatalf("canonical policy was mutated through a returned map: %#v", second.Dimensions[DimensionStrength])
	}
}

func TestClassifyRecencyBoundaries(t *testing.T) {
	policy := DefaultRecencyPolicyV1()
	asOf := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name         string
		dimension    HealthDimension
		age          time.Duration
		wantClass    RecencyClass
		wantEligible bool
	}{
		{name: "strength current boundary", dimension: DimensionStrength, age: 14 * 24 * time.Hour, wantClass: RecencyCurrent, wantEligible: true},
		{name: "strength recent after current boundary", dimension: DimensionStrength, age: 14*24*time.Hour + time.Nanosecond, wantClass: RecencyRecent, wantEligible: true},
		{name: "strength recent boundary", dimension: DimensionStrength, age: 30 * 24 * time.Hour, wantClass: RecencyRecent, wantEligible: true},
		{name: "strength stale after recent boundary", dimension: DimensionStrength, age: 30*24*time.Hour + time.Nanosecond, wantClass: RecencyStale, wantEligible: true},
		{name: "nutrition current boundary", dimension: DimensionNutrition, age: 10 * 24 * time.Hour, wantClass: RecencyCurrent, wantEligible: true},
		{name: "nutrition recent after current boundary", dimension: DimensionNutrition, age: 10*24*time.Hour + time.Nanosecond, wantClass: RecencyRecent, wantEligible: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotClass, gotEligible, err := ClassifyRecency(evidenceAt(tt.dimension, asOf.Add(-tt.age)), asOf, policy)
			if err != nil {
				t.Fatalf("ClassifyRecency() error = %v", err)
			}
			if gotClass != tt.wantClass || gotEligible != tt.wantEligible {
				t.Fatalf("ClassifyRecency() = (%q, %t), want (%q, %t)", gotClass, gotEligible, tt.wantClass, tt.wantEligible)
			}
		})
	}
}

func TestClassifyRecencyUsesInstantsNotCalendarDates(t *testing.T) {
	policy := DefaultRecencyPolicyV1()
	asOf := time.Date(2026, 9, 18, 12, 0, 0, 0, time.FixedZone("offset", 2*60*60))
	occurredAt := asOf.Add(-14 * 24 * time.Hour)

	gotClass, eligible, err := ClassifyRecency(evidenceAt(DimensionStrength, occurredAt), asOf, policy)
	if err != nil {
		t.Fatalf("ClassifyRecency() error = %v", err)
	}
	if gotClass != RecencyCurrent || !eligible {
		t.Fatalf("ClassifyRecency() = (%q, %t), want (%q, true)", gotClass, eligible, RecencyCurrent)
	}
}

func TestClassifyRecencyExcludesFutureEvidence(t *testing.T) {
	policy := DefaultRecencyPolicyV1()
	asOf := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)

	gotClass, eligible, err := ClassifyRecency(evidenceAt(DimensionStrength, asOf.Add(time.Nanosecond)), asOf, policy)
	if err != nil {
		t.Fatalf("ClassifyRecency() error = %v", err)
	}
	if gotClass != "" || eligible {
		t.Fatalf("future evidence = (%q, %t), want (empty, false)", gotClass, eligible)
	}
}

func TestClassifyRecencyRejectsInvalidPolicyAndEvidence(t *testing.T) {
	asOf := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
	validEvidence := evidenceAt(DimensionStrength, asOf.Add(-time.Hour))

	tests := []struct {
		name      string
		policy    RecencyPolicy
		evidence  HealthEvidence
		asOf      time.Time
		wantError string
	}{
		{name: "empty version", policy: RecencyPolicy{Dimensions: DefaultRecencyPolicyV1().Dimensions}, evidence: validEvidence, asOf: asOf, wantError: "version"},
		{name: "missing dimension", policy: policyWithout(DimensionQueen), evidence: validEvidence, asOf: asOf, wantError: "missing dimension"},
		{name: "negative current", policy: policyWith(DimensionStrength, DimensionRecencyPolicy{CurrentFor: -time.Nanosecond, RecentFor: time.Hour}), evidence: validEvidence, asOf: asOf, wantError: "negative current"},
		{name: "negative recent", policy: policyWith(DimensionStrength, DimensionRecencyPolicy{CurrentFor: time.Hour, RecentFor: -time.Nanosecond}), evidence: validEvidence, asOf: asOf, wantError: "negative recent"},
		{name: "recent before current", policy: policyWith(DimensionStrength, DimensionRecencyPolicy{CurrentFor: 2 * time.Hour, RecentFor: time.Hour}), evidence: validEvidence, asOf: asOf, wantError: "recent boundary"},
		{name: "unknown evidence dimension", policy: DefaultRecencyPolicyV1(), evidence: evidenceAt(HealthDimension("UNKNOWN"), asOf.Add(-time.Hour)), asOf: asOf, wantError: "no dimension"},
		{name: "zero AsOf", policy: DefaultRecencyPolicyV1(), evidence: validEvidence, asOf: time.Time{}, wantError: "AsOf"},
		{name: "zero occurrence", policy: DefaultRecencyPolicyV1(), evidence: evidenceAt(DimensionStrength, time.Time{}), asOf: asOf, wantError: "occurrence"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := ClassifyRecency(tt.evidence, tt.asOf, tt.policy)
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("error = %v, want substring %q", err, tt.wantError)
			}
		})
	}
}

func TestClassifyRecencyValidatesPolicyBeforeFutureExclusion(t *testing.T) {
	policy := policyWithout(DimensionQueen)
	asOf := time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)

	_, eligible, err := ClassifyRecency(evidenceAt(DimensionStrength, asOf.Add(time.Hour)), asOf, policy)
	if err == nil || eligible {
		t.Fatalf("result = (%t, %v), want validation error and ineligible=false", eligible, err)
	}
}

func evidenceAt(dimension HealthDimension, occurredAt time.Time) HealthEvidence {
	return HealthEvidence{
		Dimension: dimension,
		Source:    EvidenceSource{OccurredAt: occurredAt},
	}
}

func policyWithout(dimension HealthDimension) RecencyPolicy {
	policy := DefaultRecencyPolicyV1()
	delete(policy.Dimensions, dimension)
	return policy
}

func policyWith(dimension HealthDimension, limits DimensionRecencyPolicy) RecencyPolicy {
	policy := DefaultRecencyPolicyV1()
	policy.Dimensions[dimension] = limits
	return policy
}

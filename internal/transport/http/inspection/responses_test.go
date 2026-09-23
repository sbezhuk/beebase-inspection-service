package inspection

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sbezhuk/beebase-health/health"
	appinspection "github.com/sbezhuk/beebase-inspection-service/internal/application/inspection"
	"github.com/sbezhuk/beebase-inspection-service/internal/domain/inspection"
)

func TestNewColonyHealthDimensionResponses_MapsContributingEvidenceToSources(t *testing.T) {
	inspectionID := uuid.New()
	routineType := health.TypeRoutine
	occurredAt := time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC)

	dimensions := []health.DimensionEvaluation{
		{
			Dimension: health.DimensionStrength,
			State:     health.DimensionGood,
			Coverage:  health.CoverageMedium,
			ContributingEvidence: []health.HealthEvidence{
				{
					Dimension: health.DimensionStrength,
					Kind:      health.EvidenceObservation,
					State:     health.EvidenceValueRecorded,
					Value:     ptr("STRONG"),
					Source: health.EvidenceSource{
						InspectionID:   &inspectionID,
						InspectionType: &routineType,
						OccurredAt:     occurredAt,
						SourceField:    health.SourceFieldColonyStrength,
					},
				},
			},
		},
	}

	out := newColonyHealthDimensionResponses(dimensions)
	if len(out) != 1 {
		t.Fatalf("len(out) = %d, want 1", len(out))
	}
	if len(out[0].Sources) != 1 {
		t.Fatalf("len(Sources) = %d, want 1", len(out[0].Sources))
	}
	source := out[0].Sources[0]
	if source.InspectionID != inspectionID {
		t.Errorf("InspectionID = %v, want %v", source.InspectionID, inspectionID)
	}
	if source.InspectionType != inspection.TypeRoutine {
		t.Errorf("InspectionType = %v, want ROUTINE", source.InspectionType)
	}
	if source.InspectedAt != "2026-09-18" {
		t.Errorf("InspectedAt = %q, want 2026-09-18", source.InspectedAt)
	}
	if source.Field != "colonyStrength" {
		t.Errorf("Field = %q, want colonyStrength", source.Field)
	}

	// The new field must round-trip through JSON under its documented key -
	// a client decoding the existing response type gains it for free.
	encoded, err := json.Marshal(out[0])
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if _, ok := decoded["sources"]; !ok {
		t.Fatalf("encoded response missing \"sources\" key: %s", encoded)
	}
	// Existing keys remain present and unrenamed.
	for _, key := range []string{"dimension", "state", "coverage"} {
		if _, ok := decoded[key]; !ok {
			t.Errorf("encoded response missing existing key %q - backward compatibility violated", key)
		}
	}
}

func TestNewColonyHealthDimensionResponses_NoContributingEvidenceYieldsEmptyNeverNullSources(t *testing.T) {
	dimensions := []health.DimensionEvaluation{
		{Dimension: health.DimensionPestsAndDisease, State: health.DimensionUnknown, Coverage: health.CoverageNone},
	}

	out := newColonyHealthDimensionResponses(dimensions)
	if out[0].Sources == nil {
		t.Fatal("Sources is nil, want a non-nil empty slice so it encodes as [] rather than null")
	}
	if len(out[0].Sources) != 0 {
		t.Fatalf("len(Sources) = %d, want 0", len(out[0].Sources))
	}

	encoded, err := json.Marshal(out[0])
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if string(encoded) == "" {
		t.Fatal("empty encoding")
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	sources, ok := decoded["sources"].([]interface{})
	if !ok {
		t.Fatalf("sources = %#v (%T), want a JSON array", decoded["sources"], decoded["sources"])
	}
	if len(sources) != 0 {
		t.Fatalf("sources = %v, want an empty array", sources)
	}
}

// TestNewColonyHealthDimensionResponses_SkipsEvidenceWithoutInspectionIdentity
// is a defensive regression: evidence.Source.InspectionID/InspectionType
// are always populated for real inspection-derived evidence (see
// health.sourceFor), but the mapping must not panic if that ever changed -
// it should simply omit that item rather than dereference a nil pointer.
func TestNewColonyHealthDimensionResponses_SkipsEvidenceWithoutInspectionIdentity(t *testing.T) {
	dimensions := []health.DimensionEvaluation{
		{
			Dimension: health.DimensionQueen,
			State:     health.DimensionGood,
			Coverage:  health.CoverageMedium,
			ContributingEvidence: []health.HealthEvidence{
				{
					Dimension: health.DimensionQueen,
					State:     health.EvidenceValueRecorded,
					Value:     ptr("HEALTHY"),
					Source: health.EvidenceSource{
						// InspectionID/InspectionType deliberately nil.
						OccurredAt:  time.Now(),
						SourceField: health.SourceFieldQueenStatus,
					},
				},
			},
		},
	}

	out := newColonyHealthDimensionResponses(dimensions)
	if len(out[0].Sources) != 0 {
		t.Fatalf("Sources = %+v, want empty when InspectionID/InspectionType are nil", out[0].Sources)
	}
}

// TestNewColonyHealthHistoryResponse_ExistingFieldsRemainBackwardCompatible
// spot-checks that every field present before this change still populates
// exactly as before, alongside the new additive Sources data - proving the
// extension didn't repurpose or rename anything existing.
func TestNewColonyHealthHistoryResponse_ExistingFieldsRemainBackwardCompatible(t *testing.T) {
	inspectionID := uuid.New()
	inspectedAt := time.Date(2026, 6, 2, 0, 0, 0, 0, time.UTC)
	from := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 6, 3, 0, 0, 0, 0, time.UTC)

	result := appinspection.HealthHistoryResult{
		From: from,
		To:   to,
		Points: []appinspection.HealthHistoryPoint{
			{
				Date: inspectedAt,
				Evaluation: health.ColonyHealthEvaluation{
					State:    health.DimensionGood,
					Coverage: health.CoverageMedium,
					Dimensions: []health.DimensionEvaluation{
						{Dimension: health.DimensionQueen, State: health.DimensionGood, Coverage: health.CoverageMedium},
					},
				},
			},
		},
		Inspections: []*inspection.Inspection{
			{ID: inspectionID, InspectedAt: inspectedAt, Type: inspection.TypeRoutine},
		},
	}

	got := newColonyHealthHistoryResponse(result)

	if got.AlgorithmVersion != health.DefaultRecencyPolicyV1().Version {
		t.Errorf("AlgorithmVersion = %q", got.AlgorithmVersion)
	}
	if got.From != "2026-06-01" || got.To != "2026-06-03" {
		t.Errorf("From/To = %s/%s", got.From, got.To)
	}
	if got.Interval != "DAY" {
		t.Errorf("Interval = %q, want DAY", got.Interval)
	}
	if len(got.Points) != 1 || got.Points[0].Date != "2026-06-02" {
		t.Fatalf("Points = %+v", got.Points)
	}
	if got.Points[0].State != health.DimensionGood || got.Points[0].Coverage != health.CoverageMedium {
		t.Errorf("Points[0] state/coverage = %q/%q, want GOOD/MEDIUM", got.Points[0].State, got.Points[0].Coverage)
	}
	if len(got.Inspections) != 1 || got.Inspections[0].ID != inspectionID || got.Inspections[0].Date != "2026-06-02" || got.Inspections[0].Type != inspection.TypeRoutine {
		t.Fatalf("Inspections = %+v", got.Inspections)
	}
}

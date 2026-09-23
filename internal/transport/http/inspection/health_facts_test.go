package inspection

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sbezhuk/beebase-inspection-service/internal/domain/inspection"
)

func TestNewHealthFactsInspectionResponsePreservesCanonicalFacts(t *testing.T) {
	inspectionID := uuid.New()
	hiveID := uuid.New()
	emptyStages := []inspection.BroodStage{}
	assessment := &inspection.Assessment{
		Version:                inspection.AssessmentVersion,
		ColonyStrength:         ptrHealth(inspection.ColonyStrengthStrong),
		QueenStatus:            ptrHealth(inspection.QueenStatusHealthy),
		BroodStages:            &emptyStages,
		PestSigns:              nil,
		HealthWarningSigns:     &[]inspection.HealthWarningSign{},
		SeasonalConcerns:       &[]inspection.SeasonalConcern{inspection.SeasonalConcernQueen},
		SeasonalReadiness:      ptrHealth(inspection.SeasonalReadinessReady),
		SeasonalStoreReadiness: ptrHealth(inspection.SeasonalStoresSufficient),
	}

	got := newHealthFactsInspectionResponse(&inspection.Inspection{
		ID: inspectionID, HiveID: hiveID,
		InspectedAt: time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC),
		Type:        inspection.TypeSeasonal, Assessment: assessment,
	})

	if got.ID != inspectionID || got.HiveID != hiveID || got.InspectedAt != "2026-09-23" || got.Type != inspection.TypeSeasonal {
		t.Fatalf("identity/date fields = %+v", got)
	}
	if got.Assessment == nil || got.Assessment.BroodStages == nil || len(*got.Assessment.BroodStages) != 0 {
		t.Fatalf("empty BroodStages was not preserved: %+v", got.Assessment)
	}
	if got.Assessment.PestSigns != nil {
		t.Fatalf("nil PestSigns became non-nil: %+v", got.Assessment.PestSigns)
	}
	if got.Assessment.HealthWarningSigns == nil || len(*got.Assessment.HealthWarningSigns) != 0 {
		t.Fatalf("empty HealthWarningSigns was not preserved: %+v", got.Assessment.HealthWarningSigns)
	}
	if got.Assessment.SeasonalConcerns == nil || len(*got.Assessment.SeasonalConcerns) != 1 {
		t.Fatalf("populated SeasonalConcerns was not preserved: %+v", got.Assessment.SeasonalConcerns)
	}

	payload, err := json.Marshal(HealthFactsResponse{HiveID: hiveID, Inspections: []HealthFactsInspectionResponse{got}})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	text := string(payload)
	for _, forbidden := range []string{"notes", "images", "userId", "createdAt", "updatedAt", "deletedAt", "colonyHealth", "state", "coverage", "dimensions", "provenance"} {
		if containsJSONKey(text, forbidden) {
			t.Fatalf("response unexpectedly contains %q: %s", forbidden, text)
		}
	}
}

func TestNewHealthFactsInspectionResponseSupportsAllInspectionTypes(t *testing.T) {
	types := []inspection.Type{
		inspection.TypeRoutine,
		inspection.TypeQueen,
		inspection.TypeBrood,
		inspection.TypeHealth,
		inspection.TypeFeeding,
		inspection.TypeSeasonal,
	}
	for _, typ := range types {
		t.Run(string(typ), func(t *testing.T) {
			got := newHealthFactsInspectionResponse(&inspection.Inspection{
				ID: uuid.New(), HiveID: uuid.New(),
				InspectedAt: time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC),
				Type:        typ,
			})
			if got.Type != typ || got.InspectedAt != "2026-09-23" {
				t.Fatalf("response = %+v", got)
			}
		})
	}
}

func containsJSONKey(payload, key string) bool {
	return len(payload) > 0 && json.Valid([]byte(payload)) && strings.Contains(payload, "\""+key+"\":")
}

func ptrHealth[T any](value T) *T {
	return &value
}

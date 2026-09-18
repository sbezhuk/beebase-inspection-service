package health

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sbezhuk/beebase-inspection-service/internal/domain/inspection"
)

func TestNormalizeInspection_NilAssessmentProducesNoOutputs(t *testing.T) {
	result := NormalizeInspection(inspection.Inspection{Assessment: nil})

	if result.HealthEvidence == nil || result.ManagementEvents == nil || result.ContextFacts == nil {
		t.Fatal("nil assessment should produce initialized empty output slices")
	}
	if len(result.HealthEvidence) != 0 || len(result.ManagementEvents) != 0 || len(result.ContextFacts) != 0 {
		t.Fatalf("nil assessment produced output: %#v", result)
	}
}

func TestNormalizeInspection_AllSchemasPreserveFieldsAndSource(t *testing.T) {
	when := time.Date(2026, 9, 18, 12, 30, 0, 0, time.FixedZone("EET", 2*60*60))
	inspectionID := uuid.New()
	hiveID := uuid.New()

	cases := []struct {
		name       string
		typ        inspection.Type
		assessment *inspection.Assessment
		checks     []evidenceCheck
	}{
		{
			name: "routine",
			typ:  inspection.TypeRoutine,
			assessment: &inspection.Assessment{
				Version:        1,
				ColonyStrength: ptr(inspection.ColonyStrengthStrong),
				QueenStatus:    ptr(inspection.QueenStatusNotChecked),
				BroodStatus:    ptr(inspection.BroodStatusProblem),
				FoodStores:     ptr(inspection.FoodStoresLow),
				HealthConcerns: ptr(inspection.HealthConcernsPresent),
			},
			checks: []evidenceCheck{
				{DimensionStrength, EvidenceObservation, SourceFieldColonyStrength, "STRONG", EvidenceValueRecorded},
				{DimensionQueen, EvidenceBeekeeperAssessment, SourceFieldQueenStatus, "NOT_CHECKED", EvidenceValueRecorded},
				{DimensionBrood, EvidenceBeekeeperAssessment, SourceFieldBroodStatus, "PROBLEM", EvidenceValueRecorded},
				{DimensionNutrition, EvidenceObservation, SourceFieldFoodStores, "LOW", EvidenceValueRecorded},
				{DimensionOverall, EvidenceBeekeeperAssessment, SourceFieldHealthConcerns, "PRESENT", EvidenceValueRecorded},
			},
		},
		{
			name: "queen",
			typ:  inspection.TypeQueen,
			assessment: &inspection.Assessment{
				Version:        1,
				QueenObserved:  ptr(inspection.QueenObservedNotObserved),
				EggsObserved:   ptr(inspection.EggsObservedNo),
				QueenCells:     ptr(inspection.QueenCellsNone),
				QueenCondition: ptr(inspection.QueenConditionConcern),
			},
			checks: []evidenceCheck{
				{DimensionQueen, EvidenceObservation, SourceFieldQueenObserved, "NOT_OBSERVED", EvidenceValueRecorded},
				{DimensionQueen, EvidenceObservation, SourceFieldEggsObserved, "NO", EvidenceValueRecorded},
				{DimensionQueen, EvidenceObservation, SourceFieldQueenCells, "NONE", EvidenceValueRecorded},
				{DimensionQueen, EvidenceBeekeeperAssessment, SourceFieldQueenCondition, "CONCERN", EvidenceValueRecorded},
			},
		},
		{
			name: "brood",
			typ:  inspection.TypeBrood,
			assessment: &inspection.Assessment{
				Version:       1,
				BroodAmount:   ptr(inspection.BroodAmountModerate),
				BroodPattern:  ptr(inspection.BroodPatternSpotty),
				BroodStages:   &[]inspection.BroodStage{inspection.BroodStageEggs, inspection.BroodStageCapped},
				BroodConcerns: ptr(inspection.BroodConcernsUnsure),
			},
			checks: []evidenceCheck{
				{DimensionBrood, EvidenceObservation, SourceFieldBroodAmount, "MODERATE", EvidenceValueRecorded},
				{DimensionBrood, EvidenceObservation, SourceFieldBroodPattern, "SPOTTY", EvidenceValueRecorded},
				{DimensionBrood, EvidenceObservation, SourceFieldBroodStages, "EGGS", EvidenceValueRecorded},
				{DimensionBrood, EvidenceObservation, SourceFieldBroodStages, "CAPPED", EvidenceValueRecorded},
				{DimensionBrood, EvidenceBeekeeperAssessment, SourceFieldBroodConcerns, "UNSURE", EvidenceValueRecorded},
			},
		},
		{
			name: "health",
			typ:  inspection.TypeHealth,
			assessment: &inspection.Assessment{
				Version:                1,
				HealthOverallCondition: ptr(inspection.HealthOverallFair),
				PestSigns:              &[]inspection.PestSign{inspection.PestSignVarroaMites},
				HealthWarningSigns:     &[]inspection.HealthWarningSign{inspection.HealthWarningAbnormalBrood},
				HealthConcernLevel:     ptr(inspection.HealthConcernHigh),
			},
			checks: []evidenceCheck{
				{DimensionOverall, EvidenceBeekeeperAssessment, SourceFieldHealthOverallCondition, "FAIR", EvidenceValueRecorded},
				{DimensionPestsAndDisease, EvidenceObservation, SourceFieldPestSigns, "VARROA_MITES", EvidenceValueRecorded},
				{DimensionPestsAndDisease, EvidenceObservation, SourceFieldHealthWarningSigns, "ABNORMAL_BROOD", EvidenceValueRecorded},
				{DimensionOverall, EvidenceBeekeeperAssessment, SourceFieldHealthConcernLevel, "HIGH", EvidenceValueRecorded},
			},
		},
		{
			name: "feeding",
			typ:  inspection.TypeFeeding,
			assessment: &inspection.Assessment{
				Version:          1,
				FoodStores:       ptr(inspection.FoodStoresAdequate),
				FeedingNeed:      ptr(inspection.FeedingNeedSoon),
				FeedingPerformed: ptr(inspection.FeedingPerformedYes),
				FeedTypes:        &[]inspection.FeedType{inspection.FeedTypeSugarSyrup},
			},
			checks: []evidenceCheck{
				{DimensionNutrition, EvidenceObservation, SourceFieldFoodStores, "ADEQUATE", EvidenceValueRecorded},
				{DimensionNutrition, EvidenceBeekeeperAssessment, SourceFieldFeedingNeed, "SOON", EvidenceValueRecorded},
			},
		},
		{
			name: "seasonal",
			typ:  inspection.TypeSeasonal,
			assessment: &inspection.Assessment{
				Version:                1,
				Season:                 ptr(inspection.SeasonalWinter),
				ColonyStrength:         ptr(inspection.ColonyStrengthModerate),
				SeasonalStoreReadiness: ptr(inspection.SeasonalStoresMarginal),
				SeasonalReadiness:      ptr(inspection.SeasonalReadinessNeedsAttention),
				SeasonalConcerns:       &[]inspection.SeasonalConcern{inspection.SeasonalConcernFoodStores, inspection.SeasonalConcernOther},
			},
			checks: []evidenceCheck{
				{DimensionStrength, EvidenceObservation, SourceFieldColonyStrength, "MODERATE", EvidenceValueRecorded},
				{DimensionNutrition, EvidenceBeekeeperAssessment, SourceFieldSeasonalStoreReadiness, "MARGINAL", EvidenceValueRecorded},
				{DimensionNutrition, EvidenceBeekeeperAssessment, SourceFieldSeasonalConcerns, "FOOD_STORES", EvidenceValueRecorded},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := inspection.Inspection{
				ID:          inspectionID,
				HiveID:      hiveID,
				InspectedAt: when,
				Type:        tc.typ,
				Assessment:  tc.assessment,
			}
			result := NormalizeInspection(input)
			for _, check := range tc.checks {
				assertEvidence(t, result.HealthEvidence, check)
			}
			for _, evidence := range result.HealthEvidence {
				if evidence.Source.HiveID != hiveID || evidence.Source.EntityID != inspectionID || evidence.Source.InspectionID == nil || *evidence.Source.InspectionID != inspectionID || evidence.Source.InspectionType == nil || *evidence.Source.InspectionType != tc.typ || evidence.Source.AssessmentSchema != string(tc.typ) || evidence.Source.AssessmentSchemaVersion != 1 || !evidence.Source.OccurredAt.Equal(when) {
					t.Fatalf("source metadata = %#v", evidence.Source)
				}
			}
			if tc.typ == inspection.TypeFeeding {
				if len(result.HealthEvidence) != 2 || len(result.ManagementEvents) != 1 {
					t.Fatalf("feeding outputs = %#v, want two evidence items and one action", result)
				}
			}
			if tc.typ == inspection.TypeSeasonal {
				if containsEvidenceField(result.HealthEvidence, SourceFieldSeason) || containsEvidenceField(result.HealthEvidence, SourceFieldSeasonalReadiness) {
					t.Fatal("seasonal context was emitted as health evidence")
				}
				if !containsContextValue(result.ContextFacts, ContextSeason, SourceFieldSeason, "WINTER") || !containsContextValue(result.ContextFacts, ContextSeasonalReadiness, SourceFieldSeasonalReadiness, "NEEDS_ATTENTION") || !containsContextValue(result.ContextFacts, ContextSeasonalConcern, SourceFieldSeasonalConcerns, "OTHER") {
					t.Fatal("seasonal context was not preserved")
				}
			}
		})
	}
}

func TestNormalizeInspection_EmptyAndNullMultiSelectsRemainDistinct(t *testing.T) {
	cases := []struct {
		name       string
		assessment *inspection.Assessment
		field      SourceField
		dimension  HealthDimension
	}{
		{"brood null", &inspection.Assessment{}, SourceFieldBroodStages, DimensionBrood},
		{"brood empty", &inspection.Assessment{BroodStages: &[]inspection.BroodStage{}}, SourceFieldBroodStages, DimensionBrood},
		{"pests null", &inspection.Assessment{}, SourceFieldPestSigns, DimensionPestsAndDisease},
		{"pests empty", &inspection.Assessment{PestSigns: &[]inspection.PestSign{}}, SourceFieldPestSigns, DimensionPestsAndDisease},
		{"warnings null", &inspection.Assessment{}, SourceFieldHealthWarningSigns, DimensionPestsAndDisease},
		{"warnings empty", &inspection.Assessment{HealthWarningSigns: &[]inspection.HealthWarningSign{}}, SourceFieldHealthWarningSigns, DimensionPestsAndDisease},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := NormalizeInspection(inspection.Inspection{Type: inspection.TypeHealth, Assessment: tc.assessment})
			if tc.name[len(tc.name)-5:] == "empty" {
				if len(result.HealthEvidence) != 1 || result.HealthEvidence[0].State != EvidenceAssessedEmpty || result.HealthEvidence[0].Source.SourceField != tc.field || result.HealthEvidence[0].Dimension != tc.dimension {
					t.Fatalf("empty output = %#v", result.HealthEvidence)
				}
			} else if len(result.HealthEvidence) != 0 {
				t.Fatalf("null output = %#v", result.HealthEvidence)
			}
		})
	}
}

func TestNormalizeInspection_FeedingActionPreservesNullEmptyAndValues(t *testing.T) {
	performed := inspection.FeedingPerformedYes
	feedTypes := []struct {
		name   string
		values *[]inspection.FeedType
	}{
		{name: "null"},
		{name: "empty", values: &[]inspection.FeedType{}},
		{name: "populated", values: &[]inspection.FeedType{inspection.FeedTypeSugarSyrup, inspection.FeedTypeFondant}},
	}
	for _, tc := range feedTypes {
		t.Run(tc.name, func(t *testing.T) {
			result := NormalizeInspection(inspection.Inspection{
				Type: inspection.TypeFeeding,
				Assessment: &inspection.Assessment{
					FeedingPerformed: &performed,
					FeedTypes:        tc.values,
				},
			})
			if len(result.HealthEvidence) != 0 || len(result.ManagementEvents) != 1 {
				t.Fatalf("feeding action output = %#v", result)
			}
			event := result.ManagementEvents[0]
			if event.Action != ActionFeedingPerformed || event.Value == nil || *event.Value != "YES" {
				t.Fatalf("feeding performed event = %#v", event)
			}
			if tc.values == nil {
				if event.RelatedValues != nil {
					t.Fatalf("null feed types became %#v", event.RelatedValues)
				}
			} else if event.RelatedValues == nil || len(*event.RelatedValues) != len(*tc.values) {
				t.Fatalf("feed types = %#v, want length %d", event.RelatedValues, len(*tc.values))
			}
		})
	}
}

func TestNormalizeInspection_SeasonalUnmappedConcernsRemainTraceable(t *testing.T) {
	concerns := []inspection.SeasonalConcern{inspection.SeasonalConcernHiveCondition, inspection.SeasonalConcernOther}
	result := NormalizeInspection(inspection.Inspection{
		Type: inspection.TypeSeasonal,
		Assessment: &inspection.Assessment{
			SeasonalConcerns: &concerns,
		},
	})

	if len(result.HealthEvidence) != 0 || !containsContextValue(result.ContextFacts, ContextSeasonalConcern, SourceFieldSeasonalConcerns, "HIVE_CONDITION") || !containsContextValue(result.ContextFacts, ContextSeasonalConcern, SourceFieldSeasonalConcerns, "OTHER") {
		t.Fatalf("unmapped concerns lost or misclassified: %#v", result)
	}
}

func TestNormalizeInspection_SeasonalEmptyConcernsArePreserved(t *testing.T) {
	concerns := []inspection.SeasonalConcern{}
	result := NormalizeInspection(inspection.Inspection{
		Type:       inspection.TypeSeasonal,
		Assessment: &inspection.Assessment{SeasonalConcerns: &concerns},
	})

	if len(result.ContextFacts) != 1 || result.ContextFacts[0].Kind != ContextSeasonalConcern || result.ContextFacts[0].State != EvidenceAssessedEmpty || result.ContextFacts[0].Value != nil {
		t.Fatalf("empty seasonal concerns = %#v", result.ContextFacts)
	}
}

type evidenceCheck struct {
	dimension HealthDimension
	kind      EvidenceKind
	field     SourceField
	value     string
	state     EvidenceState
}

func assertEvidence(t *testing.T, evidence []HealthEvidence, check evidenceCheck) {
	t.Helper()
	for _, item := range evidence {
		if item.Dimension == check.dimension && item.Kind == check.kind && item.State == check.state && item.Source.SourceField == check.field && item.Value != nil && *item.Value == check.value {
			return
		}
	}
	t.Fatalf("missing evidence %#v in %#v", check, evidence)
}

func containsEvidenceField(evidence []HealthEvidence, field SourceField) bool {
	for _, item := range evidence {
		if item.Source.SourceField == field {
			return true
		}
	}
	return false
}

func containsContextValue(facts []ContextFact, kind ContextKind, field SourceField, value string) bool {
	for _, fact := range facts {
		if fact.Kind == kind && fact.Source.SourceField == field && fact.Value != nil && *fact.Value == value {
			return true
		}
	}
	return false
}

func ptr[T any](value T) *T { return &value }

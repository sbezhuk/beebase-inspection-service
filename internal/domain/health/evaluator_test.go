package health

import (
	"reflect"
	"testing"
	"time"
)

func TestEvaluateDimensionsReturnsAllDimensionsInOrder(t *testing.T) {
	input := DimensionEvaluationInput{AsOf: evaluationTime(), RecencyPolicy: DefaultRecencyPolicyV1()}
	got, err := EvaluateDimensions(input)
	if err != nil {
		t.Fatalf("EvaluateDimensions() error = %v", err)
	}
	if len(got) != len(evaluationDimensions) {
		t.Fatalf("evaluation count = %d, want %d", len(got), len(evaluationDimensions))
	}
	for index, dimension := range evaluationDimensions {
		if got[index].Dimension != dimension {
			t.Errorf("evaluation[%d].Dimension = %q, want %q", index, got[index].Dimension, dimension)
		}
		if got[index].State != DimensionUnknown || got[index].Coverage != CoverageNone {
			t.Errorf("empty %s = (%q, %q), want (UNKNOWN, NONE)", dimension, got[index].State, got[index].Coverage)
		}
	}
}

func TestEvaluateDimensionsInterpretsNormalizedValues(t *testing.T) {
	when := evaluationTime().Add(-time.Hour)
	tests := []struct {
		name      string
		dimension HealthDimension
		field     SourceField
		value     string
		wantState DimensionState
	}{
		{"weak strength", DimensionStrength, SourceFieldColonyStrength, "WEAK", DimensionConcern},
		{"moderate strength", DimensionStrength, SourceFieldColonyStrength, "MODERATE", DimensionGood},
		{"strong strength", DimensionStrength, SourceFieldColonyStrength, "STRONG", DimensionGood},
		{"queen healthy", DimensionQueen, SourceFieldQueenStatus, "HEALTHY", DimensionGood},
		{"queen problem", DimensionQueen, SourceFieldQueenStatus, "PROBLEM", DimensionConcern},
		{"queen not checked", DimensionQueen, SourceFieldQueenStatus, "NOT_CHECKED", DimensionUnknown},
		{"queen observed", DimensionQueen, SourceFieldQueenObserved, "OBSERVED", DimensionUnknown},
		{"queen not observed", DimensionQueen, SourceFieldQueenObserved, "NOT_OBSERVED", DimensionUnknown},
		{"queen unsure", DimensionQueen, SourceFieldQueenObserved, "UNSURE", DimensionUnknown},
		{"eggs yes", DimensionQueen, SourceFieldEggsObserved, "YES", DimensionGood},
		{"eggs no", DimensionQueen, SourceFieldEggsObserved, "NO", DimensionWatch},
		{"eggs unsure", DimensionQueen, SourceFieldEggsObserved, "UNSURE", DimensionUnknown},
		{"queen cells none", DimensionQueen, SourceFieldQueenCells, "NONE", DimensionGood},
		{"queen cells present", DimensionQueen, SourceFieldQueenCells, "PRESENT", DimensionWatch},
		{"queen cells unsure", DimensionQueen, SourceFieldQueenCells, "UNSURE", DimensionUnknown},
		{"queen normal", DimensionQueen, SourceFieldQueenCondition, "NORMAL", DimensionGood},
		{"queen concern", DimensionQueen, SourceFieldQueenCondition, "CONCERN", DimensionConcern},
		{"brood healthy", DimensionBrood, SourceFieldBroodStatus, "HEALTHY", DimensionGood},
		{"brood problem", DimensionBrood, SourceFieldBroodStatus, "PROBLEM", DimensionConcern},
		{"brood not checked", DimensionBrood, SourceFieldBroodStatus, "NOT_CHECKED", DimensionUnknown},
		{"brood low", DimensionBrood, SourceFieldBroodAmount, "LOW", DimensionWatch},
		{"brood moderate", DimensionBrood, SourceFieldBroodAmount, "MODERATE", DimensionGood},
		{"brood high", DimensionBrood, SourceFieldBroodAmount, "HIGH", DimensionGood},
		{"brood solid", DimensionBrood, SourceFieldBroodPattern, "SOLID", DimensionGood},
		{"brood mixed", DimensionBrood, SourceFieldBroodPattern, "MIXED", DimensionWatch},
		{"brood spotty", DimensionBrood, SourceFieldBroodPattern, "SPOTTY", DimensionWatch},
		{"brood stage", DimensionBrood, SourceFieldBroodStages, "EGGS", DimensionGood},
		{"brood larvae", DimensionBrood, SourceFieldBroodStages, "LARVAE", DimensionGood},
		{"brood capped", DimensionBrood, SourceFieldBroodStages, "CAPPED", DimensionGood},
		{"brood concerns none", DimensionBrood, SourceFieldBroodConcerns, "NONE", DimensionGood},
		{"brood concerns observed", DimensionBrood, SourceFieldBroodConcerns, "OBSERVED", DimensionConcern},
		{"brood concerns unsure", DimensionBrood, SourceFieldBroodConcerns, "UNSURE", DimensionUnknown},
		{"low stores", DimensionNutrition, SourceFieldFoodStores, "LOW", DimensionConcern},
		{"adequate stores", DimensionNutrition, SourceFieldFoodStores, "ADEQUATE", DimensionGood},
		{"abundant stores", DimensionNutrition, SourceFieldFoodStores, "ABUNDANT", DimensionGood},
		{"stores not checked", DimensionNutrition, SourceFieldFoodStores, "NOT_CHECKED", DimensionUnknown},
		{"feeding no", DimensionNutrition, SourceFieldFeedingNeed, "NO", DimensionGood},
		{"feeding soon", DimensionNutrition, SourceFieldFeedingNeed, "SOON", DimensionWatch},
		{"feeding yes", DimensionNutrition, SourceFieldFeedingNeed, "YES", DimensionConcern},
		{"feeding unsure", DimensionNutrition, SourceFieldFeedingNeed, "UNSURE", DimensionUnknown},
		{"health concerns none", DimensionOverall, SourceFieldHealthConcerns, "NONE", DimensionGood},
		{"health concerns present", DimensionOverall, SourceFieldHealthConcerns, "PRESENT", DimensionConcern},
		{"health concerns not checked", DimensionOverall, SourceFieldHealthConcerns, "NOT_CHECKED", DimensionUnknown},
		{"sufficient seasonal stores", DimensionNutrition, SourceFieldSeasonalStoreReadiness, "SUFFICIENT", DimensionGood},
		{"marginal seasonal stores", DimensionNutrition, SourceFieldSeasonalStoreReadiness, "MARGINAL", DimensionWatch},
		{"insufficient seasonal stores", DimensionNutrition, SourceFieldSeasonalStoreReadiness, "INSUFFICIENT", DimensionConcern},
		{"varroa sign", DimensionPestsAndDisease, SourceFieldPestSigns, "VARROA_MITES", DimensionConcern},
		{"wax moth sign", DimensionPestsAndDisease, SourceFieldPestSigns, "WAX_MOTH", DimensionConcern},
		{"small hive beetle sign", DimensionPestsAndDisease, SourceFieldPestSigns, "SMALL_HIVE_BEETLE", DimensionConcern},
		{"other pest sign", DimensionPestsAndDisease, SourceFieldPestSigns, "OTHER", DimensionConcern},
		{"warning sign", DimensionPestsAndDisease, SourceFieldHealthWarningSigns, "ABNORMAL_BROOD", DimensionWatch},
		{"deformed wings warning", DimensionPestsAndDisease, SourceFieldHealthWarningSigns, "DEFORMED_WINGS", DimensionWatch},
		{"mortality warning", DimensionPestsAndDisease, SourceFieldHealthWarningSigns, "UNUSUAL_BEE_MORTALITY", DimensionWatch},
		{"diarrhea warning", DimensionPestsAndDisease, SourceFieldHealthWarningSigns, "DIARRHEA_SIGNS", DimensionWatch},
		{"other warning", DimensionPestsAndDisease, SourceFieldHealthWarningSigns, "OTHER", DimensionWatch},
		{"overall good", DimensionOverall, SourceFieldHealthOverallCondition, "GOOD", DimensionGood},
		{"overall fair", DimensionOverall, SourceFieldHealthOverallCondition, "FAIR", DimensionWatch},
		{"overall poor", DimensionOverall, SourceFieldHealthOverallCondition, "POOR", DimensionConcern},
		{"overall none", DimensionOverall, SourceFieldHealthConcernLevel, "NONE", DimensionGood},
		{"overall low", DimensionOverall, SourceFieldHealthConcernLevel, "LOW", DimensionWatch},
		{"overall moderate", DimensionOverall, SourceFieldHealthConcernLevel, "MODERATE", DimensionWatch},
		{"overall high", DimensionOverall, SourceFieldHealthConcernLevel, "HIGH", DimensionConcern},
		{"seasonal strength concern", DimensionStrength, SourceFieldSeasonalConcerns, "COLONY_STRENGTH", DimensionWatch},
		{"seasonal queen concern", DimensionQueen, SourceFieldSeasonalConcerns, "QUEEN", DimensionWatch},
		{"seasonal brood concern", DimensionBrood, SourceFieldSeasonalConcerns, "BROOD", DimensionWatch},
		{"seasonal food concern", DimensionNutrition, SourceFieldSeasonalConcerns, "FOOD_STORES", DimensionWatch},
		{"seasonal pest concern", DimensionPestsAndDisease, SourceFieldSeasonalConcerns, "PESTS_OR_DISEASE", DimensionWatch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluateOne(t, evidenceValue(tt.dimension, tt.field, tt.value, when))
			if got.State != tt.wantState {
				t.Fatalf("state = %q, want %q; evaluation = %#v", got.State, tt.wantState, got)
			}
			if len(got.ContributingEvidence) != 1 {
				t.Fatalf("contributing evidence = %d, want 1", len(got.ContributingEvidence))
			}
		})
	}
}

func TestEvaluateDimensionsUnknownAndEmptySemantics(t *testing.T) {
	when := evaluationTime().Add(-time.Hour)
	tests := []struct {
		name      string
		evidence  HealthEvidence
		wantState DimensionState
		wantCover EvidenceCoverage
	}{
		{"not checked", evidenceValue(DimensionQueen, SourceFieldQueenStatus, "NOT_CHECKED", when), DimensionUnknown, CoverageLow},
		{"queen not observed", evidenceValue(DimensionQueen, SourceFieldQueenObserved, "NOT_OBSERVED", when), DimensionUnknown, CoverageLow},
		{"queen unsure", evidenceValue(DimensionQueen, SourceFieldEggsObserved, "UNSURE", when), DimensionUnknown, CoverageLow},
		{"empty brood stages", evidenceEmpty(DimensionBrood, SourceFieldBroodStages, when), DimensionUnknown, CoverageMedium},
		{"empty pest signs", evidenceEmpty(DimensionPestsAndDisease, SourceFieldPestSigns, when), DimensionGood, CoverageMedium},
		{"empty warning signs", evidenceEmpty(DimensionPestsAndDisease, SourceFieldHealthWarningSigns, when), DimensionGood, CoverageMedium},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluateOne(t, tt.evidence)
			if got.State != tt.wantState || got.Coverage != tt.wantCover {
				t.Fatalf("evaluation = (%q, %q), want (%q, %q)", got.State, got.Coverage, tt.wantState, tt.wantCover)
			}
		})
	}
}

func TestEvaluateDimensionsCoverageUsesDistinctFactsAndRecency(t *testing.T) {
	asOf := evaluationTime()
	current := asOf.Add(-time.Hour)
	recent := asOf.Add(-20 * 24 * time.Hour)
	stale := asOf.Add(-31 * 24 * time.Hour)

	tests := []struct {
		name     string
		evidence []HealthEvidence
		want     EvidenceCoverage
	}{
		{"one current fact", []HealthEvidence{evidenceValue(DimensionQueen, SourceFieldQueenStatus, "HEALTHY", current)}, CoverageMedium},
		{"two current facts", []HealthEvidence{
			evidenceValue(DimensionQueen, SourceFieldQueenStatus, "HEALTHY", current),
			evidenceValue(DimensionQueen, SourceFieldQueenCells, "NONE", current),
		}, CoverageHigh},
		{"recent only", []HealthEvidence{evidenceValue(DimensionQueen, SourceFieldQueenStatus, "HEALTHY", recent)}, CoverageMedium},
		{"stale only", []HealthEvidence{evidenceValue(DimensionQueen, SourceFieldQueenStatus, "HEALTHY", stale)}, CoverageNone},
		{"repeated same fact", []HealthEvidence{
			evidenceValue(DimensionQueen, SourceFieldQueenStatus, "HEALTHY", current),
			evidenceValue(DimensionQueen, SourceFieldQueenStatus, "HEALTHY", current.Add(-time.Minute)),
		}, CoverageMedium},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluateAt(t, asOf, tt.evidence...)
			queen := evaluationFor(got, DimensionQueen)
			if queen.Coverage != tt.want {
				t.Fatalf("coverage = %q, want %q", queen.Coverage, tt.want)
			}
		})
	}
}

// TestEvaluateDimensionsStaleOnlyCoverageIsNone proves that once all
// meaningful evidence for a dimension has aged past STALE, coverage drops to
// NONE rather than LOW: stale evidence explains history, not current state,
// so it must not report as "some usable evidence" either.
func TestEvaluateDimensionsStaleOnlyCoverageIsNone(t *testing.T) {
	asOf := evaluationTime()

	t.Run("very old stale evidence (2 years)", func(t *testing.T) {
		got := evaluateAt(t, asOf, evidenceValue(DimensionNutrition, SourceFieldFoodStores, "ADEQUATE", asOf.AddDate(-2, 0, 0)))
		nutrition := evaluationFor(got, DimensionNutrition)
		if nutrition.State != DimensionUnknown || nutrition.Coverage != CoverageNone {
			t.Fatalf("nutrition = (%q, %q), want (UNKNOWN, NONE)", nutrition.State, nutrition.Coverage)
		}
	})

	t.Run("31 days is stale", func(t *testing.T) {
		got := evaluateAt(t, asOf, evidenceValue(DimensionNutrition, SourceFieldFoodStores, "ADEQUATE", asOf.Add(-31*24*time.Hour)))
		nutrition := evaluationFor(got, DimensionNutrition)
		if nutrition.State != DimensionUnknown || nutrition.Coverage != CoverageNone {
			t.Fatalf("nutrition = (%q, %q), want (UNKNOWN, NONE)", nutrition.State, nutrition.Coverage)
		}
	})

	t.Run("exactly 30 days is still recent, not stale", func(t *testing.T) {
		got := evaluateAt(t, asOf, evidenceValue(DimensionNutrition, SourceFieldFoodStores, "ADEQUATE", asOf.Add(-30*24*time.Hour)))
		nutrition := evaluationFor(got, DimensionNutrition)
		if nutrition.State != DimensionGood || nutrition.Coverage != CoverageMedium {
			t.Fatalf("nutrition = (%q, %q), want (GOOD, MEDIUM)", nutrition.State, nutrition.Coverage)
		}
	})

	t.Run("30 days plus one second crosses into stale", func(t *testing.T) {
		got := evaluateAt(t, asOf, evidenceValue(DimensionNutrition, SourceFieldFoodStores, "ADEQUATE", asOf.Add(-30*24*time.Hour-time.Second)))
		nutrition := evaluationFor(got, DimensionNutrition)
		if nutrition.State != DimensionUnknown || nutrition.Coverage != CoverageNone {
			t.Fatalf("nutrition = (%q, %q), want (UNKNOWN, NONE)", nutrition.State, nutrition.Coverage)
		}
	})

	t.Run("stale evidence does not inflate coverage alongside a current fact", func(t *testing.T) {
		stale := evidenceValue(DimensionNutrition, SourceFieldFoodStores, "ADEQUATE", asOf.Add(-40*24*time.Hour))
		current := evidenceValue(DimensionNutrition, SourceFieldFeedingNeed, "NO", asOf.Add(-5*24*time.Hour))
		got := evaluateAt(t, asOf, stale, current)
		nutrition := evaluationFor(got, DimensionNutrition)
		if nutrition.Coverage != CoverageMedium {
			t.Fatalf("coverage = %q, want MEDIUM (one usable current fact; stale fact must not add coverage)", nutrition.Coverage)
		}
	})

	t.Run("stale evidence does not inflate coverage alongside a recent fact", func(t *testing.T) {
		stale := evidenceValue(DimensionNutrition, SourceFieldFoodStores, "ADEQUATE", asOf.Add(-40*24*time.Hour))
		recent := evidenceValue(DimensionNutrition, SourceFieldFeedingNeed, "NO", asOf.Add(-20*24*time.Hour))
		got := evaluateAt(t, asOf, stale, recent)
		nutrition := evaluationFor(got, DimensionNutrition)
		if nutrition.Coverage != CoverageMedium {
			t.Fatalf("coverage = %q, want MEDIUM (usable recent fact; stale fact must not add coverage)", nutrition.Coverage)
		}
	})
}

func TestEvaluateDimensionsTimelineAndManagementEvents(t *testing.T) {
	day1 := evaluationTime()
	day2 := day1.Add(24 * time.Hour)
	day8 := day1.Add(7 * 24 * time.Hour)
	low := evidenceValue(DimensionNutrition, SourceFieldFoodStores, "LOW", day1)
	adequate := evidenceValue(DimensionNutrition, SourceFieldFoodStores, "ADEQUATE", day8)

	for _, tc := range []struct {
		name      string
		asOf      time.Time
		wantState DimensionState
	}{
		{"day one", day1, DimensionConcern},
		{"day two action only", day2, DimensionConcern},
		{"day eight replacement", day8, DimensionGood},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input := DimensionEvaluationInput{
				Evidence:      []HealthEvidence{low, adequate},
				AsOf:          tc.asOf,
				RecencyPolicy: DefaultRecencyPolicyV1(),
				ManagementEvents: []ManagementEvent{{
					Action: ActionFeedingPerformed,
					Value:  stringPointer("YES"),
					Source: EvidenceSource{OccurredAt: day2},
				}},
			}
			got, err := EvaluateDimensions(input)
			if err != nil {
				t.Fatalf("EvaluateDimensions() error = %v", err)
			}
			if nutrition := evaluationFor(got, DimensionNutrition); nutrition.State != tc.wantState {
				t.Fatalf("nutrition state = %q, want %q", nutrition.State, tc.wantState)
			}
		})
	}
}

func TestEvaluateDimensionsConflictsAndPrimaryEvidence(t *testing.T) {
	when := evaluationTime().Add(-time.Hour)
	tests := []struct {
		name      string
		evidence  []HealthEvidence
		wantState DimensionState
	}{
		{"same fact conflict", []HealthEvidence{
			evidenceValue(DimensionOverall, SourceFieldHealthOverallCondition, "GOOD", when),
			evidenceValue(DimensionOverall, SourceFieldHealthOverallCondition, "POOR", when),
		}, DimensionWatch},
		{"direct strength concern not overridden by brood support", []HealthEvidence{
			evidenceValue(DimensionStrength, SourceFieldColonyStrength, "WEAK", when),
			evidenceValue(DimensionStrength, SourceFieldBroodAmount, "MODERATE", when),
		}, DimensionConcern},
		{"direct concern and direct favorable conflict", []HealthEvidence{
			evidenceValue(DimensionQueen, SourceFieldQueenStatus, "PROBLEM", when),
			evidenceValue(DimensionQueen, SourceFieldQueenCondition, "NORMAL", when),
		}, DimensionWatch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := evaluateOne(t, tt.evidence...)
			if got.State != tt.wantState {
				t.Fatalf("state = %q, want %q", got.State, tt.wantState)
			}
		})
	}
}

func TestEvaluateDimensionsExcludesFutureAndIsOrderIndependent(t *testing.T) {
	asOf := evaluationTime()
	evidence := []HealthEvidence{
		evidenceValue(DimensionQueen, SourceFieldQueenCells, "NONE", asOf.Add(-time.Hour)),
		evidenceValue(DimensionQueen, SourceFieldQueenStatus, "HEALTHY", asOf.Add(-time.Hour)),
		evidenceValue(DimensionQueen, SourceFieldQueenStatus, "PROBLEM", asOf.Add(time.Hour)),
	}
	first, err := EvaluateDimensions(DimensionEvaluationInput{Evidence: evidence, AsOf: asOf, RecencyPolicy: DefaultRecencyPolicyV1()})
	if err != nil {
		t.Fatalf("first evaluation error = %v", err)
	}
	reversed := append([]HealthEvidence(nil), evidence...)
	for left, right := 0, len(reversed)-1; left < right; left, right = left+1, right-1 {
		reversed[left], reversed[right] = reversed[right], reversed[left]
	}
	second, err := EvaluateDimensions(DimensionEvaluationInput{Evidence: reversed, AsOf: asOf, RecencyPolicy: DefaultRecencyPolicyV1()})
	if err != nil {
		t.Fatalf("second evaluation error = %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("order changed result:\nfirst = %#v\nsecond = %#v", first, second)
	}
	queen := evaluationFor(first, DimensionQueen)
	if queen.State != DimensionGood || len(queen.ContributingEvidence) != 2 {
		t.Fatalf("future evidence affected result: %#v", queen)
	}
}

func TestEvaluateDimensionsRejectsUnsupportedNormalizedValue(t *testing.T) {
	_, err := EvaluateDimensions(DimensionEvaluationInput{
		Evidence:      []HealthEvidence{evidenceValue(DimensionQueen, SourceFieldQueenStatus, "INVALID", evaluationTime())},
		AsOf:          evaluationTime(),
		RecencyPolicy: DefaultRecencyPolicyV1(),
	})
	if err == nil {
		t.Fatal("EvaluateDimensions() error = nil, want unsupported value error")
	}
}

func evaluateOne(t *testing.T, evidence ...HealthEvidence) DimensionEvaluation {
	t.Helper()
	return evaluationFor(evaluateAt(t, evaluationTime(), evidence...), evidence[0].Dimension)
}

func evaluateAt(t *testing.T, asOf time.Time, evidence ...HealthEvidence) []DimensionEvaluation {
	t.Helper()
	got, err := EvaluateDimensions(DimensionEvaluationInput{Evidence: evidence, AsOf: asOf, RecencyPolicy: DefaultRecencyPolicyV1()})
	if err != nil {
		t.Fatalf("EvaluateDimensions() error = %v", err)
	}
	return got
}

func evaluationFor(evaluations []DimensionEvaluation, dimension HealthDimension) DimensionEvaluation {
	for _, evaluation := range evaluations {
		if evaluation.Dimension == dimension {
			return evaluation
		}
	}
	panic("missing dimension evaluation")
}

func evidenceValue(dimension HealthDimension, field SourceField, value string, occurredAt time.Time) HealthEvidence {
	return HealthEvidence{
		Dimension: dimension,
		State:     EvidenceValueRecorded,
		Source: EvidenceSource{
			OccurredAt:  occurredAt,
			SourceField: field,
		},
		Value: stringPointer(value),
	}
}

func evidenceEmpty(dimension HealthDimension, field SourceField, occurredAt time.Time) HealthEvidence {
	return HealthEvidence{
		Dimension: dimension,
		State:     EvidenceAssessedEmpty,
		Source: EvidenceSource{
			OccurredAt:  occurredAt,
			SourceField: field,
		},
	}
}

func stringPointer(value string) *string {
	return &value
}

func evaluationTime() time.Time {
	return time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC)
}

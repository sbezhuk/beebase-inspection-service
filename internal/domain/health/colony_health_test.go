package health

import (
	"reflect"
	"testing"
)

func TestEvaluateColonyHealthUnknownComponents(t *testing.T) {
	got, err := EvaluateColonyHealth(allDimensionEvaluations(func(dimension HealthDimension) DimensionEvaluation {
		return DimensionEvaluation{Dimension: dimension, State: DimensionUnknown, Coverage: CoverageNone}
	}))
	if err != nil {
		t.Fatalf("EvaluateColonyHealth() error = %v", err)
	}
	if got.State != DimensionUnknown || got.Coverage != CoverageNone {
		t.Fatalf("result = (%q, %q), want (UNKNOWN, NONE)", got.State, got.Coverage)
	}
}

func TestEvaluateColonyHealthRequiresThreeMeaningfulGoodComponents(t *testing.T) {
	tests := []struct {
		name      string
		states    map[HealthDimension]DimensionState
		wantState DimensionState
		wantCover EvidenceCoverage
	}{
		{
			name: "one good is not good",
			states: map[HealthDimension]DimensionState{
				DimensionStrength: DimensionGood,
			},
			wantState: DimensionWatch,
			wantCover: CoverageLow,
		},
		{
			name: "two good are not good",
			states: map[HealthDimension]DimensionState{
				DimensionStrength: DimensionGood,
				DimensionQueen:    DimensionGood,
			},
			wantState: DimensionWatch,
			wantCover: CoverageMedium,
		},
		{
			name: "three good are sufficient",
			states: map[HealthDimension]DimensionState{
				DimensionStrength: DimensionGood,
				DimensionQueen:    DimensionGood,
				DimensionBrood:    DimensionGood,
			},
			wantState: DimensionGood,
			wantCover: CoverageMedium,
		},
		{
			name: "four good have high coverage",
			states: map[HealthDimension]DimensionState{
				DimensionStrength:  DimensionGood,
				DimensionQueen:     DimensionGood,
				DimensionBrood:     DimensionGood,
				DimensionNutrition: DimensionGood,
			},
			wantState: DimensionGood,
			wantCover: CoverageHigh,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EvaluateColonyHealth(componentEvaluations(tt.states))
			if err != nil {
				t.Fatalf("EvaluateColonyHealth() error = %v", err)
			}
			if got.State != tt.wantState || got.Coverage != tt.wantCover {
				t.Fatalf("result = (%q, %q), want (%q, %q)", got.State, got.Coverage, tt.wantState, tt.wantCover)
			}
		})
	}
}

func TestEvaluateColonyHealthConcernRules(t *testing.T) {
	tests := []struct {
		name      string
		state     DimensionState
		coverage  EvidenceCoverage
		wantState DimensionState
	}{
		{name: "medium concern", state: DimensionConcern, coverage: CoverageMedium, wantState: DimensionConcern},
		{name: "high concern", state: DimensionConcern, coverage: CoverageHigh, wantState: DimensionConcern},
		{name: "low concern", state: DimensionConcern, coverage: CoverageLow, wantState: DimensionWatch},
		{name: "watch", state: DimensionWatch, coverage: CoverageMedium, wantState: DimensionWatch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EvaluateColonyHealth(componentEvaluations(map[HealthDimension]DimensionState{DimensionNutrition: tt.state}, tt.coverage))
			if err != nil {
				t.Fatalf("EvaluateColonyHealth() error = %v", err)
			}
			if got.State != tt.wantState {
				t.Fatalf("state = %q, want %q", got.State, tt.wantState)
			}
		})
	}
}

func TestEvaluateColonyHealthMultipleConcernsRemainCategorical(t *testing.T) {
	states := map[HealthDimension]DimensionState{
		DimensionStrength:        DimensionConcern,
		DimensionQueen:           DimensionConcern,
		DimensionBrood:           DimensionConcern,
		DimensionNutrition:       DimensionConcern,
		DimensionPestsAndDisease: DimensionConcern,
	}
	got, err := EvaluateColonyHealth(componentEvaluations(states, CoverageHigh))
	if err != nil {
		t.Fatalf("EvaluateColonyHealth() error = %v", err)
	}
	if got.State != DimensionConcern || got.Coverage != CoverageHigh {
		t.Fatalf("result = (%q, %q), want (CONCERN, HIGH)", got.State, got.Coverage)
	}
}

func TestEvaluateColonyHealthBeekeeperOverallIsIndependent(t *testing.T) {
	componentGood := map[HealthDimension]DimensionState{
		DimensionStrength:        DimensionGood,
		DimensionQueen:           DimensionGood,
		DimensionBrood:           DimensionGood,
		DimensionNutrition:       DimensionGood,
		DimensionPestsAndDisease: DimensionGood,
	}
	withConcern, err := EvaluateColonyHealth(allDimensionEvaluations(func(dimension HealthDimension) DimensionEvaluation {
		state := DimensionUnknown
		coverage := CoverageNone
		if dimension == DimensionOverall {
			state, coverage = DimensionConcern, CoverageHigh
		} else {
			state, coverage = componentGood[dimension], CoverageMedium
		}
		return DimensionEvaluation{Dimension: dimension, State: state, Coverage: coverage}
	}))
	if err != nil {
		t.Fatalf("beekeeper concern error = %v", err)
	}
	if withConcern.State != DimensionGood {
		t.Fatalf("beekeeper concern changed aggregate state to %q", withConcern.State)
	}

	componentConcern := componentGood
	componentConcern[DimensionNutrition] = DimensionConcern
	withGood, err := EvaluateColonyHealth(allDimensionEvaluations(func(dimension HealthDimension) DimensionEvaluation {
		state := DimensionUnknown
		coverage := CoverageNone
		if dimension == DimensionOverall {
			state, coverage = DimensionGood, CoverageHigh
		} else {
			state, coverage = componentConcern[dimension], CoverageMedium
		}
		return DimensionEvaluation{Dimension: dimension, State: state, Coverage: coverage}
	}))
	if err != nil {
		t.Fatalf("beekeeper good error = %v", err)
	}
	if withGood.State != DimensionConcern {
		t.Fatalf("beekeeper good suppressed aggregate state %q", withGood.State)
	}
}

func TestEvaluateColonyHealthRetainsAllDimensionsInCanonicalOrder(t *testing.T) {
	input := []DimensionEvaluation{
		{Dimension: DimensionOverall, State: DimensionGood, Coverage: CoverageMedium},
		{Dimension: DimensionNutrition, State: DimensionGood, Coverage: CoverageMedium},
		{Dimension: DimensionStrength, State: DimensionGood, Coverage: CoverageMedium},
		{Dimension: DimensionPestsAndDisease, State: DimensionGood, Coverage: CoverageMedium},
		{Dimension: DimensionBrood, State: DimensionGood, Coverage: CoverageMedium},
		{Dimension: DimensionQueen, State: DimensionGood, Coverage: CoverageMedium},
	}
	got, err := EvaluateColonyHealth(input)
	if err != nil {
		t.Fatalf("EvaluateColonyHealth() error = %v", err)
	}
	for index, dimension := range evaluationDimensions {
		if got.Dimensions[index].Dimension != dimension {
			t.Errorf("dimension[%d] = %q, want %q", index, got.Dimensions[index].Dimension, dimension)
		}
	}
}

func TestEvaluateColonyHealthRejectsMalformedDimensionSets(t *testing.T) {
	valid := allDimensionEvaluations(func(dimension HealthDimension) DimensionEvaluation {
		return DimensionEvaluation{Dimension: dimension, State: DimensionUnknown, Coverage: CoverageNone}
	})
	tests := []struct {
		name  string
		input []DimensionEvaluation
	}{
		{name: "missing", input: valid[:len(valid)-1]},
		{name: "duplicate", input: append(append([]DimensionEvaluation(nil), valid[:len(valid)-1]...), valid[0])},
		{name: "unknown dimension", input: replaceDimension(valid, HealthDimension("INVALID"))},
		{name: "invalid state", input: replaceState(valid, DimensionState("INVALID"))},
		{name: "invalid coverage", input: replaceCoverage(valid, EvidenceCoverage("INVALID"))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := EvaluateColonyHealth(tt.input); err == nil {
				t.Fatal("EvaluateColonyHealth() error = nil, want validation error")
			}
		})
	}
}

func TestEvaluateColonyHealthInputOrderDoesNotMatter(t *testing.T) {
	input := componentEvaluations(map[HealthDimension]DimensionState{
		DimensionStrength:        DimensionGood,
		DimensionQueen:           DimensionGood,
		DimensionBrood:           DimensionWatch,
		DimensionNutrition:       DimensionUnknown,
		DimensionPestsAndDisease: DimensionUnknown,
	})
	reversed := append([]DimensionEvaluation(nil), input...)
	for left, right := 0, len(reversed)-1; left < right; left, right = left+1, right-1 {
		reversed[left], reversed[right] = reversed[right], reversed[left]
	}
	first, err := EvaluateColonyHealth(input)
	if err != nil {
		t.Fatalf("first evaluation error = %v", err)
	}
	second, err := EvaluateColonyHealth(reversed)
	if err != nil {
		t.Fatalf("second evaluation error = %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("input order changed result:\nfirst = %#v\nsecond = %#v", first, second)
	}
}

func componentEvaluations(states map[HealthDimension]DimensionState, coverage ...EvidenceCoverage) []DimensionEvaluation {
	selectedCoverage := CoverageMedium
	if len(coverage) > 0 {
		selectedCoverage = coverage[0]
	}
	return allDimensionEvaluations(func(dimension HealthDimension) DimensionEvaluation {
		state := states[dimension]
		if state == "" {
			state = DimensionUnknown
		}
		currentCoverage := CoverageNone
		if state != DimensionUnknown {
			currentCoverage = selectedCoverage
		}
		return DimensionEvaluation{Dimension: dimension, State: state, Coverage: currentCoverage}
	})
}

func allDimensionEvaluations(factory func(HealthDimension) DimensionEvaluation) []DimensionEvaluation {
	result := make([]DimensionEvaluation, 0, len(evaluationDimensions))
	for _, dimension := range evaluationDimensions {
		result = append(result, factory(dimension))
	}
	return result
}

func replaceDimension(input []DimensionEvaluation, dimension HealthDimension) []DimensionEvaluation {
	result := append([]DimensionEvaluation(nil), input...)
	result[0].Dimension = dimension
	return result
}

func replaceState(input []DimensionEvaluation, state DimensionState) []DimensionEvaluation {
	result := append([]DimensionEvaluation(nil), input...)
	result[0].State = state
	return result
}

func replaceCoverage(input []DimensionEvaluation, coverage EvidenceCoverage) []DimensionEvaluation {
	result := append([]DimensionEvaluation(nil), input...)
	result[0].Coverage = coverage
	return result
}

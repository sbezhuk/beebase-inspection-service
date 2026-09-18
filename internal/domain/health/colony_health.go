package health

import "fmt"

// ColonyHealthEvaluation is the calculated aggregate of the five component
// dimensions. Dimensions retains the independent evaluations, including the
// beekeeper's separate OVERALL assessment, for explanation.
type ColonyHealthEvaluation struct {
	State      DimensionState
	Coverage   EvidenceCoverage
	Dimensions []DimensionEvaluation
}

var colonyHealthComponents = [...]HealthDimension{
	DimensionStrength,
	DimensionQueen,
	DimensionBrood,
	DimensionNutrition,
	DimensionPestsAndDisease,
}

// EvaluateColonyHealth calculates categorical Colony Health from validated
// dimension evaluations. The OVERALL dimension is retained for explanation
// but is intentionally excluded from the aggregate calculation.
func EvaluateColonyHealth(dimensions []DimensionEvaluation) (ColonyHealthEvaluation, error) {
	byDimension, err := validateDimensionEvaluations(dimensions)
	if err != nil {
		return ColonyHealthEvaluation{}, err
	}

	ordered := make([]DimensionEvaluation, 0, len(evaluationDimensions))
	for _, dimension := range evaluationDimensions {
		ordered = append(ordered, byDimension[dimension])
	}

	usable := 0
	good := 0
	hasWatch := false
	hasQualifyingConcern := false
	hasLowCoverageConcern := false
	for _, dimension := range colonyHealthComponents {
		evaluation := byDimension[dimension]
		if evaluation.State == DimensionUnknown {
			continue
		}
		usable++
		switch evaluation.State {
		case DimensionGood:
			good++
		case DimensionWatch:
			hasWatch = true
		case DimensionConcern:
			if evaluation.Coverage == CoverageMedium || evaluation.Coverage == CoverageHigh {
				hasQualifyingConcern = true
			} else {
				hasLowCoverageConcern = true
			}
		}
	}

	state := DimensionUnknown
	switch {
	case hasQualifyingConcern:
		state = DimensionConcern
	case hasWatch || hasLowCoverageConcern:
		state = DimensionWatch
	case usable >= 3 && good == usable:
		state = DimensionGood
	case usable > 0:
		state = DimensionWatch
	}

	return ColonyHealthEvaluation{
		State:      state,
		Coverage:   aggregateCoverage(usable),
		Dimensions: ordered,
	}, nil
}

func validateDimensionEvaluations(dimensions []DimensionEvaluation) (map[HealthDimension]DimensionEvaluation, error) {
	if len(dimensions) != len(evaluationDimensions) {
		return nil, fmt.Errorf("dimension evaluation set must contain exactly %d dimensions", len(evaluationDimensions))
	}
	result := make(map[HealthDimension]DimensionEvaluation, len(dimensions))
	for _, evaluation := range dimensions {
		if !containsDimension(evaluation.Dimension) {
			return nil, fmt.Errorf("dimension evaluation has unknown dimension %q", evaluation.Dimension)
		}
		if _, exists := result[evaluation.Dimension]; exists {
			return nil, fmt.Errorf("dimension evaluation contains duplicate dimension %q", evaluation.Dimension)
		}
		if !validDimensionState(evaluation.State) {
			return nil, fmt.Errorf("dimension %q has invalid state %q", evaluation.Dimension, evaluation.State)
		}
		if !validEvidenceCoverage(evaluation.Coverage) {
			return nil, fmt.Errorf("dimension %q has invalid coverage %q", evaluation.Dimension, evaluation.Coverage)
		}
		result[evaluation.Dimension] = evaluation
	}
	for _, dimension := range evaluationDimensions {
		if _, exists := result[dimension]; !exists {
			return nil, fmt.Errorf("dimension evaluation is missing dimension %q", dimension)
		}
	}
	return result, nil
}

func aggregateCoverage(usable int) EvidenceCoverage {
	switch {
	case usable == 0:
		return CoverageNone
	case usable == 1:
		return CoverageLow
	case usable <= 3:
		return CoverageMedium
	default:
		return CoverageHigh
	}
}

func validDimensionState(state DimensionState) bool {
	switch state {
	case DimensionUnknown, DimensionGood, DimensionWatch, DimensionConcern:
		return true
	default:
		return false
	}
}

func validEvidenceCoverage(coverage EvidenceCoverage) bool {
	switch coverage {
	case CoverageNone, CoverageLow, CoverageMedium, CoverageHigh:
		return true
	default:
		return false
	}
}

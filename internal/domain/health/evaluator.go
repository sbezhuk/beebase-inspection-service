package health

import (
	"fmt"
	"sort"
	"time"
)

type DimensionState string

const (
	DimensionUnknown DimensionState = "UNKNOWN"
	DimensionGood    DimensionState = "GOOD"
	DimensionWatch   DimensionState = "WATCH"
	DimensionConcern DimensionState = "CONCERN"
)

type EvidenceCoverage string

const (
	CoverageNone   EvidenceCoverage = "NONE"
	CoverageLow    EvidenceCoverage = "LOW"
	CoverageMedium EvidenceCoverage = "MEDIUM"
	CoverageHigh   EvidenceCoverage = "HIGH"
)

type DimensionEvaluationInput struct {
	Evidence         []HealthEvidence
	ContextFacts     []ContextFact
	ManagementEvents []ManagementEvent
	AsOf             time.Time
	RecencyPolicy    RecencyPolicy
}

type DimensionEvaluation struct {
	Dimension            HealthDimension
	State                DimensionState
	Coverage             EvidenceCoverage
	ContributingEvidence []HealthEvidence
}

type evidenceSignal string

const (
	signalUnknown   evidenceSignal = "UNKNOWN"
	signalNeutral   evidenceSignal = "NEUTRAL"
	signalFavorable evidenceSignal = "FAVORABLE"
	signalWatch     evidenceSignal = "WATCH"
	signalConcern   evidenceSignal = "CONCERN"
)

type semanticFact struct {
	dimension HealthDimension
	field     SourceField
}

type classifiedEvidence struct {
	evidence   HealthEvidence
	recency    RecencyClass
	signal     evidenceSignal
	meaningful bool
	primary    bool
}

type factGroup struct {
	fact     semanticFact
	occurred time.Time
	items    []classifiedEvidence
}

var evaluationDimensions = [...]HealthDimension{
	DimensionStrength,
	DimensionQueen,
	DimensionBrood,
	DimensionNutrition,
	DimensionPestsAndDisease,
	DimensionOverall,
}

// EvaluateDimensions deterministically evaluates the six independently
// modelled dimensions. Context facts and management events are intentionally
// not state inputs in v1; they are accepted so callers can keep one pure
// evaluation boundary for future explainability.
func EvaluateDimensions(input DimensionEvaluationInput) ([]DimensionEvaluation, error) {
	if input.AsOf.IsZero() {
		return nil, fmt.Errorf("dimension evaluation AsOf is required")
	}
	if err := input.RecencyPolicy.Validate(); err != nil {
		return nil, err
	}

	groups := make(map[semanticFact][]classifiedEvidence)
	for _, evidence := range input.Evidence {
		if !containsDimension(evidence.Dimension) {
			return nil, fmt.Errorf("health evidence has unknown dimension %q", evidence.Dimension)
		}
		class, eligible, err := ClassifyRecency(evidence, input.AsOf, input.RecencyPolicy)
		if err != nil {
			return nil, err
		}
		if !eligible {
			continue
		}
		signal, meaningful, err := interpretEvidence(evidence)
		if err != nil {
			return nil, err
		}
		fact := semanticFact{dimension: evidence.Dimension, field: evidence.Source.SourceField}
		groups[fact] = append(groups[fact], classifiedEvidence{
			evidence:   evidence,
			recency:    class,
			signal:     signal,
			meaningful: meaningful,
			primary:    isPrimaryEvidence(evidence),
		})
	}

	latest := latestFactGroups(groups)
	result := make([]DimensionEvaluation, 0, len(evaluationDimensions))
	for _, dimension := range evaluationDimensions {
		result = append(result, evaluateDimension(dimension, latest))
	}
	return result, nil
}

func latestFactGroups(groups map[semanticFact][]classifiedEvidence) map[semanticFact]factGroup {
	latest := make(map[semanticFact]factGroup, len(groups))
	for fact, items := range groups {
		occurred := items[0].evidence.Source.OccurredAt
		for _, item := range items[1:] {
			if item.evidence.Source.OccurredAt.After(occurred) {
				occurred = item.evidence.Source.OccurredAt
			}
		}
		group := factGroup{fact: fact, occurred: occurred}
		for _, item := range items {
			if item.evidence.Source.OccurredAt.Equal(occurred) {
				group.items = append(group.items, item)
			}
		}
		latest[fact] = group
	}
	return latest
}

func evaluateDimension(dimension HealthDimension, groups map[semanticFact]factGroup) DimensionEvaluation {
	var dimensionGroups []factGroup
	for fact, group := range groups {
		if fact.dimension == dimension {
			dimensionGroups = append(dimensionGroups, group)
		}
	}
	sort.Slice(dimensionGroups, func(i, j int) bool {
		return dimensionGroups[i].fact.field < dimensionGroups[j].fact.field
	})

	state := DimensionUnknown
	coverage := coverageFor(dimensionGroups)
	var directional []classifiedEvidence
	var eligible []classifiedEvidence
	for _, group := range dimensionGroups {
		for _, item := range group.items {
			if item.recency != RecencyStale {
				eligible = append(eligible, item)
				if item.signal == signalFavorable || item.signal == signalWatch || item.signal == signalConcern {
					directional = append(directional, item)
				}
			}
		}
	}
	state = stateFor(directional)
	contributing := directional
	if state == DimensionUnknown {
		contributing = eligible
		if len(contributing) == 0 {
			for _, group := range dimensionGroups {
				contributing = append(contributing, group.items...)
			}
		}
	}
	sortClassifiedEvidence(contributing)
	return DimensionEvaluation{
		Dimension:            dimension,
		State:                state,
		Coverage:             coverage,
		ContributingEvidence: evidenceOnly(contributing),
	}
}

func stateFor(items []classifiedEvidence) DimensionState {
	hasFavorable := false
	hasWatch := false
	hasConcern := false
	hasPrimaryConcern := false
	hasPrimaryFavorable := false
	for _, item := range items {
		switch item.signal {
		case signalFavorable:
			hasFavorable = true
			if item.primary {
				hasPrimaryFavorable = true
			}
		case signalWatch:
			hasWatch = true
		case signalConcern:
			hasConcern = true
			if item.primary {
				hasPrimaryConcern = true
			}
		}
	}
	switch {
	case hasPrimaryConcern && hasFavorable && !hasPrimaryFavorable:
		return DimensionConcern
	case hasConcern && hasFavorable:
		return DimensionWatch
	case hasConcern:
		return DimensionConcern
	case hasWatch:
		return DimensionWatch
	case hasFavorable:
		return DimensionGood
	default:
		return DimensionUnknown
	}
}

func coverageFor(groups []factGroup) EvidenceCoverage {
	if len(groups) == 0 {
		return CoverageNone
	}
	meaningfulCurrent := 0
	hasMeaningfulRecent := false
	hasMeaningful := false
	for _, group := range groups {
		meaningful := false
		for _, item := range group.items {
			if item.meaningful {
				meaningful = true
				break
			}
		}
		if !meaningful {
			continue
		}
		hasMeaningful = true
		switch group.items[0].recency {
		case RecencyCurrent:
			meaningfulCurrent++
		case RecencyRecent:
			hasMeaningfulRecent = true
		}
	}
	if !hasMeaningful {
		return CoverageLow
	}
	if meaningfulCurrent >= 2 {
		return CoverageHigh
	}
	if meaningfulCurrent == 1 || hasMeaningfulRecent {
		return CoverageMedium
	}
	return CoverageLow
}

func interpretEvidence(evidence HealthEvidence) (evidenceSignal, bool, error) {
	if evidence.State == EvidenceAssessedEmpty {
		switch evidence.Source.SourceField {
		case SourceFieldPestSigns, SourceFieldHealthWarningSigns:
			return signalFavorable, true, nil
		case SourceFieldBroodStages:
			return signalNeutral, true, nil
		default:
			return signalUnknown, false, fmt.Errorf("assessed-empty evidence is invalid for source field %q", evidence.Source.SourceField)
		}
	}
	if evidence.State != EvidenceValueRecorded || evidence.Value == nil {
		return signalUnknown, false, fmt.Errorf("value-recorded evidence for source field %q has no value", evidence.Source.SourceField)
	}

	value := *evidence.Value
	switch evidence.Source.SourceField {
	case SourceFieldColonyStrength:
		switch value {
		case "WEAK":
			return signalConcern, true, nil
		case "MODERATE", "STRONG":
			return signalFavorable, true, nil
		default:
			return unknownValue(evidence)
		}
	case SourceFieldQueenStatus:
		switch value {
		case "HEALTHY":
			return signalFavorable, true, nil
		case "PROBLEM":
			return signalConcern, true, nil
		case "NOT_CHECKED":
			return signalUnknown, false, nil
		default:
			return unknownValue(evidence)
		}
	case SourceFieldBroodStatus:
		switch value {
		case "HEALTHY":
			return signalFavorable, true, nil
		case "PROBLEM":
			return signalConcern, true, nil
		case "NOT_CHECKED":
			return signalUnknown, false, nil
		default:
			return unknownValue(evidence)
		}
	case SourceFieldFoodStores:
		switch value {
		case "LOW":
			return signalConcern, true, nil
		case "ADEQUATE", "ABUNDANT":
			return signalFavorable, true, nil
		case "NOT_CHECKED":
			return signalUnknown, false, nil
		default:
			return unknownValue(evidence)
		}
	case SourceFieldFeedingNeed:
		switch value {
		case "NO":
			return signalFavorable, true, nil
		case "SOON":
			return signalWatch, true, nil
		case "YES":
			return signalConcern, true, nil
		case "UNSURE":
			return signalUnknown, false, nil
		default:
			return unknownValue(evidence)
		}
	case SourceFieldHealthConcerns:
		switch value {
		case "NONE":
			return signalFavorable, true, nil
		case "PRESENT":
			return signalConcern, true, nil
		case "NOT_CHECKED":
			return signalUnknown, false, nil
		default:
			return unknownValue(evidence)
		}
	case SourceFieldQueenObserved:
		switch value {
		case "OBSERVED":
			return signalNeutral, true, nil
		case "NOT_OBSERVED", "UNSURE":
			return signalUnknown, false, nil
		default:
			return unknownValue(evidence)
		}
	case SourceFieldEggsObserved:
		switch value {
		case "YES":
			return signalFavorable, true, nil
		case "NO":
			return signalWatch, true, nil
		case "UNSURE":
			return signalUnknown, false, nil
		default:
			return unknownValue(evidence)
		}
	case SourceFieldQueenCells:
		switch value {
		case "NONE":
			return signalFavorable, true, nil
		case "PRESENT":
			return signalWatch, true, nil
		case "UNSURE":
			return signalUnknown, false, nil
		default:
			return unknownValue(evidence)
		}
	case SourceFieldQueenCondition:
		switch value {
		case "NORMAL":
			return signalFavorable, true, nil
		case "CONCERN":
			return signalConcern, true, nil
		default:
			return unknownValue(evidence)
		}
	case SourceFieldBroodAmount:
		switch value {
		case "LOW":
			return signalWatch, true, nil
		case "MODERATE", "HIGH":
			return signalFavorable, true, nil
		default:
			return unknownValue(evidence)
		}
	case SourceFieldBroodPattern:
		switch value {
		case "SOLID":
			return signalFavorable, true, nil
		case "MIXED", "SPOTTY":
			return signalWatch, true, nil
		default:
			return unknownValue(evidence)
		}
	case SourceFieldBroodStages:
		switch value {
		case "EGGS", "LARVAE", "CAPPED":
			return signalFavorable, true, nil
		default:
			return unknownValue(evidence)
		}
	case SourceFieldBroodConcerns:
		switch value {
		case "NONE":
			return signalFavorable, true, nil
		case "OBSERVED":
			return signalConcern, true, nil
		case "UNSURE":
			return signalUnknown, false, nil
		default:
			return unknownValue(evidence)
		}
	case SourceFieldHealthOverallCondition:
		switch value {
		case "GOOD":
			return signalFavorable, true, nil
		case "FAIR":
			return signalWatch, true, nil
		case "POOR":
			return signalConcern, true, nil
		default:
			return unknownValue(evidence)
		}
	case SourceFieldPestSigns:
		return signalConcern, true, nil
	case SourceFieldHealthWarningSigns:
		return signalWatch, true, nil
	case SourceFieldHealthConcernLevel:
		switch value {
		case "NONE":
			return signalFavorable, true, nil
		case "LOW", "MODERATE":
			return signalWatch, true, nil
		case "HIGH":
			return signalConcern, true, nil
		default:
			return unknownValue(evidence)
		}
	case SourceFieldSeasonalStoreReadiness:
		switch value {
		case "SUFFICIENT":
			return signalFavorable, true, nil
		case "MARGINAL":
			return signalWatch, true, nil
		case "INSUFFICIENT":
			return signalConcern, true, nil
		default:
			return unknownValue(evidence)
		}
	case SourceFieldSeasonalConcerns:
		switch value {
		case "FOOD_STORES", "COLONY_STRENGTH", "QUEEN", "BROOD", "PESTS_OR_DISEASE":
			return signalWatch, true, nil
		default:
			return unknownValue(evidence)
		}
	default:
		return signalUnknown, false, fmt.Errorf("health evidence has unsupported source field %q", evidence.Source.SourceField)
	}
}

func unknownValue(evidence HealthEvidence) (evidenceSignal, bool, error) {
	return signalUnknown, false, fmt.Errorf("health evidence has unsupported value %q for source field %q", *evidence.Value, evidence.Source.SourceField)
}

func sortClassifiedEvidence(items []classifiedEvidence) {
	sort.Slice(items, func(i, j int) bool {
		left := items[i].evidence
		right := items[j].evidence
		if !left.Source.OccurredAt.Equal(right.Source.OccurredAt) {
			return left.Source.OccurredAt.After(right.Source.OccurredAt)
		}
		if left.Source.SourceField != right.Source.SourceField {
			return left.Source.SourceField < right.Source.SourceField
		}
		leftValue := ""
		if left.Value != nil {
			leftValue = *left.Value
		}
		rightValue := ""
		if right.Value != nil {
			rightValue = *right.Value
		}
		if leftValue != rightValue {
			return leftValue < rightValue
		}
		return left.State < right.State
	})
}

func evidenceOnly(items []classifiedEvidence) []HealthEvidence {
	result := make([]HealthEvidence, 0, len(items))
	for _, item := range items {
		result = append(result, item.evidence)
	}
	return result
}

func containsDimension(dimension HealthDimension) bool {
	for _, known := range evaluationDimensions {
		if dimension == known {
			return true
		}
	}
	return false
}

func isPrimaryEvidence(evidence HealthEvidence) bool {
	switch evidence.Dimension {
	case DimensionStrength:
		return evidence.Source.SourceField == SourceFieldColonyStrength
	case DimensionQueen:
		return evidence.Source.SourceField == SourceFieldQueenStatus || evidence.Source.SourceField == SourceFieldQueenCondition
	case DimensionBrood:
		return evidence.Source.SourceField == SourceFieldBroodStatus || evidence.Source.SourceField == SourceFieldBroodConcerns
	case DimensionNutrition:
		return evidence.Source.SourceField == SourceFieldFoodStores || evidence.Source.SourceField == SourceFieldFeedingNeed || evidence.Source.SourceField == SourceFieldSeasonalStoreReadiness
	case DimensionPestsAndDisease:
		return evidence.Source.SourceField == SourceFieldPestSigns || evidence.Source.SourceField == SourceFieldHealthWarningSigns
	case DimensionOverall:
		return true
	default:
		return false
	}
}

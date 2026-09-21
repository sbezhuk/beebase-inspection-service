package health

import (
	"fmt"
	"sort"
	"time"

	"github.com/sbezhuk/beebase-inspection-service/internal/domain/inspection"
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
	// Every meaningful fact found above is STALE: it can explain history but
	// cannot describe the current state, so it contributes no coverage.
	return CoverageNone
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
		switch inspection.ColonyStrength(value) {
		case inspection.ColonyStrengthWeak:
			return signalConcern, true, nil
		case inspection.ColonyStrengthModerate, inspection.ColonyStrengthStrong:
			return signalFavorable, true, nil
		default:
			return unknownValue(evidence)
		}
	case SourceFieldQueenStatus:
		switch inspection.QueenStatus(value) {
		case inspection.QueenStatusHealthy:
			return signalFavorable, true, nil
		case inspection.QueenStatusProblem:
			return signalConcern, true, nil
		case inspection.QueenStatusNotChecked:
			return signalUnknown, false, nil
		default:
			return unknownValue(evidence)
		}
	case SourceFieldBroodStatus:
		switch inspection.BroodStatus(value) {
		case inspection.BroodStatusHealthy:
			return signalFavorable, true, nil
		case inspection.BroodStatusProblem:
			return signalConcern, true, nil
		case inspection.BroodStatusNotChecked:
			return signalUnknown, false, nil
		default:
			return unknownValue(evidence)
		}
	case SourceFieldFoodStores:
		switch inspection.FoodStores(value) {
		case inspection.FoodStoresLow:
			return signalConcern, true, nil
		case inspection.FoodStoresAdequate, inspection.FoodStoresAbundant:
			return signalFavorable, true, nil
		case inspection.FoodStoresNotChecked:
			return signalUnknown, false, nil
		default:
			return unknownValue(evidence)
		}
	case SourceFieldFeedingNeed:
		switch inspection.FeedingNeed(value) {
		case inspection.FeedingNeedNo:
			return signalFavorable, true, nil
		case inspection.FeedingNeedSoon:
			return signalWatch, true, nil
		case inspection.FeedingNeedYes:
			return signalConcern, true, nil
		case inspection.FeedingNeedUnsure:
			return signalUnknown, false, nil
		default:
			return unknownValue(evidence)
		}
	case SourceFieldHealthConcerns:
		switch inspection.HealthConcerns(value) {
		case inspection.HealthConcernsNone:
			return signalFavorable, true, nil
		case inspection.HealthConcernsPresent:
			return signalConcern, true, nil
		case inspection.HealthConcernsNotChecked:
			return signalUnknown, false, nil
		default:
			return unknownValue(evidence)
		}
	case SourceFieldQueenObserved:
		switch inspection.QueenObserved(value) {
		case inspection.QueenObservedObserved:
			return signalNeutral, true, nil
		case inspection.QueenObservedNotObserved, inspection.QueenObservedUnsure:
			return signalUnknown, false, nil
		default:
			return unknownValue(evidence)
		}
	case SourceFieldEggsObserved:
		switch inspection.EggsObserved(value) {
		case inspection.EggsObservedYes:
			return signalFavorable, true, nil
		case inspection.EggsObservedNo:
			return signalWatch, true, nil
		case inspection.EggsObservedUnsure:
			return signalUnknown, false, nil
		default:
			return unknownValue(evidence)
		}
	case SourceFieldQueenCells:
		switch inspection.QueenCells(value) {
		case inspection.QueenCellsNone:
			return signalFavorable, true, nil
		case inspection.QueenCellsPresent:
			return signalWatch, true, nil
		case inspection.QueenCellsUnsure:
			return signalUnknown, false, nil
		default:
			return unknownValue(evidence)
		}
	case SourceFieldQueenCondition:
		switch inspection.QueenCondition(value) {
		case inspection.QueenConditionNormal:
			return signalFavorable, true, nil
		case inspection.QueenConditionConcern:
			return signalConcern, true, nil
		default:
			return unknownValue(evidence)
		}
	case SourceFieldBroodAmount:
		switch inspection.BroodAmount(value) {
		case inspection.BroodAmountLow:
			return signalWatch, true, nil
		case inspection.BroodAmountModerate, inspection.BroodAmountHigh:
			return signalFavorable, true, nil
		default:
			return unknownValue(evidence)
		}
	case SourceFieldBroodPattern:
		switch inspection.BroodPattern(value) {
		case inspection.BroodPatternSolid:
			return signalFavorable, true, nil
		case inspection.BroodPatternMixed, inspection.BroodPatternSpotty:
			return signalWatch, true, nil
		default:
			return unknownValue(evidence)
		}
	case SourceFieldBroodStages:
		switch inspection.BroodStage(value) {
		case inspection.BroodStageEggs, inspection.BroodStageLarvae, inspection.BroodStageCapped:
			return signalFavorable, true, nil
		default:
			return unknownValue(evidence)
		}
	case SourceFieldBroodConcerns:
		switch inspection.BroodConcerns(value) {
		case inspection.BroodConcernsNone:
			return signalFavorable, true, nil
		case inspection.BroodConcernsObserved:
			return signalConcern, true, nil
		case inspection.BroodConcernsUnsure:
			return signalUnknown, false, nil
		default:
			return unknownValue(evidence)
		}
	case SourceFieldHealthOverallCondition:
		switch inspection.HealthOverallCondition(value) {
		case inspection.HealthOverallGood:
			return signalFavorable, true, nil
		case inspection.HealthOverallFair:
			return signalWatch, true, nil
		case inspection.HealthOverallPoor:
			return signalConcern, true, nil
		default:
			return unknownValue(evidence)
		}
	case SourceFieldPestSigns:
		return signalConcern, true, nil
	case SourceFieldHealthWarningSigns:
		return signalWatch, true, nil
	case SourceFieldHealthConcernLevel:
		switch inspection.HealthConcernLevel(value) {
		case inspection.HealthConcernNone:
			return signalFavorable, true, nil
		case inspection.HealthConcernLow, inspection.HealthConcernModerate:
			return signalWatch, true, nil
		case inspection.HealthConcernHigh:
			return signalConcern, true, nil
		default:
			return unknownValue(evidence)
		}
	case SourceFieldSeasonalStoreReadiness:
		switch inspection.SeasonalStoreReadiness(value) {
		case inspection.SeasonalStoresSufficient:
			return signalFavorable, true, nil
		case inspection.SeasonalStoresMarginal:
			return signalWatch, true, nil
		case inspection.SeasonalStoresInsufficient:
			return signalConcern, true, nil
		default:
			return unknownValue(evidence)
		}
	case SourceFieldSeasonalConcerns:
		switch inspection.SeasonalConcern(value) {
		case inspection.SeasonalConcernFoodStores, inspection.SeasonalConcernColonyStrength, inspection.SeasonalConcernQueen, inspection.SeasonalConcernBrood, inspection.SeasonalConcernPestsOrDisease:
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

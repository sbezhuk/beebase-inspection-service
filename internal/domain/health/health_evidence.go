// Package health contains pure normalization and dimension-evaluation logic
// for colony health evidence. It does not calculate aggregate health scores.
package health

import (
	"time"

	"github.com/google/uuid"

	"github.com/sbezhuk/beebase-inspection-service/internal/domain/inspection"
)

type HealthDimension string

const (
	DimensionStrength        HealthDimension = "STRENGTH"
	DimensionQueen           HealthDimension = "QUEEN"
	DimensionBrood           HealthDimension = "BROOD"
	DimensionNutrition       HealthDimension = "NUTRITION"
	DimensionPestsAndDisease HealthDimension = "PESTS_AND_DISEASE"
	DimensionOverall         HealthDimension = "OVERALL"
)

type EvidenceKind string

const (
	EvidenceObservation         EvidenceKind = "OBSERVATION"
	EvidenceBeekeeperAssessment EvidenceKind = "BEEKEEPER_ASSESSMENT"
)

type EvidenceState string

const (
	EvidenceValueRecorded EvidenceState = "VALUE_RECORDED"
	EvidenceAssessedEmpty EvidenceState = "ASSESSED_EMPTY"
)

type EvidenceProvider string

const (
	ProviderInspectionAssessment EvidenceProvider = "INSPECTION_ASSESSMENT"
	ProviderQueenHistory         EvidenceProvider = "QUEEN_HISTORY"
)

type SourceField string

const (
	SourceFieldColonyStrength         SourceField = "colonyStrength"
	SourceFieldQueenStatus            SourceField = "queenStatus"
	SourceFieldBroodStatus            SourceField = "broodStatus"
	SourceFieldFoodStores             SourceField = "foodStores"
	SourceFieldHealthConcerns         SourceField = "healthConcerns"
	SourceFieldQueenObserved          SourceField = "queenObserved"
	SourceFieldEggsObserved           SourceField = "eggsObserved"
	SourceFieldQueenCells             SourceField = "queenCells"
	SourceFieldQueenCondition         SourceField = "queenCondition"
	SourceFieldBroodAmount            SourceField = "broodAmount"
	SourceFieldBroodPattern           SourceField = "broodPattern"
	SourceFieldBroodStages            SourceField = "broodStages"
	SourceFieldBroodConcerns          SourceField = "broodConcerns"
	SourceFieldHealthOverallCondition SourceField = "healthOverallCondition"
	SourceFieldPestSigns              SourceField = "pestSigns"
	SourceFieldHealthWarningSigns     SourceField = "healthWarningSigns"
	SourceFieldHealthConcernLevel     SourceField = "healthConcernLevel"
	SourceFieldFeedingNeed            SourceField = "feedingNeed"
	SourceFieldFeedingPerformed       SourceField = "feedingPerformed"
	SourceFieldFeedTypes              SourceField = "feedTypes"
	SourceFieldSeason                 SourceField = "season"
	SourceFieldSeasonalStoreReadiness SourceField = "seasonalStoreReadiness"
	SourceFieldSeasonalReadiness      SourceField = "seasonalReadiness"
	SourceFieldSeasonalConcerns       SourceField = "seasonalConcerns"
)

type ManagementAction string

const (
	ActionFeedingPerformed ManagementAction = "FEEDING_PERFORMED"
	ActionFeedTypes        ManagementAction = "FEED_TYPES"
)

type ContextKind string

const (
	ContextSeason            ContextKind = "SEASON"
	ContextSeasonalReadiness ContextKind = "SEASONAL_READINESS"
	ContextSeasonalConcern   ContextKind = "SEASONAL_CONCERN"
)

type EvidenceSource struct {
	Provider                EvidenceProvider
	HiveID                  uuid.UUID
	EntityID                uuid.UUID
	InspectionID            *uuid.UUID
	InspectionType          *inspection.Type
	AssessmentSchema        string
	AssessmentSchemaVersion int
	OccurredAt              time.Time
	SourceField             SourceField
}

type HealthEvidence struct {
	Dimension HealthDimension
	Kind      EvidenceKind
	State     EvidenceState
	Source    EvidenceSource
	Value     *string
}

type ManagementEvent struct {
	Source        EvidenceSource
	Action        ManagementAction
	Value         *string
	RelatedValues *[]string
}

type ContextFact struct {
	Source EvidenceSource
	Kind   ContextKind
	State  EvidenceState
	Value  *string
}

type NormalizedInspection struct {
	HealthEvidence   []HealthEvidence
	ManagementEvents []ManagementEvent
	ContextFacts     []ContextFact
}

func NormalizeInspection(input inspection.Inspection) NormalizedInspection {
	result := NormalizedInspection{
		HealthEvidence:   []HealthEvidence{},
		ManagementEvents: []ManagementEvent{},
		ContextFacts:     []ContextFact{},
	}
	assessment := input.Assessment
	if assessment == nil {
		return result
	}

	base := sourceFor(input, assessment)
	addValue := func(dimension HealthDimension, kind EvidenceKind, field SourceField, value string) {
		valueCopy := value
		source := base
		source.SourceField = field
		result.HealthEvidence = append(result.HealthEvidence, HealthEvidence{
			Dimension: dimension,
			Kind:      kind,
			State:     EvidenceValueRecorded,
			Source:    source,
			Value:     &valueCopy,
		})
	}
	addEmpty := func(dimension HealthDimension, kind EvidenceKind, field SourceField) {
		source := base
		source.SourceField = field
		result.HealthEvidence = append(result.HealthEvidence, HealthEvidence{
			Dimension: dimension,
			Kind:      kind,
			State:     EvidenceAssessedEmpty,
			Source:    source,
		})
	}
	addContext := func(kind ContextKind, field SourceField, value string) {
		valueCopy := value
		source := base
		source.SourceField = field
		result.ContextFacts = append(result.ContextFacts, ContextFact{
			Kind:   kind,
			State:  EvidenceValueRecorded,
			Source: source,
			Value:  &valueCopy,
		})
	}
	addContextEmpty := func(kind ContextKind, field SourceField) {
		source := base
		source.SourceField = field
		result.ContextFacts = append(result.ContextFacts, ContextFact{
			Kind:   kind,
			State:  EvidenceAssessedEmpty,
			Source: source,
		})
	}

	if assessment.ColonyStrength != nil {
		value := string(*assessment.ColonyStrength)
		switch input.Type {
		case inspection.TypeRoutine, inspection.TypeSeasonal:
			addValue(DimensionStrength, EvidenceObservation, SourceFieldColonyStrength, value)
		}
	}
	if assessment.QueenStatus != nil {
		addValue(DimensionQueen, EvidenceBeekeeperAssessment, SourceFieldQueenStatus, string(*assessment.QueenStatus))
	}
	if assessment.BroodStatus != nil {
		addValue(DimensionBrood, EvidenceBeekeeperAssessment, SourceFieldBroodStatus, string(*assessment.BroodStatus))
	}
	if assessment.FoodStores != nil {
		addValue(DimensionNutrition, EvidenceObservation, SourceFieldFoodStores, string(*assessment.FoodStores))
	}
	if assessment.HealthConcerns != nil {
		addValue(DimensionOverall, EvidenceBeekeeperAssessment, SourceFieldHealthConcerns, string(*assessment.HealthConcerns))
	}
	if assessment.QueenObserved != nil {
		addValue(DimensionQueen, EvidenceObservation, SourceFieldQueenObserved, string(*assessment.QueenObserved))
	}
	if assessment.EggsObserved != nil {
		addValue(DimensionQueen, EvidenceObservation, SourceFieldEggsObserved, string(*assessment.EggsObserved))
	}
	if assessment.QueenCells != nil {
		addValue(DimensionQueen, EvidenceObservation, SourceFieldQueenCells, string(*assessment.QueenCells))
	}
	if assessment.QueenCondition != nil {
		addValue(DimensionQueen, EvidenceBeekeeperAssessment, SourceFieldQueenCondition, string(*assessment.QueenCondition))
	}
	if assessment.BroodAmount != nil {
		addValue(DimensionBrood, EvidenceObservation, SourceFieldBroodAmount, string(*assessment.BroodAmount))
	}
	if assessment.BroodPattern != nil {
		addValue(DimensionBrood, EvidenceObservation, SourceFieldBroodPattern, string(*assessment.BroodPattern))
	}
	if assessment.BroodStages != nil {
		if len(*assessment.BroodStages) == 0 {
			addEmpty(DimensionBrood, EvidenceObservation, SourceFieldBroodStages)
		} else {
			for _, value := range *assessment.BroodStages {
				addValue(DimensionBrood, EvidenceObservation, SourceFieldBroodStages, string(value))
			}
		}
	}
	if assessment.BroodConcerns != nil {
		addValue(DimensionBrood, EvidenceBeekeeperAssessment, SourceFieldBroodConcerns, string(*assessment.BroodConcerns))
	}
	if assessment.HealthOverallCondition != nil {
		addValue(DimensionOverall, EvidenceBeekeeperAssessment, SourceFieldHealthOverallCondition, string(*assessment.HealthOverallCondition))
	}
	if assessment.PestSigns != nil {
		if len(*assessment.PestSigns) == 0 {
			addEmpty(DimensionPestsAndDisease, EvidenceObservation, SourceFieldPestSigns)
		} else {
			for _, value := range *assessment.PestSigns {
				addValue(DimensionPestsAndDisease, EvidenceObservation, SourceFieldPestSigns, string(value))
			}
		}
	}
	if assessment.HealthWarningSigns != nil {
		if len(*assessment.HealthWarningSigns) == 0 {
			addEmpty(DimensionPestsAndDisease, EvidenceObservation, SourceFieldHealthWarningSigns)
		} else {
			for _, value := range *assessment.HealthWarningSigns {
				addValue(DimensionPestsAndDisease, EvidenceObservation, SourceFieldHealthWarningSigns, string(value))
			}
		}
	}
	if assessment.HealthConcernLevel != nil {
		addValue(DimensionOverall, EvidenceBeekeeperAssessment, SourceFieldHealthConcernLevel, string(*assessment.HealthConcernLevel))
	}
	if assessment.FeedingNeed != nil {
		addValue(DimensionNutrition, EvidenceBeekeeperAssessment, SourceFieldFeedingNeed, string(*assessment.FeedingNeed))
	}

	if assessment.FeedingPerformed != nil {
		value := string(*assessment.FeedingPerformed)
		source := base
		source.SourceField = SourceFieldFeedingPerformed
		result.ManagementEvents = append(result.ManagementEvents, ManagementEvent{
			Source:        source,
			Action:        ActionFeedingPerformed,
			Value:         &value,
			RelatedValues: stringSlicePtr(assessment.FeedTypes),
		})
	}
	if assessment.FeedTypes != nil && assessment.FeedingPerformed == nil {
		source := base
		source.SourceField = SourceFieldFeedTypes
		result.ManagementEvents = append(result.ManagementEvents, ManagementEvent{
			Source:        source,
			Action:        ActionFeedTypes,
			RelatedValues: stringSlicePtr(assessment.FeedTypes),
		})
	}

	if assessment.Season != nil {
		addContext(ContextSeason, SourceFieldSeason, string(*assessment.Season))
	}
	if assessment.SeasonalStoreReadiness != nil {
		addValue(DimensionNutrition, EvidenceBeekeeperAssessment, SourceFieldSeasonalStoreReadiness, string(*assessment.SeasonalStoreReadiness))
	}
	if assessment.SeasonalReadiness != nil {
		addContext(ContextSeasonalReadiness, SourceFieldSeasonalReadiness, string(*assessment.SeasonalReadiness))
	}
	if assessment.SeasonalConcerns != nil {
		if len(*assessment.SeasonalConcerns) == 0 {
			addContextEmpty(ContextSeasonalConcern, SourceFieldSeasonalConcerns)
		} else {
			for _, concern := range *assessment.SeasonalConcerns {
				value := string(concern)
				switch concern {
				case inspection.SeasonalConcernFoodStores:
					addValue(DimensionNutrition, EvidenceBeekeeperAssessment, SourceFieldSeasonalConcerns, value)
				case inspection.SeasonalConcernColonyStrength:
					addValue(DimensionStrength, EvidenceBeekeeperAssessment, SourceFieldSeasonalConcerns, value)
				case inspection.SeasonalConcernQueen:
					addValue(DimensionQueen, EvidenceBeekeeperAssessment, SourceFieldSeasonalConcerns, value)
				case inspection.SeasonalConcernBrood:
					addValue(DimensionBrood, EvidenceBeekeeperAssessment, SourceFieldSeasonalConcerns, value)
				case inspection.SeasonalConcernPestsOrDisease:
					addValue(DimensionPestsAndDisease, EvidenceBeekeeperAssessment, SourceFieldSeasonalConcerns, value)
				case inspection.SeasonalConcernHiveCondition, inspection.SeasonalConcernOther:
					addContext(ContextSeasonalConcern, SourceFieldSeasonalConcerns, value)
				}
			}
		}
	}

	return result
}

func sourceFor(input inspection.Inspection, assessment *inspection.Assessment) EvidenceSource {
	inspectionID := input.ID
	inspectionType := input.Type
	version := assessment.Version
	if version == 0 {
		version = inspection.AssessmentVersion
	}
	return EvidenceSource{
		Provider:                ProviderInspectionAssessment,
		HiveID:                  input.HiveID,
		EntityID:                input.ID,
		InspectionID:            &inspectionID,
		InspectionType:          &inspectionType,
		AssessmentSchema:        string(input.Type),
		AssessmentSchemaVersion: version,
		OccurredAt:              input.InspectedAt,
	}
}

func stringSlicePtr(values *[]inspection.FeedType) *[]string {
	if values == nil {
		return nil
	}
	result := make([]string, len(*values))
	for index, value := range *values {
		result[index] = string(value)
	}
	return &result
}

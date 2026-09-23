package inspection

import (
	"github.com/sbezhuk/beebase-health/health"
	inspectiondomain "github.com/sbezhuk/beebase-inspection-service/internal/domain/inspection"
)

// toHealthInspection is the service-side boundary between persisted
// inspection models and the pure health module. It intentionally copies only
// the fields used by normalization; ownership, sync, notes, media, and
// persistence timestamps never enter the health engine.
func toHealthInspection(input *inspectiondomain.Inspection) health.Inspection {
	if input == nil {
		return health.Inspection{}
	}
	return health.Inspection{
		ID:          input.ID,
		HiveID:      input.HiveID,
		InspectedAt: input.InspectedAt,
		Type:        health.Type(input.Type),
		Assessment:  toHealthAssessment(input.Assessment),
	}
}

func toHealthAssessment(input *inspectiondomain.Assessment) *health.Assessment {
	if input == nil {
		return nil
	}
	return &health.Assessment{
		Version:                input.Version,
		ColonyStrength:         convertPtr[inspectiondomain.ColonyStrength, health.ColonyStrength](input.ColonyStrength),
		QueenStatus:            convertPtr[inspectiondomain.QueenStatus, health.QueenStatus](input.QueenStatus),
		BroodStatus:            convertPtr[inspectiondomain.BroodStatus, health.BroodStatus](input.BroodStatus),
		FoodStores:             convertPtr[inspectiondomain.FoodStores, health.FoodStores](input.FoodStores),
		HealthConcerns:         convertPtr[inspectiondomain.HealthConcerns, health.HealthConcerns](input.HealthConcerns),
		QueenObserved:          convertPtr[inspectiondomain.QueenObserved, health.QueenObserved](input.QueenObserved),
		EggsObserved:           convertPtr[inspectiondomain.EggsObserved, health.EggsObserved](input.EggsObserved),
		QueenCells:             convertPtr[inspectiondomain.QueenCells, health.QueenCells](input.QueenCells),
		QueenCondition:         convertPtr[inspectiondomain.QueenCondition, health.QueenCondition](input.QueenCondition),
		BroodAmount:            convertPtr[inspectiondomain.BroodAmount, health.BroodAmount](input.BroodAmount),
		BroodPattern:           convertPtr[inspectiondomain.BroodPattern, health.BroodPattern](input.BroodPattern),
		BroodStages:            convertSlice[inspectiondomain.BroodStage, health.BroodStage](input.BroodStages),
		BroodConcerns:          convertPtr[inspectiondomain.BroodConcerns, health.BroodConcerns](input.BroodConcerns),
		HealthOverallCondition: convertPtr[inspectiondomain.HealthOverallCondition, health.HealthOverallCondition](input.HealthOverallCondition),
		PestSigns:              convertSlice[inspectiondomain.PestSign, health.PestSign](input.PestSigns),
		HealthWarningSigns:     convertSlice[inspectiondomain.HealthWarningSign, health.HealthWarningSign](input.HealthWarningSigns),
		HealthConcernLevel:     convertPtr[inspectiondomain.HealthConcernLevel, health.HealthConcernLevel](input.HealthConcernLevel),
		FeedingNeed:            convertPtr[inspectiondomain.FeedingNeed, health.FeedingNeed](input.FeedingNeed),
		FeedingPerformed:       convertPtr[inspectiondomain.FeedingPerformed, health.FeedingPerformed](input.FeedingPerformed),
		FeedTypes:              convertSlice[inspectiondomain.FeedType, health.FeedType](input.FeedTypes),
		Season:                 convertPtr[inspectiondomain.SeasonalPhase, health.SeasonalPhase](input.Season),
		SeasonalStoreReadiness: convertPtr[inspectiondomain.SeasonalStoreReadiness, health.SeasonalStoreReadiness](input.SeasonalStoreReadiness),
		SeasonalReadiness:      convertPtr[inspectiondomain.SeasonalReadiness, health.SeasonalReadiness](input.SeasonalReadiness),
		SeasonalConcerns:       convertSlice[inspectiondomain.SeasonalConcern, health.SeasonalConcern](input.SeasonalConcerns),
	}
}

func convertPtr[S ~string, T ~string](value *S) *T {
	if value == nil {
		return nil
	}
	converted := T(*value)
	return &converted
}

func convertSlice[S ~string, T ~string](values *[]S) *[]T {
	if values == nil {
		return nil
	}
	converted := make([]T, len(*values))
	for index, value := range *values {
		converted[index] = T(value)
	}
	return &converted
}

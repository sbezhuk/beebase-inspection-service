package inspection

import "fmt"

const AssessmentVersion = 1

type ColonyStrength string
type QueenStatus string
type BroodStatus string
type FoodStores string
type HealthConcerns string
type QueenObserved string
type EggsObserved string
type QueenCells string
type QueenCondition string
type BroodAmount string
type BroodPattern string
type BroodStage string
type BroodConcerns string
type HealthOverallCondition string
type PestSign string
type HealthWarningSign string
type HealthConcernLevel string
type FeedingNeed string
type FeedingPerformed string
type FeedType string
type SeasonalPhase string
type SeasonalStoreReadiness string
type SeasonalReadiness string
type SeasonalConcern string

type assessmentField string

const (
	fieldColonyStrength         assessmentField = "colonyStrength"
	fieldQueenStatus            assessmentField = "queenStatus"
	fieldBroodStatus            assessmentField = "broodStatus"
	fieldFoodStores             assessmentField = "foodStores"
	fieldHealthConcerns         assessmentField = "healthConcerns"
	fieldQueenObserved          assessmentField = "queenObserved"
	fieldEggsObserved           assessmentField = "eggsObserved"
	fieldQueenCells             assessmentField = "queenCells"
	fieldQueenCondition         assessmentField = "queenCondition"
	fieldBroodAmount            assessmentField = "broodAmount"
	fieldBroodPattern           assessmentField = "broodPattern"
	fieldBroodStages            assessmentField = "broodStages"
	fieldBroodConcerns          assessmentField = "broodConcerns"
	fieldHealthOverallCondition assessmentField = "healthOverallCondition"
	fieldPestSigns              assessmentField = "pestSigns"
	fieldHealthWarningSigns     assessmentField = "healthWarningSigns"
	fieldHealthConcernLevel     assessmentField = "healthConcernLevel"
	fieldFeedingNeed            assessmentField = "feedingNeed"
	fieldFeedingPerformed       assessmentField = "feedingPerformed"
	fieldFeedTypes              assessmentField = "feedTypes"
	fieldSeason                 assessmentField = "season"
	fieldSeasonalStoreReadiness assessmentField = "seasonalStoreReadiness"
	fieldSeasonalReadiness      assessmentField = "seasonalReadiness"
	fieldSeasonalConcerns       assessmentField = "seasonalConcerns"
)

const (
	ColonyStrengthWeak               ColonyStrength         = "WEAK"
	ColonyStrengthModerate           ColonyStrength         = "MODERATE"
	ColonyStrengthStrong             ColonyStrength         = "STRONG"
	QueenStatusHealthy               QueenStatus            = "HEALTHY"
	QueenStatusProblem               QueenStatus            = "PROBLEM"
	QueenStatusNotChecked            QueenStatus            = "NOT_CHECKED"
	BroodStatusHealthy               BroodStatus            = "HEALTHY"
	BroodStatusProblem               BroodStatus            = "PROBLEM"
	BroodStatusNotChecked            BroodStatus            = "NOT_CHECKED"
	FoodStoresLow                    FoodStores             = "LOW"
	FoodStoresAdequate               FoodStores             = "ADEQUATE"
	FoodStoresAbundant               FoodStores             = "ABUNDANT"
	FoodStoresNotChecked             FoodStores             = "NOT_CHECKED"
	HealthConcernsNone               HealthConcerns         = "NONE"
	HealthConcernsPresent            HealthConcerns         = "PRESENT"
	HealthConcernsNotChecked         HealthConcerns         = "NOT_CHECKED"
	QueenObservedObserved            QueenObserved          = "OBSERVED"
	QueenObservedNotObserved         QueenObserved          = "NOT_OBSERVED"
	QueenObservedUnsure              QueenObserved          = "UNSURE"
	EggsObservedYes                  EggsObserved           = "YES"
	EggsObservedNo                   EggsObserved           = "NO"
	EggsObservedUnsure               EggsObserved           = "UNSURE"
	QueenCellsNone                   QueenCells             = "NONE"
	QueenCellsPresent                QueenCells             = "PRESENT"
	QueenCellsUnsure                 QueenCells             = "UNSURE"
	QueenConditionNormal             QueenCondition         = "NORMAL"
	QueenConditionConcern            QueenCondition         = "CONCERN"
	BroodAmountLow                   BroodAmount            = "LOW"
	BroodAmountModerate              BroodAmount            = "MODERATE"
	BroodAmountHigh                  BroodAmount            = "HIGH"
	BroodPatternSolid                BroodPattern           = "SOLID"
	BroodPatternMixed                BroodPattern           = "MIXED"
	BroodPatternSpotty               BroodPattern           = "SPOTTY"
	BroodStageEggs                   BroodStage             = "EGGS"
	BroodStageLarvae                 BroodStage             = "LARVAE"
	BroodStageCapped                 BroodStage             = "CAPPED"
	BroodConcernsNone                BroodConcerns          = "NONE"
	BroodConcernsObserved            BroodConcerns          = "OBSERVED"
	BroodConcernsUnsure              BroodConcerns          = "UNSURE"
	HealthOverallGood                HealthOverallCondition = "GOOD"
	HealthOverallFair                HealthOverallCondition = "FAIR"
	HealthOverallPoor                HealthOverallCondition = "POOR"
	PestSignVarroaMites              PestSign               = "VARROA_MITES"
	PestSignWaxMoth                  PestSign               = "WAX_MOTH"
	PestSignSmallHiveBeetle          PestSign               = "SMALL_HIVE_BEETLE"
	PestSignOther                    PestSign               = "OTHER"
	HealthWarningAbnormalBrood       HealthWarningSign      = "ABNORMAL_BROOD"
	HealthWarningDeformedWings       HealthWarningSign      = "DEFORMED_WINGS"
	HealthWarningUnusualBeeMortality HealthWarningSign      = "UNUSUAL_BEE_MORTALITY"
	HealthWarningDiarrheaSigns       HealthWarningSign      = "DIARRHEA_SIGNS"
	HealthWarningOther               HealthWarningSign      = "OTHER"
	HealthConcernNone                HealthConcernLevel     = "NONE"
	HealthConcernLow                 HealthConcernLevel     = "LOW"
	HealthConcernModerate            HealthConcernLevel     = "MODERATE"
	HealthConcernHigh                HealthConcernLevel     = "HIGH"
	FeedingNeedNo                    FeedingNeed            = "NO"
	FeedingNeedSoon                  FeedingNeed            = "SOON"
	FeedingNeedYes                   FeedingNeed            = "YES"
	FeedingNeedUnsure                FeedingNeed            = "UNSURE"
	FeedingPerformedYes              FeedingPerformed       = "YES"
	FeedingPerformedNo               FeedingPerformed       = "NO"
	FeedTypeSugarSyrup               FeedType               = "SUGAR_SYRUP"
	FeedTypeFondant                  FeedType               = "FONDANT"
	FeedTypeDrySugar                 FeedType               = "DRY_SUGAR"
	FeedTypePollenSubstitute         FeedType               = "POLLEN_SUBSTITUTE"
	FeedTypeOther                    FeedType               = "OTHER"
	SeasonalSpring                   SeasonalPhase          = "SPRING"
	SeasonalSummer                   SeasonalPhase          = "SUMMER"
	SeasonalAutumn                   SeasonalPhase          = "AUTUMN"
	SeasonalWinter                   SeasonalPhase          = "WINTER"
	SeasonalStoresSufficient         SeasonalStoreReadiness = "SUFFICIENT"
	SeasonalStoresMarginal           SeasonalStoreReadiness = "MARGINAL"
	SeasonalStoresInsufficient       SeasonalStoreReadiness = "INSUFFICIENT"
	SeasonalReadinessReady           SeasonalReadiness      = "READY"
	SeasonalReadinessNeedsAttention  SeasonalReadiness      = "NEEDS_ATTENTION"
	SeasonalReadinessNotReady        SeasonalReadiness      = "NOT_READY"
	SeasonalReadinessUnsure          SeasonalReadiness      = "UNSURE"
	SeasonalConcernFoodStores        SeasonalConcern        = "FOOD_STORES"
	SeasonalConcernColonyStrength    SeasonalConcern        = "COLONY_STRENGTH"
	SeasonalConcernQueen             SeasonalConcern        = "QUEEN"
	SeasonalConcernBrood             SeasonalConcern        = "BROOD"
	SeasonalConcernPestsOrDisease    SeasonalConcern        = "PESTS_OR_DISEASE"
	SeasonalConcernHiveCondition     SeasonalConcern        = "HIVE_CONDITION"
	SeasonalConcernOther             SeasonalConcern        = "OTHER"
)

// Assessment is explicit and typed. A nil field means that metric was not answered.
type Assessment struct {
	Version                int
	ColonyStrength         *ColonyStrength
	QueenStatus            *QueenStatus
	BroodStatus            *BroodStatus
	FoodStores             *FoodStores
	HealthConcerns         *HealthConcerns
	QueenObserved          *QueenObserved
	EggsObserved           *EggsObserved
	QueenCells             *QueenCells
	QueenCondition         *QueenCondition
	BroodAmount            *BroodAmount
	BroodPattern           *BroodPattern
	BroodStages            *[]BroodStage
	BroodConcerns          *BroodConcerns
	HealthOverallCondition *HealthOverallCondition
	PestSigns              *[]PestSign
	HealthWarningSigns     *[]HealthWarningSign
	HealthConcernLevel     *HealthConcernLevel
	FeedingNeed            *FeedingNeed
	FeedingPerformed       *FeedingPerformed
	FeedTypes              *[]FeedType
	Season                 *SeasonalPhase
	SeasonalStoreReadiness *SeasonalStoreReadiness
	SeasonalReadiness      *SeasonalReadiness
	SeasonalConcerns       *[]SeasonalConcern
}

func (a *Assessment) ValidateFor(t Type) error {
	if a == nil {
		return nil
	}
	if t != TypeRoutine && t != TypeQueen && t != TypeBrood && t != TypeHealth && t != TypeFeeding && t != TypeSeasonal {
		return fmt.Errorf("assessment is not supported for %s", t)
	}
	if a.Version == 0 {
		a.Version = AssessmentVersion
	}
	if a.Version != AssessmentVersion {
		return fmt.Errorf("unsupported assessment version %d", a.Version)
	}
	if err := validateAllowedFields(a, t); err != nil {
		return err
	}
	if t == TypeRoutine {
		if a.ColonyStrength != nil && *a.ColonyStrength != ColonyStrengthWeak && *a.ColonyStrength != ColonyStrengthModerate && *a.ColonyStrength != ColonyStrengthStrong {
			return fmt.Errorf("invalid colony strength")
		}
		if a.QueenStatus != nil && *a.QueenStatus != QueenStatusHealthy && *a.QueenStatus != QueenStatusProblem && *a.QueenStatus != QueenStatusNotChecked {
			return fmt.Errorf("invalid queen status")
		}
		if a.BroodStatus != nil && *a.BroodStatus != BroodStatusHealthy && *a.BroodStatus != BroodStatusProblem && *a.BroodStatus != BroodStatusNotChecked {
			return fmt.Errorf("invalid brood status")
		}
		if a.FoodStores != nil && *a.FoodStores != FoodStoresLow && *a.FoodStores != FoodStoresAdequate && *a.FoodStores != FoodStoresAbundant && *a.FoodStores != FoodStoresNotChecked {
			return fmt.Errorf("invalid food stores")
		}
		if a.HealthConcerns != nil && *a.HealthConcerns != HealthConcernsNone && *a.HealthConcerns != HealthConcernsPresent && *a.HealthConcerns != HealthConcernsNotChecked {
			return fmt.Errorf("invalid health concerns")
		}
		return nil
	}
	if t == TypeHealth {
		if a.HealthOverallCondition != nil && *a.HealthOverallCondition != HealthOverallGood && *a.HealthOverallCondition != HealthOverallFair && *a.HealthOverallCondition != HealthOverallPoor {
			return fmt.Errorf("invalid health overall condition")
		}
		if a.HealthConcernLevel != nil && *a.HealthConcernLevel != HealthConcernNone && *a.HealthConcernLevel != HealthConcernLow && *a.HealthConcernLevel != HealthConcernModerate && *a.HealthConcernLevel != HealthConcernHigh {
			return fmt.Errorf("invalid health concern level")
		}
		if err := validatePestSigns(a.PestSigns); err != nil {
			return err
		}
		if err := validateHealthWarningSigns(a.HealthWarningSigns); err != nil {
			return err
		}
		return nil
	}
	if t == TypeFeeding {
		if a.FoodStores != nil && *a.FoodStores != FoodStoresLow && *a.FoodStores != FoodStoresAdequate && *a.FoodStores != FoodStoresAbundant && *a.FoodStores != FoodStoresNotChecked {
			return fmt.Errorf("invalid food stores")
		}
		if a.FeedingNeed != nil && *a.FeedingNeed != FeedingNeedNo && *a.FeedingNeed != FeedingNeedSoon && *a.FeedingNeed != FeedingNeedYes && *a.FeedingNeed != FeedingNeedUnsure {
			return fmt.Errorf("invalid feeding need")
		}
		if a.FeedingPerformed != nil && *a.FeedingPerformed != FeedingPerformedYes && *a.FeedingPerformed != FeedingPerformedNo {
			return fmt.Errorf("invalid feeding performed")
		}
		if a.FeedingPerformed == nil || *a.FeedingPerformed != FeedingPerformedYes {
			if a.FeedTypes != nil && len(*a.FeedTypes) > 0 {
				return fmt.Errorf("feed types require feeding performed YES")
			}
		}
		if err := validateFeedTypes(a.FeedTypes); err != nil {
			return err
		}
		return nil
	}
	if t == TypeSeasonal {
		if a.Season != nil && *a.Season != SeasonalSpring && *a.Season != SeasonalSummer && *a.Season != SeasonalAutumn && *a.Season != SeasonalWinter {
			return fmt.Errorf("invalid seasonal phase")
		}
		if a.ColonyStrength != nil && *a.ColonyStrength != ColonyStrengthWeak && *a.ColonyStrength != ColonyStrengthModerate && *a.ColonyStrength != ColonyStrengthStrong {
			return fmt.Errorf("invalid colony strength")
		}
		if a.SeasonalStoreReadiness != nil && *a.SeasonalStoreReadiness != SeasonalStoresSufficient && *a.SeasonalStoreReadiness != SeasonalStoresMarginal && *a.SeasonalStoreReadiness != SeasonalStoresInsufficient {
			return fmt.Errorf("invalid seasonal store readiness")
		}
		if a.SeasonalReadiness != nil && *a.SeasonalReadiness != SeasonalReadinessReady && *a.SeasonalReadiness != SeasonalReadinessNeedsAttention && *a.SeasonalReadiness != SeasonalReadinessNotReady && *a.SeasonalReadiness != SeasonalReadinessUnsure {
			return fmt.Errorf("invalid seasonal readiness")
		}
		return validateSeasonalConcerns(a.SeasonalConcerns)
	}
	if a.QueenObserved != nil && *a.QueenObserved != QueenObservedObserved && *a.QueenObserved != QueenObservedNotObserved && *a.QueenObserved != QueenObservedUnsure {
		return fmt.Errorf("invalid queen observed")
	}
	if a.EggsObserved != nil && *a.EggsObserved != EggsObservedYes && *a.EggsObserved != EggsObservedNo && *a.EggsObserved != EggsObservedUnsure {
		return fmt.Errorf("invalid eggs observed")
	}
	if a.QueenCells != nil && *a.QueenCells != QueenCellsNone && *a.QueenCells != QueenCellsPresent && *a.QueenCells != QueenCellsUnsure {
		return fmt.Errorf("invalid queen cells")
	}
	if a.QueenCondition != nil && *a.QueenCondition != QueenConditionNormal && *a.QueenCondition != QueenConditionConcern {
		return fmt.Errorf("invalid queen condition")
	}
	if a.QueenCondition != nil && (a.QueenObserved == nil || *a.QueenObserved != QueenObservedObserved) {
		return fmt.Errorf("queen condition requires queen observed")
	}
	if t == TypeQueen {
		return nil
	}
	if a.BroodAmount != nil && *a.BroodAmount != BroodAmountLow && *a.BroodAmount != BroodAmountModerate && *a.BroodAmount != BroodAmountHigh {
		return fmt.Errorf("invalid brood amount")
	}
	if a.BroodPattern != nil && *a.BroodPattern != BroodPatternSolid && *a.BroodPattern != BroodPatternMixed && *a.BroodPattern != BroodPatternSpotty {
		return fmt.Errorf("invalid brood pattern")
	}
	if a.BroodConcerns != nil && *a.BroodConcerns != BroodConcernsNone && *a.BroodConcerns != BroodConcernsObserved && *a.BroodConcerns != BroodConcernsUnsure {
		return fmt.Errorf("invalid brood concerns")
	}
	if a.BroodStages != nil {
		seen := map[BroodStage]struct{}{}
		for _, stage := range *a.BroodStages {
			if stage != BroodStageEggs && stage != BroodStageLarvae && stage != BroodStageCapped {
				return fmt.Errorf("invalid brood stage")
			}
			if _, exists := seen[stage]; exists {
				return fmt.Errorf("duplicate brood stage")
			}
			seen[stage] = struct{}{}
		}
	}
	return nil
}

func validateAllowedFields(a *Assessment, t Type) error {
	allowed := map[Type]map[assessmentField]struct{}{
		TypeRoutine: {
			fieldColonyStrength: {}, fieldQueenStatus: {}, fieldBroodStatus: {},
			fieldFoodStores: {}, fieldHealthConcerns: {},
		},
		TypeQueen: {
			fieldQueenObserved: {}, fieldEggsObserved: {}, fieldQueenCells: {},
			fieldQueenCondition: {},
		},
		TypeBrood: {
			fieldBroodAmount: {}, fieldBroodPattern: {}, fieldBroodStages: {},
			fieldBroodConcerns: {},
		},
		TypeHealth: {
			fieldHealthOverallCondition: {}, fieldPestSigns: {},
			fieldHealthWarningSigns: {}, fieldHealthConcernLevel: {},
		},
		TypeFeeding: {
			fieldFoodStores: {}, fieldFeedingNeed: {}, fieldFeedingPerformed: {},
			fieldFeedTypes: {},
		},
		TypeSeasonal: {
			fieldSeason: {}, fieldColonyStrength: {},
			fieldSeasonalStoreReadiness: {}, fieldSeasonalReadiness: {},
			fieldSeasonalConcerns: {},
		},
	}
	fields := a.presentFields()
	for field := range fields {
		if _, ok := allowed[t][field]; !ok {
			return fmt.Errorf("assessment field %q is not valid for %s", field, t)
		}
	}
	return nil
}

func (a *Assessment) presentFields() map[assessmentField]struct{} {
	fields := make(map[assessmentField]struct{})
	if a.ColonyStrength != nil {
		fields[fieldColonyStrength] = struct{}{}
	}
	if a.QueenStatus != nil {
		fields[fieldQueenStatus] = struct{}{}
	}
	if a.BroodStatus != nil {
		fields[fieldBroodStatus] = struct{}{}
	}
	if a.FoodStores != nil {
		fields[fieldFoodStores] = struct{}{}
	}
	if a.HealthConcerns != nil {
		fields[fieldHealthConcerns] = struct{}{}
	}
	if a.QueenObserved != nil {
		fields[fieldQueenObserved] = struct{}{}
	}
	if a.EggsObserved != nil {
		fields[fieldEggsObserved] = struct{}{}
	}
	if a.QueenCells != nil {
		fields[fieldQueenCells] = struct{}{}
	}
	if a.QueenCondition != nil {
		fields[fieldQueenCondition] = struct{}{}
	}
	if a.BroodAmount != nil {
		fields[fieldBroodAmount] = struct{}{}
	}
	if a.BroodPattern != nil {
		fields[fieldBroodPattern] = struct{}{}
	}
	if a.BroodStages != nil {
		fields[fieldBroodStages] = struct{}{}
	}
	if a.BroodConcerns != nil {
		fields[fieldBroodConcerns] = struct{}{}
	}
	if a.HealthOverallCondition != nil {
		fields[fieldHealthOverallCondition] = struct{}{}
	}
	if a.PestSigns != nil {
		fields[fieldPestSigns] = struct{}{}
	}
	if a.HealthWarningSigns != nil {
		fields[fieldHealthWarningSigns] = struct{}{}
	}
	if a.HealthConcernLevel != nil {
		fields[fieldHealthConcernLevel] = struct{}{}
	}
	if a.FeedingNeed != nil {
		fields[fieldFeedingNeed] = struct{}{}
	}
	if a.FeedingPerformed != nil {
		fields[fieldFeedingPerformed] = struct{}{}
	}
	if a.FeedTypes != nil {
		fields[fieldFeedTypes] = struct{}{}
	}
	if a.Season != nil {
		fields[fieldSeason] = struct{}{}
	}
	if a.SeasonalStoreReadiness != nil {
		fields[fieldSeasonalStoreReadiness] = struct{}{}
	}
	if a.SeasonalReadiness != nil {
		fields[fieldSeasonalReadiness] = struct{}{}
	}
	if a.SeasonalConcerns != nil {
		fields[fieldSeasonalConcerns] = struct{}{}
	}
	return fields
}

func validatePestSigns(values *[]PestSign) error {
	seen := map[PestSign]struct{}{}
	if values == nil {
		return nil
	}
	for _, value := range *values {
		if value != PestSignVarroaMites && value != PestSignWaxMoth && value != PestSignSmallHiveBeetle && value != PestSignOther {
			return fmt.Errorf("invalid pest sign")
		}
		if _, exists := seen[value]; exists {
			return fmt.Errorf("duplicate pest sign")
		}
		seen[value] = struct{}{}
	}
	return nil
}

func validateHealthWarningSigns(values *[]HealthWarningSign) error {
	seen := map[HealthWarningSign]struct{}{}
	if values == nil {
		return nil
	}
	for _, value := range *values {
		if value != HealthWarningAbnormalBrood && value != HealthWarningDeformedWings && value != HealthWarningUnusualBeeMortality && value != HealthWarningDiarrheaSigns && value != HealthWarningOther {
			return fmt.Errorf("invalid health warning sign")
		}
		if _, exists := seen[value]; exists {
			return fmt.Errorf("duplicate health warning sign")
		}
		seen[value] = struct{}{}
	}
	return nil
}

func validateFeedTypes(values *[]FeedType) error {
	seen := map[FeedType]struct{}{}
	if values == nil {
		return nil
	}
	for _, value := range *values {
		if value != FeedTypeSugarSyrup && value != FeedTypeFondant && value != FeedTypeDrySugar && value != FeedTypePollenSubstitute && value != FeedTypeOther {
			return fmt.Errorf("invalid feed type")
		}
		if _, exists := seen[value]; exists {
			return fmt.Errorf("duplicate feed type")
		}
		seen[value] = struct{}{}
	}
	return nil
}

func validateSeasonalConcerns(values *[]SeasonalConcern) error {
	seen := map[SeasonalConcern]struct{}{}
	if values == nil {
		return nil
	}
	for _, value := range *values {
		if value != SeasonalConcernFoodStores && value != SeasonalConcernColonyStrength && value != SeasonalConcernQueen && value != SeasonalConcernBrood && value != SeasonalConcernPestsOrDisease && value != SeasonalConcernHiveCondition && value != SeasonalConcernOther {
			return fmt.Errorf("invalid seasonal concern")
		}
		if _, exists := seen[value]; exists {
			return fmt.Errorf("duplicate seasonal concern")
		}
		seen[value] = struct{}{}
	}
	return nil
}

package inspection

import "fmt"

const AssessmentVersion = 1

type ColonyStrength string
type QueenStatus string
type BroodStatus string
type FoodStores string
type HealthConcerns string

const (
	ColonyStrengthWeak       ColonyStrength = "WEAK"
	ColonyStrengthModerate   ColonyStrength = "MODERATE"
	ColonyStrengthStrong     ColonyStrength = "STRONG"
	QueenStatusHealthy       QueenStatus    = "HEALTHY"
	QueenStatusProblem       QueenStatus    = "PROBLEM"
	QueenStatusNotChecked    QueenStatus    = "NOT_CHECKED"
	BroodStatusHealthy       BroodStatus    = "HEALTHY"
	BroodStatusProblem       BroodStatus    = "PROBLEM"
	BroodStatusNotChecked    BroodStatus    = "NOT_CHECKED"
	FoodStoresLow            FoodStores     = "LOW"
	FoodStoresAdequate       FoodStores     = "ADEQUATE"
	FoodStoresAbundant       FoodStores     = "ABUNDANT"
	FoodStoresNotChecked     FoodStores     = "NOT_CHECKED"
	HealthConcernsNone       HealthConcerns = "NONE"
	HealthConcernsPresent    HealthConcerns = "PRESENT"
	HealthConcernsNotChecked HealthConcerns = "NOT_CHECKED"
)

// Assessment is explicit and typed. A nil field means that metric was not answered.
type Assessment struct {
	Version        int
	ColonyStrength *ColonyStrength
	QueenStatus    *QueenStatus
	BroodStatus    *BroodStatus
	FoodStores     *FoodStores
	HealthConcerns *HealthConcerns
}

func (a *Assessment) ValidateFor(t Type) error {
	if a == nil {
		return nil
	}
	if t != TypeRoutine {
		return fmt.Errorf("assessment is not supported for %s", t)
	}
	if a.Version == 0 {
		a.Version = AssessmentVersion
	}
	if a.Version != AssessmentVersion {
		return fmt.Errorf("unsupported assessment version %d", a.Version)
	}
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

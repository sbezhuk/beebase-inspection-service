package inspection

import "testing"

func TestAssessmentValidation(t *testing.T) {
	queen := QueenStatusProblem
	assessment := &Assessment{QueenStatus: &queen}
	if err := assessment.ValidateFor(TypeRoutine); err != nil {
		t.Fatalf("valid partial routine assessment: %v", err)
	}
	if assessment.Version != AssessmentVersion {
		t.Fatalf("version = %d, want %d", assessment.Version, AssessmentVersion)
	}

	invalid := QueenStatus("ABSENT")
	if err := (&Assessment{QueenStatus: &invalid}).ValidateFor(TypeRoutine); err == nil {
		t.Fatal("invalid enum accepted")
	}
	if err := (&Assessment{}).ValidateFor(Type("INVALID")); err == nil {
		t.Fatal("assessment accepted for unsupported inspection type")
	}
}

func TestQueenAssessmentValidation(t *testing.T) {
	observed := QueenObservedObserved
	condition := QueenConditionConcern
	eggs := EggsObservedYes
	cells := QueenCellsPresent
	assessment := &Assessment{
		QueenObserved:  &observed,
		EggsObserved:   &eggs,
		QueenCells:     &cells,
		QueenCondition: &condition,
	}
	if err := assessment.ValidateFor(TypeQueen); err != nil {
		t.Fatalf("valid queen assessment: %v", err)
	}
	if assessment.Version != AssessmentVersion {
		t.Fatalf("version = %d, want %d", assessment.Version, AssessmentVersion)
	}

	notObserved := QueenObservedNotObserved
	invalid := &Assessment{QueenObserved: &notObserved, QueenCondition: &condition}
	if err := invalid.ValidateFor(TypeQueen); err == nil {
		t.Fatal("queen condition accepted when queen was not observed")
	}

	if err := (&Assessment{QueenObserved: &notObserved}).ValidateFor(TypeQueen); err != nil {
		t.Fatalf("NOT_OBSERVED should be a valid observation: %v", err)
	}
}

func TestBroodAssessmentValidation(t *testing.T) {
	stages := []BroodStage{BroodStageEggs, BroodStageCapped}
	assessment := &Assessment{
		BroodAmount:   ptr(BroodAmountModerate),
		BroodPattern:  ptr(BroodPatternSolid),
		BroodStages:   &stages,
		BroodConcerns: ptr(BroodConcernsNone),
	}
	if err := assessment.ValidateFor(TypeBrood); err != nil {
		t.Fatalf("valid brood assessment: %v", err)
	}

	duplicate := []BroodStage{BroodStageEggs, BroodStageEggs}
	if err := (&Assessment{BroodStages: &duplicate}).ValidateFor(TypeBrood); err == nil {
		t.Fatal("duplicate brood stage accepted")
	}

	if err := (&Assessment{BroodStages: &[]BroodStage{BroodStage("INVALID")}}).ValidateFor(TypeBrood); err == nil {
		t.Fatal("invalid brood stage accepted")
	}
}

func TestHealthAssessmentValidation(t *testing.T) {
	pests := []PestSign{PestSignVarroaMites, PestSignOther}
	warnings := []HealthWarningSign{HealthWarningAbnormalBrood, HealthWarningDeformedWings}
	assessment := &Assessment{
		HealthOverallCondition: &[]HealthOverallCondition{HealthOverallFair}[0],
		PestSigns:              &pests,
		HealthWarningSigns:     &warnings,
		HealthConcernLevel:     &[]HealthConcernLevel{HealthConcernModerate}[0],
	}
	if err := assessment.ValidateFor(TypeHealth); err != nil {
		t.Fatalf("valid health assessment: %v", err)
	}

	duplicate := []PestSign{PestSignOther, PestSignOther}
	if err := (&Assessment{PestSigns: &duplicate}).ValidateFor(TypeHealth); err == nil {
		t.Fatal("duplicate pest sign accepted")
	}

	invalid := []HealthWarningSign{HealthWarningSign("DIAGNOSIS")}
	if err := (&Assessment{HealthWarningSigns: &invalid}).ValidateFor(TypeHealth); err == nil {
		t.Fatal("invalid health warning sign accepted")
	}

	if err := (&Assessment{BroodAmount: &[]BroodAmount{BroodAmountHigh}[0]}).ValidateFor(TypeHealth); err == nil {
		t.Fatal("BROOD field accepted for HEALTH")
	}
}

func TestFeedingAssessmentValidation(t *testing.T) {
	feedTypes := []FeedType{FeedTypeSugarSyrup, FeedTypePollenSubstitute}
	assessment := &Assessment{
		FoodStores:       ptr(FoodStoresLow),
		FeedingNeed:      ptr(FeedingNeedSoon),
		FeedingPerformed: ptr(FeedingPerformedYes),
		FeedTypes:        &feedTypes,
	}
	if err := assessment.ValidateFor(TypeFeeding); err != nil {
		t.Fatalf("valid FEEDING assessment: %v", err)
	}
	if err := (&Assessment{FeedingPerformed: ptr(FeedingPerformedYes)}).ValidateFor(TypeFeeding); err != nil {
		t.Fatalf("unanswered feed type should be valid: %v", err)
	}

	stale := []FeedType{FeedTypeFondant}
	if err := (&Assessment{FeedingPerformed: ptr(FeedingPerformedNo), FeedTypes: &stale}).ValidateFor(TypeFeeding); err == nil {
		t.Fatal("feed type accepted when feeding was not performed")
	}
	duplicate := []FeedType{FeedTypeOther, FeedTypeOther}
	if err := (&Assessment{FeedingPerformed: ptr(FeedingPerformedYes), FeedTypes: &duplicate}).ValidateFor(TypeFeeding); err == nil {
		t.Fatal("duplicate feed type accepted")
	}
	invalid := []FeedType{FeedType("INVALID")}
	if err := (&Assessment{FeedingPerformed: ptr(FeedingPerformedYes), FeedTypes: &invalid}).ValidateFor(TypeFeeding); err == nil {
		t.Fatal("invalid feed type accepted")
	}
}

func TestSeasonalAssessmentValidation(t *testing.T) {
	concerns := []SeasonalConcern{SeasonalConcernFoodStores, SeasonalConcernHiveCondition}
	assessment := &Assessment{
		Season:                 ptr(SeasonalSpring),
		ColonyStrength:         ptr(ColonyStrengthStrong),
		SeasonalStoreReadiness: ptr(SeasonalStoresMarginal),
		SeasonalReadiness:      ptr(SeasonalReadinessNeedsAttention),
		SeasonalConcerns:       &concerns,
	}
	if err := assessment.ValidateFor(TypeSeasonal); err != nil {
		t.Fatalf("valid SEASONAL assessment: %v", err)
	}
	if err := (&Assessment{SeasonalConcerns: &[]SeasonalConcern{}}).ValidateFor(TypeSeasonal); err != nil {
		t.Fatalf("empty concerns should be valid: %v", err)
	}
	if err := (&Assessment{Season: ptr(SeasonalPhase("INVALID"))}).ValidateFor(TypeSeasonal); err == nil {
		t.Fatal("invalid season accepted")
	}
	if err := (&Assessment{SeasonalConcerns: &[]SeasonalConcern{SeasonalConcernQueen, SeasonalConcernQueen}}).ValidateFor(TypeSeasonal); err == nil {
		t.Fatal("duplicate seasonal concern accepted")
	}
	if err := (&Assessment{SeasonalConcerns: &[]SeasonalConcern{SeasonalConcern("INVALID")}}).ValidateFor(TypeSeasonal); err == nil {
		t.Fatal("invalid seasonal concern accepted")
	}
	if err := (&Assessment{FoodStores: ptr(FoodStoresLow)}).ValidateFor(TypeSeasonal); err == nil {
		t.Fatal("routine food stores accepted for SEASONAL")
	}
	if err := (&Assessment{Season: ptr(SeasonalSpring)}).ValidateFor(TypeRoutine); err == nil {
		t.Fatal("seasonal field accepted for ROUTINE")
	}
}

func TestAssessmentValidation_IsolatesSchemas(t *testing.T) {
	type fieldCase struct {
		name   string
		owners []Type
		value  *Assessment
	}
	seasonalConcerns := []SeasonalConcern{SeasonalConcernFoodStores}
	feedTypes := []FeedType{FeedTypeSugarSyrup}
	fields := []fieldCase{
		{"ROUTINE.colonyStrength", []Type{TypeRoutine, TypeSeasonal}, &Assessment{ColonyStrength: ptr(ColonyStrengthStrong)}},
		{"ROUTINE.queenStatus", []Type{TypeRoutine}, &Assessment{QueenStatus: ptr(QueenStatusHealthy)}},
		{"ROUTINE.broodStatus", []Type{TypeRoutine}, &Assessment{BroodStatus: ptr(BroodStatusHealthy)}},
		{"ROUTINE.foodStores", []Type{TypeRoutine, TypeFeeding}, &Assessment{FoodStores: ptr(FoodStoresAdequate)}},
		{"ROUTINE.healthConcerns", []Type{TypeRoutine}, &Assessment{HealthConcerns: ptr(HealthConcernsNone)}},
		{"QUEEN.queenObserved", []Type{TypeQueen}, &Assessment{QueenObserved: ptr(QueenObservedObserved)}},
		{"QUEEN.eggsObserved", []Type{TypeQueen}, &Assessment{EggsObserved: ptr(EggsObservedYes)}},
		{"QUEEN.queenCells", []Type{TypeQueen}, &Assessment{QueenCells: ptr(QueenCellsNone)}},
		{"QUEEN.queenCondition", []Type{TypeQueen}, &Assessment{QueenCondition: ptr(QueenConditionNormal), QueenObserved: ptr(QueenObservedObserved)}},
		{"BROOD.broodAmount", []Type{TypeBrood}, &Assessment{BroodAmount: ptr(BroodAmountModerate)}},
		{"BROOD.broodPattern", []Type{TypeBrood}, &Assessment{BroodPattern: ptr(BroodPatternSolid)}},
		{"BROOD.broodStages", []Type{TypeBrood}, &Assessment{BroodStages: assessmentSlicePtr(BroodStageEggs)}},
		{"BROOD.broodConcerns", []Type{TypeBrood}, &Assessment{BroodConcerns: ptr(BroodConcernsNone)}},
		{"HEALTH.healthOverallCondition", []Type{TypeHealth}, &Assessment{HealthOverallCondition: ptr(HealthOverallGood)}},
		{"HEALTH.pestSigns", []Type{TypeHealth}, &Assessment{PestSigns: &[]PestSign{PestSignVarroaMites}}},
		{"HEALTH.healthWarningSigns", []Type{TypeHealth}, &Assessment{HealthWarningSigns: &[]HealthWarningSign{HealthWarningAbnormalBrood}}},
		{"HEALTH.healthConcernLevel", []Type{TypeHealth}, &Assessment{HealthConcernLevel: ptr(HealthConcernLow)}},
		{"FEEDING.feedingNeed", []Type{TypeFeeding}, &Assessment{FeedingNeed: ptr(FeedingNeedNo)}},
		{"FEEDING.feedingPerformed", []Type{TypeFeeding}, &Assessment{FeedingPerformed: ptr(FeedingPerformedNo)}},
		{"FEEDING.feedTypes", []Type{TypeFeeding}, &Assessment{FeedingPerformed: ptr(FeedingPerformedYes), FeedTypes: &feedTypes}},
		{"SEASONAL.season", []Type{TypeSeasonal}, &Assessment{Season: ptr(SeasonalSpring)}},
		{"SEASONAL.seasonalStoreReadiness", []Type{TypeSeasonal}, &Assessment{SeasonalStoreReadiness: ptr(SeasonalStoresSufficient)}},
		{"SEASONAL.seasonalReadiness", []Type{TypeSeasonal}, &Assessment{SeasonalReadiness: ptr(SeasonalReadinessReady)}},
		{"SEASONAL.seasonalConcerns", []Type{TypeSeasonal}, &Assessment{SeasonalConcerns: &seasonalConcerns}},
	}
	for _, field := range fields {
		field := field
		t.Run(field.name, func(t *testing.T) {
			for _, typ := range Types {
				allowed := false
				for _, owner := range field.owners {
					allowed = allowed || typ == owner
				}
				err := field.value.ValidateFor(typ)
				if allowed && err != nil {
					t.Errorf("ValidateFor(%s) = %v, want valid", typ, err)
				}
				if !allowed && err == nil {
					t.Errorf("ValidateFor(%s) accepted foreign field", typ)
				}
			}
		})
	}
}

func TestAllAssessmentSchemasAllowValidEmptyMultiSelects(t *testing.T) {
	cases := map[Type]*Assessment{
		TypeBrood:    {BroodStages: &[]BroodStage{}},
		TypeHealth:   {PestSigns: &[]PestSign{}, HealthWarningSigns: &[]HealthWarningSign{}},
		TypeFeeding:  {FeedingPerformed: ptr(FeedingPerformedYes), FeedTypes: &[]FeedType{}},
		TypeSeasonal: {SeasonalConcerns: &[]SeasonalConcern{}},
	}
	for typ, assessment := range cases {
		if err := assessment.ValidateFor(typ); err != nil {
			t.Errorf("ValidateFor(%s) rejected valid empty multi-select: %v", typ, err)
		}
	}
}

func ptr[T any](value T) *T { return &value }

func assessmentSlicePtr[T any](values ...T) *[]T { return &values }

//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sbezhuk/beebase-inspection-service/internal/domain/inspection"
	repopostgres "github.com/sbezhuk/beebase-inspection-service/internal/repository/postgres"
)

func TestInspectionRepository_AssessmentsRoundTripThroughPostgres(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	t.Cleanup(func() { _ = tx.Rollback(ctx) })
	repo := repopostgres.NewInspectionRepository(tx)

	brood := inspection.New(uuid.New(), uuid.New(), inspectedAt(), "legacy brood inspection", inspection.TypeBrood)
	if err := repo.Create(ctx, brood); err != nil {
		t.Fatalf("create legacy BROOD assessment-less inspection: %v", err)
	}
	legacy, err := repo.GetByID(ctx, brood.UserID, brood.ID)
	if err != nil {
		t.Fatalf("read legacy BROOD inspection after migration: %v", err)
	}
	if legacy.Assessment != nil {
		t.Fatal("legacy BROOD inspection unexpectedly received an assessment")
	}

	stages := []inspection.BroodStage{inspection.BroodStageEggs, inspection.BroodStageLarvae, inspection.BroodStageCapped}
	brood.Assessment = &inspection.Assessment{BroodStages: &stages}
	brood.UpdatedAt = time.Now().UTC()
	if err := repo.Update(ctx, brood); err != nil {
		t.Fatalf("update BROOD assessment: %v", err)
	}
	gotBrood, err := repo.GetByID(ctx, brood.UserID, brood.ID)
	if err != nil {
		t.Fatalf("read BROOD assessment: %v", err)
	}
	if gotBrood.Assessment == nil || gotBrood.Assessment.BroodStages == nil || len(*gotBrood.Assessment.BroodStages) != 3 {
		t.Fatalf("BROOD stages = %#v, want three values", gotBrood.Assessment)
	}

	emptyStages := []inspection.BroodStage{}
	brood.Assessment.BroodStages = &emptyStages
	brood.UpdatedAt = time.Now().UTC()
	if err := repo.Update(ctx, brood); err != nil {
		t.Fatalf("clear BROOD stages to empty array: %v", err)
	}
	gotBrood, err = repo.GetByID(ctx, brood.UserID, brood.ID)
	if err != nil {
		t.Fatalf("read empty BROOD stages: %v", err)
	}
	if gotBrood.Assessment == nil || gotBrood.Assessment.BroodStages == nil || len(*gotBrood.Assessment.BroodStages) != 0 {
		t.Fatalf("empty BROOD stages = %#v, want non-nil empty slice", gotBrood.Assessment)
	}

	brood.Assessment = nil
	brood.UpdatedAt = time.Now().UTC()
	if err := repo.Update(ctx, brood); err != nil {
		t.Fatalf("clear BROOD assessment to NULL: %v", err)
	}
	gotBrood, err = repo.GetByID(ctx, brood.UserID, brood.ID)
	if err != nil {
		t.Fatalf("read cleared BROOD assessment: %v", err)
	}
	if gotBrood.Assessment != nil {
		t.Fatal("cleared BROOD assessment was not NULL")
	}

	health := inspection.New(uuid.New(), uuid.New(), inspectedAt(), "observed mites", inspection.TypeHealth)
	emptyPests := []inspection.PestSign{}
	warnings := []inspection.HealthWarningSign{inspection.HealthWarningAbnormalBrood, inspection.HealthWarningDeformedWings}
	overall := inspection.HealthOverallFair
	concern := inspection.HealthConcernModerate
	health.Assessment = &inspection.Assessment{
		HealthOverallCondition: &overall,
		PestSigns:              &emptyPests,
		HealthWarningSigns:     &warnings,
		HealthConcernLevel:     &concern,
	}
	if err := repo.Create(ctx, health); err != nil {
		t.Fatalf("create HEALTH assessment: %v", err)
	}
	gotHealth, err := repo.GetByID(ctx, health.UserID, health.ID)
	if err != nil {
		t.Fatalf("read HEALTH assessment: %v", err)
	}
	if gotHealth.Assessment == nil || gotHealth.Assessment.PestSigns == nil || len(*gotHealth.Assessment.PestSigns) != 0 {
		t.Fatalf("empty HEALTH pest signs = %#v, want non-nil empty slice", gotHealth.Assessment)
	}
	if gotHealth.Assessment.HealthWarningSigns == nil || len(*gotHealth.Assessment.HealthWarningSigns) != 2 {
		t.Fatalf("HEALTH warning signs = %#v, want two values", gotHealth.Assessment)
	}

	pests := []inspection.PestSign{inspection.PestSignVarroaMites, inspection.PestSignSmallHiveBeetle}
	health.Assessment.PestSigns = &pests
	health.UpdatedAt = time.Now().UTC()
	if err := repo.Update(ctx, health); err != nil {
		t.Fatalf("update HEALTH pest signs: %v", err)
	}
	gotHealth, err = repo.GetByID(ctx, health.UserID, health.ID)
	if err != nil {
		t.Fatalf("read updated HEALTH assessment: %v", err)
	}
	if gotHealth.Assessment == nil || gotHealth.Assessment.PestSigns == nil || len(*gotHealth.Assessment.PestSigns) != 2 {
		t.Fatalf("updated HEALTH pest signs = %#v, want two values", gotHealth.Assessment)
	}

	seasonal := inspection.New(uuid.New(), uuid.New(), inspectedAt(), "seasonal readiness", inspection.TypeSeasonal)
	season := inspection.SeasonalAutumn
	strength := inspection.ColonyStrengthStrong
	stores := inspection.SeasonalStoresMarginal
	readiness := inspection.SeasonalReadinessNeedsAttention
	concerns := []inspection.SeasonalConcern{inspection.SeasonalConcernFoodStores, inspection.SeasonalConcernColonyStrength, inspection.SeasonalConcernHiveCondition}
	seasonal.Assessment = &inspection.Assessment{
		Season:                 &season,
		ColonyStrength:         &strength,
		SeasonalStoreReadiness: &stores,
		SeasonalReadiness:      &readiness,
		SeasonalConcerns:       &concerns,
	}
	if err := repo.Create(ctx, seasonal); err != nil {
		t.Fatalf("create SEASONAL assessment: %v", err)
	}
	gotSeasonal, err := repo.GetByID(ctx, seasonal.UserID, seasonal.ID)
	if err != nil {
		t.Fatalf("read SEASONAL assessment: %v", err)
	}
	if gotSeasonal.Assessment == nil || gotSeasonal.Assessment.Season == nil || *gotSeasonal.Assessment.Season != inspection.SeasonalAutumn || gotSeasonal.Assessment.SeasonalConcerns == nil || len(*gotSeasonal.Assessment.SeasonalConcerns) != 3 {
		t.Fatalf("SEASONAL assessment = %#v, want phase and three concerns", gotSeasonal.Assessment)
	}
	emptyConcerns := []inspection.SeasonalConcern{}
	seasonal.Assessment.SeasonalConcerns = &emptyConcerns
	seasonal.UpdatedAt = time.Now().UTC()
	if err := repo.Update(ctx, seasonal); err != nil {
		t.Fatalf("clear SEASONAL concerns to empty array: %v", err)
	}
	gotSeasonal, err = repo.GetByID(ctx, seasonal.UserID, seasonal.ID)
	if err != nil || gotSeasonal.Assessment == nil || gotSeasonal.Assessment.SeasonalConcerns == nil || len(*gotSeasonal.Assessment.SeasonalConcerns) != 0 {
		t.Fatalf("empty SEASONAL concerns = %#v, err %v", gotSeasonal.Assessment, err)
	}
}

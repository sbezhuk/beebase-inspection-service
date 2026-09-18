package inspection_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	appinspection "github.com/sbezhuk/beebase-inspection-service/internal/application/inspection"
	"github.com/sbezhuk/beebase-inspection-service/internal/domain/health"
	"github.com/sbezhuk/beebase-inspection-service/internal/domain/inspection"
)

func TestGetHiveHealthCombinesFullHistoryBeforeEvaluation(t *testing.T) {
	repo := newFakeRepo()
	verifier := newFakeHiveVerifier()
	service := appinspection.NewService(repo, verifier, newFakeMediaClient(), 14)
	userID := uuid.New()
	hiveID := uuid.New()
	verifier.allow("token", hiveID)

	day1 := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	day8 := day1.Add(7 * 24 * time.Hour)
	low := inspection.FoodStoresLow
	adequate := inspection.FoodStoresAdequate
	for _, current := range []struct {
		at    time.Time
		store *inspection.FoodStores
	}{
		{day1, &low},
		{day8, &adequate},
	} {
		created := inspection.New(userID, hiveID, current.at, "", inspection.TypeFeeding)
		created.Assessment = &inspection.Assessment{Version: 1, FoodStores: current.store}
		if err := repo.Create(context.Background(), created); err != nil {
			t.Fatalf("seed inspection: %v", err)
		}
	}

	got, err := service.GetHiveHealth(context.Background(), userID, "token", hiveID, day8)
	if err != nil {
		t.Fatalf("GetHiveHealth: %v", err)
	}
	if got.State != health.DimensionWatch {
		// One nutrition dimension alone cannot satisfy aggregate GOOD, and the
		// latest food-store value should not leave a stale concern behind.
		t.Fatalf("aggregate state = %q, want WATCH", got.State)
	}
	nutrition := findDimension(got.Dimensions, health.DimensionNutrition)
	if nutrition.State != health.DimensionGood {
		t.Fatalf("nutrition state = %q, want GOOD", nutrition.State)
	}
}

func TestGetHiveHealthEmptyHistoryReturnsUnknownNone(t *testing.T) {
	verifier := newFakeHiveVerifier()
	hiveID := uuid.New()
	verifier.allow("token", hiveID)
	service := appinspection.NewService(newFakeRepo(), verifier, newFakeMediaClient(), 14)

	got, err := service.GetHiveHealth(context.Background(), uuid.New(), "token", hiveID, time.Date(2026, 9, 18, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("GetHiveHealth: %v", err)
	}
	if got.State != health.DimensionUnknown || got.Coverage != health.CoverageNone {
		t.Fatalf("result = (%q, %q), want (UNKNOWN, NONE)", got.State, got.Coverage)
	}
}

func TestGetHiveHealthRequiresHiveAccess(t *testing.T) {
	service := appinspection.NewService(newFakeRepo(), newFakeHiveVerifier(), newFakeMediaClient(), 14)
	_, err := service.GetHiveHealth(context.Background(), uuid.New(), "attacker", uuid.New(), time.Now().UTC())
	if !errors.Is(err, appinspection.ErrHiveNotFound) {
		t.Fatalf("GetHiveHealth error = %v, want ErrHiveNotFound", err)
	}
}

func findDimension(dimensions []health.DimensionEvaluation, dimension health.HealthDimension) health.DimensionEvaluation {
	for _, current := range dimensions {
		if current.Dimension == dimension {
			return current
		}
	}
	panic("missing dimension")
}

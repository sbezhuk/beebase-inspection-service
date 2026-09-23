package inspection_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sbezhuk/beebase-health/health"
	appinspection "github.com/sbezhuk/beebase-inspection-service/internal/application/inspection"
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

func TestGetHiveHealthMatchesSameDateHistoryPointAtAnyClockTime(t *testing.T) {
	repo := newFakeRepo()
	verifier := newFakeHiveVerifier()
	userID := uuid.New()
	hiveID := uuid.New()
	verifier.allow("token", hiveID)

	// inspectedAt is a persisted calendar date. At exactly 30 calendar days,
	// the date-only history point is still RECENT; a wall-clock asOf later on
	// that same date must not turn the same evidence STALE.
	recorded := time.Date(2026, 8, 21, 0, 0, 0, 0, time.UTC)
	asOfDate := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	foodStores := inspection.FoodStoresAdequate
	created := inspection.New(userID, hiveID, recorded, "", inspection.TypeFeeding)
	created.Assessment = &inspection.Assessment{Version: 1, FoodStores: &foodStores}
	if err := repo.Create(context.Background(), created); err != nil {
		t.Fatalf("seed inspection: %v", err)
	}

	service := appinspection.NewService(repo, verifier, newFakeMediaClient(), 14, newFakeEntitlementResolver())
	live, err := service.GetHiveHealth(context.Background(), userID, "token", hiveID, asOfDate)
	if err != nil {
		t.Fatalf("GetHiveHealth: %v", err)
	}
	history, err := service.GetHiveHealthHistory(context.Background(), userID, "token", hiveID, asOfDate, asOfDate)
	if err != nil {
		t.Fatalf("GetHiveHealthHistory: %v", err)
	}

	point := history.Points[0].Evaluation
	if live.State != point.State || live.Coverage != point.Coverage {
		t.Fatalf("same-date current/history aggregate diverged: live=(%q,%q), history=(%q,%q)", live.State, live.Coverage, point.State, point.Coverage)
	}
	for _, dimension := range live.Dimensions {
		other := findDimension(point.Dimensions, dimension.Dimension)
		if dimension.State != other.State || dimension.Coverage != other.Coverage {
			t.Fatalf("same-date %s diverged: live=(%q,%q), history=(%q,%q)", dimension.Dimension, dimension.State, dimension.Coverage, other.State, other.Coverage)
		}
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

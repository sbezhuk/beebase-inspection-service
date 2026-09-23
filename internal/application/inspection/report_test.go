package inspection_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	appinspection "github.com/sbezhuk/beebase-inspection-service/internal/application/inspection"
	"github.com/sbezhuk/beebase-inspection-service/internal/domain/inspection"
)

func TestGetInternalReportDataUsesCanonicalHistoryAndInclusiveInspections(t *testing.T) {
	repo := newFakeRepo()
	service := appinspection.NewService(repo, newFakeHiveVerifier(), newFakeMediaClient(), 14)
	hiveID := uuid.New()
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC)
	for _, day := range []time.Time{from.AddDate(0, 0, -1), from, to, to.AddDate(0, 0, 1)} {
		if err := repo.Create(context.Background(), inspection.New(uuid.New(), hiveID, day, "", inspection.TypeRoutine)); err != nil {
			t.Fatal(err)
		}
	}

	history, current, err := service.GetInternalReportData(context.Background(), hiveID, from, to)
	if err != nil {
		t.Fatalf("GetInternalReportData: %v", err)
	}
	if len(history.Points) != 3 {
		t.Fatalf("history points = %d, want 3", len(history.Points))
	}
	if len(history.Inspections) != 2 {
		t.Fatalf("in-range inspections = %d, want 2", len(history.Inspections))
	}
	if current.State == "" {
		t.Fatal("canonical current report health was not calculated")
	}
}

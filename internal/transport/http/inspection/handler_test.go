package inspection

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sbezhuk/beebase-health/health"
	appinspection "github.com/sbezhuk/beebase-inspection-service/internal/application/inspection"
	"github.com/sbezhuk/beebase-inspection-service/internal/domain/inspection"
)

func TestNewColonyHealthResponseUsesCalculatedStateAndCanonicalDimensions(t *testing.T) {
	asOf := time.Date(2026, 9, 18, 18, 30, 0, 0, time.UTC)
	evaluation := health.ColonyHealthEvaluation{
		State:    health.DimensionWatch,
		Coverage: health.CoverageHigh,
		Dimensions: []health.DimensionEvaluation{
			{Dimension: health.DimensionStrength, State: health.DimensionGood, Coverage: health.CoverageMedium},
			{Dimension: health.DimensionQueen, State: health.DimensionGood, Coverage: health.CoverageHigh},
			{Dimension: health.DimensionBrood, State: health.DimensionWatch, Coverage: health.CoverageMedium},
			{Dimension: health.DimensionNutrition, State: health.DimensionGood, Coverage: health.CoverageHigh},
			{Dimension: health.DimensionPestsAndDisease, State: health.DimensionGood, Coverage: health.CoverageLow},
			{Dimension: health.DimensionOverall, State: health.DimensionConcern, Coverage: health.CoverageMedium},
		},
	}
	response := newColonyHealthResponse(asOf, evaluation)
	if response.AsOf != asOf || response.State != health.DimensionWatch || response.Coverage != health.CoverageHigh {
		t.Fatalf("response header = %#v", response)
	}
	if len(response.Dimensions) != 6 || response.Dimensions[5].Dimension != health.DimensionOverall {
		t.Fatalf("dimensions = %#v", response.Dimensions)
	}
	if response.Dimensions[5].State != health.DimensionConcern {
		t.Fatalf("beekeeper OVERALL was changed: %#v", response.Dimensions[5])
	}
}

func TestCurrentHealthAsOfUsesUTCDateBoundary(t *testing.T) {
	now := time.Date(2026, 9, 20, 23, 45, 12, 0, time.FixedZone("local", 2*60*60))
	got := currentHealthAsOf(now)
	want := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("currentHealthAsOf = %v, want %v", got, want)
	}
}

func TestWriteServiceError(t *testing.T) {
	h := NewHandler(nil, slog.New(slog.NewTextHandler(io.Discard, nil)), "")

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{
			name:       "inspection not found",
			err:        inspection.ErrNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   CodeInspectionNotFound,
		},
		{
			name:       "hive not found",
			err:        appinspection.ErrHiveNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   CodeHiveNotFound,
		},
		{
			name:       "media limit reached",
			err:        appinspection.ErrMediaLimitReached,
			wantStatus: http.StatusBadRequest,
			wantCode:   CodeMediaLimitReached,
		},
		{
			name:       "internal error",
			err:        errors.New("db explosion"),
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.writeServiceError(rec, tt.err)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			var body struct {
				Error struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if body.Error.Code != tt.wantCode {
				t.Fatalf("code = %q, want %q", body.Error.Code, tt.wantCode)
			}
		})
	}
}

func TestWriteServiceError_ImageNotFound(t *testing.T) {
	h := NewHandler(nil, slog.New(slog.NewTextHandler(io.Discard, nil)), "")
	rec := httptest.NewRecorder()
	h.writeServiceError(rec, appinspection.ErrImageNotFound)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	var body struct {
		Error struct {
			Code   string            `json:"code"`
			Fields map[string]string `json:"fields"`
		} `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Error.Code != "validation_error" {
		t.Fatalf("code = %q, want %q", body.Error.Code, "validation_error")
	}
	if body.Error.Fields["images"] != CodeImageNotFound {
		t.Fatalf("fields[images] = %q, want %q", body.Error.Fields["images"], CodeImageNotFound)
	}
}

//go:build integration

package http_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	inspectionhttp "github.com/sbezhuk/beebase-inspection-service/internal/transport/http/inspection"
)

// TestInspectionFlow_HiveHealth_NotGatedByEntitlement proves the existing
// GET .../health endpoint stays available to a Free caller after adding
// the Pro-only history endpoint alongside it - only /health/history is
// gated, never /health itself.
func TestInspectionFlow_HiveHealth_NotGatedByEntitlement(t *testing.T) {
	stack := newTestStack(t)
	userID := uuid.New()
	hiveID := uuid.New()
	token := stack.tokenFor(t, userID)
	stack.hive.allow(token, hiveID)
	stack.sub.setEntitlement(token, "free")

	resp := stack.request(t, http.MethodGet, "/api/v1/hives/"+hiveID.String()+"/health", token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET .../health for a Free caller: status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var got inspectionhttp.ColonyHealthResponse
	decodeJSON(t, resp, &got)
	if got.State != "UNKNOWN" || got.Coverage != "NONE" {
		t.Fatalf("health = (%q, %q), want (UNKNOWN, NONE) for a hive with no inspections", got.State, got.Coverage)
	}
}

func TestInspectionFlow_HiveHealthHistory_WithoutTokenIsUnauthorized(t *testing.T) {
	stack := newTestStack(t)
	hiveID := uuid.New()

	resp := stack.request(t, http.MethodGet, "/api/v1/hives/"+hiveID.String()+"/health/history?from=2026-06-01&to=2026-06-02", "", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestInspectionFlow_HiveHealthHistory_InvalidHiveID(t *testing.T) {
	stack := newTestStack(t)
	token := stack.tokenFor(t, uuid.New())

	resp := stack.request(t, http.MethodGet, "/api/v1/hives/not-a-uuid/health/history?from=2026-06-01&to=2026-06-02", token, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// TestInspectionFlow_HiveHealthHistory_QueryValidation covers the
// documented validation rules: from/to required and well-formed, from
// must not fall after to, the inclusive range must not exceed 365 daily
// points, and interval (when given) must be "day".
func TestInspectionFlow_HiveHealthHistory_QueryValidation(t *testing.T) {
	stack := newTestStack(t)
	userID := uuid.New()
	hiveID := uuid.New()
	token := stack.tokenFor(t, userID)
	stack.hive.allow(token, hiveID)

	base := "/api/v1/hives/" + hiveID.String() + "/health/history"
	cases := map[string]string{
		"missing from":         base + "?to=2026-06-02",
		"missing to":           base + "?from=2026-06-01",
		"invalid from":         base + "?from=not-a-date&to=2026-06-02",
		"invalid to":           base + "?from=2026-06-01&to=not-a-date",
		"from after to":        base + "?from=2026-06-10&to=2026-06-01",
		"range too long":       base + "?from=2026-01-01&to=2027-01-01",
		"unsupported interval": base + "?from=2026-06-01&to=2026-06-02&interval=week",
	}
	for name, path := range cases {
		t.Run(name, func(t *testing.T) {
			resp := stack.request(t, http.MethodGet, path, token, nil)
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("GET %s: status = %d, want %d", path, resp.StatusCode, http.StatusBadRequest)
			}
			var body struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			decodeJSON(t, resp, &body)
			if body.Error.Code != "validation_error" {
				t.Fatalf("GET %s: error.code = %q, want validation_error", path, body.Error.Code)
			}
		})
	}
}

// TestInspectionFlow_HiveHealthHistory_HiveNotOwnedIsNotFound proves the
// non-disclosure convention every other inspection-service endpoint
// already follows extends to history: a hive that doesn't exist, or
// belongs to someone else, is indistinguishable from the caller's point
// of view - both are 404 hive_not_found, never 403, regardless of the
// caller's own entitlement.
func TestInspectionFlow_HiveHealthHistory_HiveNotOwnedIsNotFound(t *testing.T) {
	stack := newTestStack(t)
	token := stack.tokenFor(t, uuid.New())
	stack.sub.setEntitlement(token, "free")

	resp := stack.request(t, http.MethodGet, "/api/v1/hives/"+uuid.New().String()+"/health/history?from=2026-06-01&to=2026-06-02", token, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	decodeJSON(t, resp, &body)
	if body.Error.Code != "hive_not_found" {
		t.Fatalf("error.code = %q, want hive_not_found", body.Error.Code)
	}
}

// TestInspectionFlow_HiveHealthHistory_FreeUserForbidden proves a Free
// caller who does own the hive still gets 403 with the stable
// health_history_pro_required code (not the writability-flavored
// parent_resource_pro_locked code used elsewhere), so the client can
// route them to the Subscription screen.
func TestInspectionFlow_HiveHealthHistory_FreeUserForbidden(t *testing.T) {
	stack := newTestStack(t)
	userID := uuid.New()
	hiveID := uuid.New()
	token := stack.tokenFor(t, userID)
	stack.hive.allow(token, hiveID)
	stack.sub.setEntitlement(token, "free")

	resp := stack.request(t, http.MethodGet, "/api/v1/hives/"+hiveID.String()+"/health/history?from=2026-06-01&to=2026-06-02", token, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	decodeJSON(t, resp, &body)
	if body.Error.Code != "health_history_pro_required" {
		t.Fatalf("error.code = %q, want health_history_pro_required", body.Error.Code)
	}
}

// TestInspectionFlow_HiveHealthHistory_ProUserSucceeds is the end-to-end
// happy path: a Pro caller who owns the hive gets one point per calendar
// day in the inclusive range, plus the inspection that occurred in it.
func TestInspectionFlow_HiveHealthHistory_ProUserSucceeds(t *testing.T) {
	stack := newTestStack(t)
	userID := uuid.New()
	hiveID := uuid.New()
	token := stack.tokenFor(t, userID)
	stack.hive.allow(token, hiveID)
	stack.sub.setEntitlement(token, "pro")

	createResp := stack.request(t, http.MethodPost, "/api/v1/inspections", token, map[string]any{
		"hiveId":      hiveID.String(),
		"inspectedAt": "2026-06-02",
		"notes":       "queen looked strong",
		"type":        "ROUTINE",
		"assessment":  map[string]any{"version": 1, "queenStatus": "HEALTHY"},
	})
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("seed create: status = %d, want %d", createResp.StatusCode, http.StatusCreated)
	}
	var created inspectionhttp.Response
	decodeJSON(t, createResp, &created)

	resp := stack.request(t, http.MethodGet, "/api/v1/hives/"+hiveID.String()+"/health/history?from=2026-06-01&to=2026-06-03", token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var got inspectionhttp.ColonyHealthHistoryResponse
	decodeJSON(t, resp, &got)

	if got.AlgorithmVersion != "v1" {
		t.Fatalf("algorithmVersion = %q, want v1", got.AlgorithmVersion)
	}
	if got.From != "2026-06-01" || got.To != "2026-06-03" {
		t.Fatalf("from/to = %s/%s, want 2026-06-01/2026-06-03", got.From, got.To)
	}
	if got.Interval != "DAY" {
		t.Fatalf("interval = %q, want DAY", got.Interval)
	}
	if len(got.Points) != 3 {
		t.Fatalf("len(points) = %d, want 3", len(got.Points))
	}
	if got.Points[0].Date != "2026-06-01" || got.Points[2].Date != "2026-06-03" {
		t.Fatalf("points span = %s..%s, want 2026-06-01..2026-06-03", got.Points[0].Date, got.Points[2].Date)
	}
	// Before the inspection: no evidence yet.
	if got.Points[0].State != "UNKNOWN" {
		t.Fatalf("points[0].state = %q, want UNKNOWN (before the inspection)", got.Points[0].State)
	}
	// On and after the inspection date: queen dimension reflects it.
	for _, point := range got.Points[1:] {
		var queenState string
		for _, dimension := range point.Dimensions {
			if dimension.Dimension == "QUEEN" {
				queenState = string(dimension.State)
			}
		}
		if queenState != "GOOD" {
			t.Fatalf("points[%s] queen state = %q, want GOOD", point.Date, queenState)
		}
	}

	if len(got.Inspections) != 1 {
		t.Fatalf("len(inspections) = %d, want 1", len(got.Inspections))
	}
	if got.Inspections[0].ID != created.ID || got.Inspections[0].Date != "2026-06-02" || string(got.Inspections[0].Type) != "ROUTINE" {
		t.Fatalf("inspections[0] = %+v, want id=%s date=2026-06-02 type=ROUTINE", got.Inspections[0], created.ID)
	}
}

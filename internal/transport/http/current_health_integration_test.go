//go:build integration

package http_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"

	inspectionhttp "github.com/sbezhuk/beebase-inspection-service/internal/transport/http/inspection"
)

// TestInspectionFlow_HiveHealth_NotGatedByEntitlement proves the public
// Current Health endpoint remains available to a caller regardless of
// subscription entitlement.
func TestInspectionFlow_HiveHealth_NotGatedByEntitlement(t *testing.T) {
	stack := newTestStack(t)
	userID := uuid.New()
	hiveID := uuid.New()
	token := stack.tokenFor(t, userID)
	stack.hive.allow(token, hiveID)

	resp := stack.request(t, http.MethodGet, "/api/v1/hives/"+hiveID.String()+"/health", token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET .../health: status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var got inspectionhttp.ColonyHealthResponse
	decodeJSON(t, resp, &got)
	if got.State != "UNKNOWN" || got.Coverage != "NONE" {
		t.Fatalf("health = (%q, %q), want (UNKNOWN, NONE) for a hive with no inspections", got.State, got.Coverage)
	}
}

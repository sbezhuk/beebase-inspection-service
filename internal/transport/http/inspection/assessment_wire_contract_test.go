package inspection

import (
	"encoding/json"
	"testing"

	"github.com/sbezhuk/beebase-inspection-service/internal/domain/inspection"
)

// These tests exist because AssessmentRequest/AssessmentResponse fields were
// changed from *string/*[]string to the domain's own named string types
// (e.g. *inspection.FoodStores) as a type-safety refactor. That change must
// be invisible on the wire: JSON in and out is asserted byte-for-byte
// against the same strings the API has always used, so an already-released
// Flutter client (which only knows plain JSON strings) keeps working
// unmodified against this backend.

// TestAssessmentRequest_JSONToTypedField proves decoding an old-client-style
// JSON payload still populates the now-typed field with the exact same
// value it always did.
func TestAssessmentRequest_JSONToTypedField(t *testing.T) {
	var req AssessmentRequest
	if err := json.Unmarshal([]byte(`{"foodStores":"NOT_CHECKED"}`), &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if req.FoodStores == nil || *req.FoodStores != inspection.FoodStoresNotChecked {
		t.Fatalf("FoodStores = %v, want %q", req.FoodStores, inspection.FoodStoresNotChecked)
	}
	if string(*req.FoodStores) != "NOT_CHECKED" {
		t.Fatalf("underlying string = %q, want %q", string(*req.FoodStores), "NOT_CHECKED")
	}
}

// TestAssessmentRequest_ArrayJSONToTypedSlice proves an array field decodes
// element-for-element into the typed slice, preserving order and duplicates
// exactly as the raw []string version did (validation, not decoding, is
// responsible for rejecting duplicates).
func TestAssessmentRequest_ArrayJSONToTypedSlice(t *testing.T) {
	var req AssessmentRequest
	if err := json.Unmarshal([]byte(`{"pestSigns":["VARROA_MITES","OTHER"]}`), &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if req.PestSigns == nil {
		t.Fatal("PestSigns is nil")
	}
	want := []inspection.PestSign{inspection.PestSignVarroaMites, inspection.PestSignOther}
	got := *req.PestSigns
	if len(got) != len(want) {
		t.Fatalf("PestSigns = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("PestSigns[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// TestAssessmentRequest_OldClientPayloadRemainsValid replays a complete,
// historically-valid Flutter request body through decode -> domain() ->
// ValidateFor and confirms it is still accepted, proving the typed DTO
// doesn't reject anything the old raw-string DTO accepted.
func TestAssessmentRequest_OldClientPayloadRemainsValid(t *testing.T) {
	body := `{"version":1,"colonyStrength":"STRONG","foodStores":"NOT_CHECKED","healthConcerns":"NONE"}`
	var req AssessmentRequest
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if err := req.domain().ValidateFor(inspection.TypeRoutine); err != nil {
		t.Fatalf("ValidateFor() error = %v, want nil (old client payload must remain valid)", err)
	}
}

// TestAssessmentRequest_InvalidValueStillRejected proves the typed DTO does
// not accidentally widen validation: an out-of-enum string is still decoded
// (Go's json package doesn't validate named string types on unmarshal - the
// named type only documents the contract) but is still rejected at
// ValidateFor, exactly as it was when the field was a raw *string.
func TestAssessmentRequest_InvalidValueStillRejected(t *testing.T) {
	var req AssessmentRequest
	if err := json.Unmarshal([]byte(`{"foodStores":"SOMETHING_NEW"}`), &req); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if req.FoodStores == nil || string(*req.FoodStores) != "SOMETHING_NEW" {
		t.Fatalf("FoodStores = %v, want decoded-but-unvalidated %q", req.FoodStores, "SOMETHING_NEW")
	}
	if err := req.domain().ValidateFor(inspection.TypeRoutine); err == nil {
		t.Fatal("ValidateFor() error = nil, want rejection of an out-of-enum value")
	}
}

// TestAssessmentResponse_TypedFieldToJSON proves a typed enum field
// marshals to the exact same JSON string the API has always sent - the
// named string type has no custom MarshalJSON, so it serializes as its
// underlying string, identical to the old *string field.
func TestAssessmentResponse_TypedFieldToJSON(t *testing.T) {
	notChecked := inspection.FoodStoresNotChecked
	resp := AssessmentResponse{Version: 1, FoodStores: &notChecked}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	const want = `{"version":1,"foodStores":"NOT_CHECKED"}`
	if string(data) != want {
		t.Fatalf("JSON = %s, want %s", data, want)
	}
}

// TestAssessmentResponse_ArrayFieldToJSON is the array-field equivalent of
// TestAssessmentResponse_TypedFieldToJSON: order and every element's exact
// wire string are preserved.
func TestAssessmentResponse_ArrayFieldToJSON(t *testing.T) {
	stages := []inspection.BroodStage{inspection.BroodStageEggs, inspection.BroodStageLarvae, inspection.BroodStageCapped}
	resp := AssessmentResponse{Version: 1, BroodStages: &stages}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	const want = `{"version":1,"broodStages":["EGGS","LARVAE","CAPPED"]}`
	if string(data) != want {
		t.Fatalf("JSON = %s, want %s", data, want)
	}
}

// TestAssessmentResponse_OmitsAbsentFields proves the omitempty contract on
// nil enum pointers/slices is unaffected by the type change - a client that
// distinguishes "field absent" from "field null" still sees the same
// absence it always did.
func TestAssessmentResponse_OmitsAbsentFields(t *testing.T) {
	resp := AssessmentResponse{Version: 1}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	const want = `{"version":1}`
	if string(data) != want {
		t.Fatalf("JSON = %s, want %s (every unset field must be omitted, not null)", data, want)
	}
}

// TestAssessmentResponse_FullRoundTrip builds a representative, fully
// populated assessment through the same path a real request/response cycle
// uses (JSON -> AssessmentRequest -> domain Assessment -> AssessmentResponse
// -> JSON) and asserts the emitted JSON is byte-identical to the original
// wire strings, proving every field-level typed conversion composes
// correctly end to end.
func TestAssessmentResponse_FullRoundTrip(t *testing.T) {
	const inbound = `{"version":1,"healthOverallCondition":"GOOD","pestSigns":["VARROA_MITES"],"healthWarningSigns":["ABNORMAL_BROOD"],"healthConcernLevel":"LOW"}`
	var req AssessmentRequest
	if err := json.Unmarshal([]byte(inbound), &req); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	domainAssessment := req.domain()
	if err := domainAssessment.ValidateFor(inspection.TypeHealth); err != nil {
		t.Fatalf("ValidateFor() error = %v", err)
	}

	resp := AssessmentResponse{
		Version:                domainAssessment.Version,
		HealthOverallCondition: domainAssessment.HealthOverallCondition,
		PestSigns:              domainAssessment.PestSigns,
		HealthWarningSigns:     domainAssessment.HealthWarningSigns,
		HealthConcernLevel:     domainAssessment.HealthConcernLevel,
	}
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	if string(data) != inbound {
		t.Fatalf("round-tripped JSON = %s, want %s", data, inbound)
	}
}

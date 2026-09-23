package inspection

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/sbezhuk/beebase-common/httpx"
	"github.com/sbezhuk/beebase-inspection-service/internal/domain/inspection"
)

const (
	CodeHealthFactsToRequired = "to_required"
	CodeHealthFactsToInvalid  = "to_invalid"
)

type HealthFactsResponse struct {
	HiveID      uuid.UUID                       `json:"hiveId"`
	Inspections []HealthFactsInspectionResponse `json:"inspections"`
}

type HealthFactsInspectionResponse struct {
	ID          uuid.UUID           `json:"id"`
	HiveID      uuid.UUID           `json:"hiveId"`
	InspectedAt string              `json:"inspectedAt"`
	Type        inspection.Type     `json:"type"`
	Assessment  *AssessmentResponse `json:"assessment,omitempty"`
}

// InternalHealthFacts handles GET /internal/api/v1/hives/{hiveId}/health-facts.
// Authentication is applied by the root router's internal-auth middleware.
// It returns raw inspection facts only; it does not calculate health.
func (h *Handler) InternalHealthFacts(w http.ResponseWriter, r *http.Request) {
	hiveID, err := uuid.Parse(chi.URLParam(r, "hiveId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, CodeInvalidHiveID, "hive id must be a valid UUID")
		return
	}

	rawTo := r.URL.Query().Get("to")
	if rawTo == "" {
		httpx.WriteError(w, http.StatusBadRequest, CodeHealthFactsToRequired, "to is required")
		return
	}
	to, err := time.Parse(dateFilterLayout, rawTo)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, CodeHealthFactsToInvalid, "to must be a valid calendar date")
		return
	}

	facts, err := h.service.GetInternalHealthFacts(r.Context(), hiveID, to)
	if err != nil {
		httpx.WriteInternalError(w, h.log, err)
		return
	}

	items := make([]HealthFactsInspectionResponse, len(facts))
	for i, current := range facts {
		items[i] = newHealthFactsInspectionResponse(current)
	}
	httpx.WriteJSON(w, http.StatusOK, HealthFactsResponse{HiveID: hiveID, Inspections: items})
}

func newHealthFactsInspectionResponse(current *inspection.Inspection) HealthFactsInspectionResponse {
	var assessment *AssessmentResponse
	if current.Assessment != nil {
		assessment = newAssessmentResponse(current.Assessment)
	}
	return HealthFactsInspectionResponse{
		ID: current.ID, HiveID: current.HiveID,
		InspectedAt: current.InspectedAt.Format(dateFilterLayout),
		Type:        current.Type, Assessment: assessment,
	}
}

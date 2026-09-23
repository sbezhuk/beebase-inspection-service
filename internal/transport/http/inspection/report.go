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
	CodeReportFromRequired = "from_required"
	CodeReportFromInvalid  = "from_invalid"
	CodeReportToRequired   = "to_required"
	CodeReportToInvalid    = "to_invalid"
	CodeReportFromAfterTo  = "from_after_to"
	CodeReportRangeTooLong = "report_range_too_long"
	maxReportMonths        = 12
)

type ReportInspectionResponse struct {
	ID          uuid.UUID           `json:"id"`
	HiveID      uuid.UUID           `json:"hiveId"`
	InspectedAt string              `json:"inspectedAt"`
	Notes       string              `json:"notes"`
	Type        inspection.Type     `json:"type"`
	TypeLabel   string              `json:"typeLabel"`
	CreatedAt   time.Time           `json:"createdAt"`
	UpdatedAt   time.Time           `json:"updatedAt"`
	Assessment  *AssessmentResponse `json:"assessment,omitempty"`
}

type ReportDataResponse struct {
	From          string                      `json:"from"`
	To            string                      `json:"to"`
	Inspections   []ReportInspectionResponse  `json:"inspections"`
	ColonyHealth  ColonyHealthResponse        `json:"colonyHealth"`
	HealthHistory ColonyHealthHistoryResponse `json:"healthHistory"`
}

// InternalReportData handles GET /internal/api/v1/hives/{hiveId}/report-data.
// Authentication is applied by the root router's internal-auth middleware.
func (h *Handler) InternalReportData(w http.ResponseWriter, r *http.Request) {
	hiveID, err := uuid.Parse(chi.URLParam(r, "hiveId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, CodeInvalidHiveID, "hive id must be a valid UUID")
		return
	}
	from, to, fields := parseReportRange(r)
	if len(fields) > 0 {
		httpx.WriteValidationError(w, fields)
		return
	}

	history, healthResult, err := h.service.GetInternalReportData(r.Context(), hiveID, from, to)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	items := make([]ReportInspectionResponse, len(history.Inspections))
	for i, current := range history.Inspections {
		items[i] = newReportInspectionResponse(current)
	}
	httpx.WriteJSON(w, http.StatusOK, ReportDataResponse{
		From:          from.Format(dateFilterLayout),
		To:            to.Format(dateFilterLayout),
		Inspections:   items,
		ColonyHealth:  newColonyHealthResponse(to, healthResult),
		HealthHistory: newColonyHealthHistoryResponse(history),
	})
}

func newReportInspectionResponse(current *inspection.Inspection) ReportInspectionResponse {
	var assessment *AssessmentResponse
	if current.Assessment != nil {
		assessment = newAssessmentResponse(current.Assessment)
	}
	return ReportInspectionResponse{
		ID: current.ID, HiveID: current.HiveID,
		InspectedAt: current.InspectedAt.Format(dateFilterLayout),
		Notes:       current.Notes, Type: current.Type, TypeLabel: current.Type.Label(),
		CreatedAt: current.CreatedAt, UpdatedAt: current.UpdatedAt, Assessment: assessment,
	}
}

func parseReportRange(r *http.Request) (from, to time.Time, fields map[string]string) {
	fields = map[string]string{}
	rawFrom, rawTo := r.URL.Query().Get("from"), r.URL.Query().Get("to")
	if rawFrom == "" {
		fields["from"] = CodeReportFromRequired
	} else if parsed, err := time.Parse(dateFilterLayout, rawFrom); err != nil {
		fields["from"] = CodeReportFromInvalid
	} else {
		from = parsed
	}
	if rawTo == "" {
		fields["to"] = CodeReportToRequired
	} else if parsed, err := time.Parse(dateFilterLayout, rawTo); err != nil {
		fields["to"] = CodeReportToInvalid
	} else {
		to = parsed
	}
	if len(fields) > 0 {
		return time.Time{}, time.Time{}, fields
	}
	if from.After(to) {
		fields["to"] = CodeReportFromAfterTo
		return time.Time{}, time.Time{}, fields
	}
	if to.After(from.AddDate(0, maxReportMonths, 0)) {
		fields["to"] = CodeReportRangeTooLong
		return time.Time{}, time.Time{}, fields
	}
	return from, to, nil
}

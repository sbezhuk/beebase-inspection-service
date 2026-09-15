// Package inspection holds the HTTP handlers for inspection management.
// Handlers stay thin: they decode/validate the request, pull the
// authenticated user's ID (and, for Create, their raw access token,
// forwarded to hive-service) from the request, call into the application
// service, and map the result (or error) to a response. No business
// logic or repository access happens here.
package inspection

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	httpmw "github.com/sbezhuk/beebase-common/authmw"
	"github.com/sbezhuk/beebase-common/httpx"
	"github.com/sbezhuk/beebase-common/pagination"
	appinspection "github.com/sbezhuk/beebase-inspection-service/internal/application/inspection"
	"github.com/sbezhuk/beebase-inspection-service/internal/domain/inspection"
)

// Error codes for inspection failures, returned as the top-level
// "error.code". Each is a stable key a client can map to a localized
// message. CodeHiveNotFound intentionally reuses hive-service's own code
// string, since it's the same meaning from the client's point of view
// regardless of which service returned it.
const (
	CodeInspectionNotFound  = "inspection_not_found"
	CodeInvalidInspectionID = "invalid_inspection_id"
	CodeInvalidHiveID       = "invalid_hive_id"
	CodeHiveNotFound        = "hive_not_found"
	CodeImageNotFound       = "image_not_found"
	CodeInvalidSearch       = "invalid_search"
	CodeInvalidSortOrder    = "invalid_sort_order"
	CodeMediaLimitReached   = "media_limit_reached"
	CodeInvalidDateFrom     = "invalid_date_from"
	CodeInvalidDateTo       = "invalid_date_to"
	CodeInvalidDateRange    = "invalid_date_range"
)

const minSearchLength = 3

// dateFilterLayout is the ISO 8601 calendar-date format the date_from/
// date_to query parameters must use - a date only, no time-of-day or
// offset (unlike inspected_at in the request body, which is a full RFC
// 3339 timestamp).
const dateFilterLayout = "2006-01-02"

// Handler exposes the inspection HTTP endpoints. Every method requires
// the request to have already passed through httpmw.RequireAuth.
type Handler struct {
	service       *appinspection.Service
	log           *slog.Logger
	publicBaseURL string
	reminders     interface {
		Cleanup(context.Context, string, uuid.UUID) error
	}
}

// NewHandler returns a Handler backed by service. publicBaseURL is the
// gateway's externally reachable base URL, used to build each image's
// image_url.
func NewHandler(service *appinspection.Service, log *slog.Logger, publicBaseURL string, reminders ...interface {
	Cleanup(context.Context, string, uuid.UUID) error
}) *Handler {
	h := &Handler{service: service, log: log, publicBaseURL: publicBaseURL}
	if len(reminders) > 0 {
		h.reminders = reminders[0]
	}
	return h
}

// Create handles POST /inspections.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	userID, token, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	var req CreateRequest
	if !decodeAndValidate(w, r, &req) {
		return
	}
	// Already validated as well-formed by CreateRequest.Validate.
	hiveID, _ := uuid.Parse(req.HiveID)
	inspectedAt, _ := time.Parse(time.RFC3339, req.InspectedAt)

	images := make([]uuid.UUID, len(req.Images))
	for i, s := range req.Images {
		images[i], _ = uuid.Parse(s) // already validated by req.Validate
	}

	created, err := h.service.Create(r.Context(), userID, token, appinspection.CreateInput{
		HiveID:      hiveID,
		InspectedAt: inspectedAt,
		Notes:       req.Notes,
		Type:        inspection.Type(req.Type),
		Images:      images,
	})
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, newResponse(created, h.publicBaseURL))
}

// Get handles GET /inspections/{inspectionID}.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	inspectionID, ok := h.pathInspectionID(w, r)
	if !ok {
		return
	}

	got, err := h.service.Get(r.Context(), userID, inspectionID)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, newResponse(got, h.publicBaseURL))
}

// List handles GET /inspections.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	p, fields := pagination.ParseParams(r)
	search, fields := parseSearch(r, fields)
	typ, fields := parseType(r, fields)
	dateFrom, dateTo, fields := parseDateFilter(r, fields)
	sortOrder, fields := parseSortOrder(r, fields)
	if len(fields) > 0 {
		httpx.WriteValidationError(w, fields)
		return
	}

	inspections, total, err := h.service.List(r.Context(), userID, p, search, typ, dateFrom, dateTo, sortOrder)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, pagination.NewResponse(newListResponse(inspections, h.publicBaseURL), p, total))
}

// ListByHive handles GET /hives/{hiveID}/inspections.
func (h *Handler) ListByHive(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	hiveID, err := uuid.Parse(chi.URLParam(r, "hiveID"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, CodeInvalidHiveID, "hive id must be a valid UUID")
		return
	}

	p, fields := pagination.ParseParams(r)
	search, fields := parseSearch(r, fields)
	typ, fields := parseType(r, fields)
	dateFrom, dateTo, fields := parseDateFilter(r, fields)
	sortOrder, fields := parseSortOrder(r, fields)
	if len(fields) > 0 {
		httpx.WriteValidationError(w, fields)
		return
	}

	inspections, total, err := h.service.ListByHive(r.Context(), userID, hiveID, p, search, typ, dateFrom, dateTo, sortOrder)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, pagination.NewResponse(newListResponse(inspections, h.publicBaseURL), p, total))
}

func parseSearch(r *http.Request, fields map[string]string) (*string, map[string]string) {
	s := r.URL.Query().Get("search")
	if s == "" {
		return nil, fields
	}
	if len(s) < minSearchLength {
		if fields == nil {
			fields = map[string]string{}
		}
		fields["search"] = CodeInvalidSearch
		return nil, fields
	}
	return &s, fields
}

// parseType reads the optional "type" query parameter, validating it
// against the same InspectionType enum CreateRequest/UpdateRequest.Validate
// use. An absent value means no filter.
func parseType(r *http.Request, fields map[string]string) (*inspection.Type, map[string]string) {
	raw := r.URL.Query().Get("type")
	if raw == "" {
		return nil, fields
	}
	t := inspection.Type(raw)
	if !t.Valid() {
		if fields == nil {
			fields = map[string]string{}
		}
		fields["type"] = CodeTypeInvalid
		return nil, fields
	}
	return &t, fields
}

// parseDateFilter reads the optional "date_from"/"date_to" query
// parameters: each is an ISO 8601 calendar date (YYYY-MM-DD), independently
// optional, restricting inspected_at. date_from is returned as that day's
// start (00:00:00 UTC), an inclusive lower bound. date_to is returned as
// the start of the following day (00:00:00 UTC), an exclusive upper bound
// - since the filter must match the requested day in full, "inspected_at
// < date_to" (the day after) does the same job as an inclusive same-day
// upper bound would, without depending on the column's time resolution.
// When both are given, date_from must not fall after date_to (as calendar
// dates, not as the adjusted bounds returned here); given alone, each
// applies independently, and neither requires the other.
func parseDateFilter(r *http.Request, fields map[string]string) (dateFrom, dateTo *time.Time, _ map[string]string) {
	rawFrom := r.URL.Query().Get("date_from")
	rawTo := r.URL.Query().Get("date_to")

	var fromDay, toDay *time.Time

	if rawFrom != "" {
		parsed, err := time.Parse(dateFilterLayout, rawFrom)
		if err != nil {
			if fields == nil {
				fields = map[string]string{}
			}
			fields["date_from"] = CodeInvalidDateFrom
		} else {
			fromDay = &parsed
			dateFrom = &parsed
		}
	}

	if rawTo != "" {
		parsed, err := time.Parse(dateFilterLayout, rawTo)
		if err != nil {
			if fields == nil {
				fields = map[string]string{}
			}
			fields["date_to"] = CodeInvalidDateTo
		} else {
			toDay = &parsed
			exclusive := parsed.AddDate(0, 0, 1)
			dateTo = &exclusive
		}
	}

	if fromDay != nil && toDay != nil && fromDay.After(*toDay) {
		if fields == nil {
			fields = map[string]string{}
		}
		fields["date_to"] = CodeInvalidDateRange
	}

	return dateFrom, dateTo, fields
}

// parseSortOrder reads the optional "sortOrder" query parameter, which
// requests the list be ordered by creation date instead of the endpoint's
// default order (which today is the inspection's own business date,
// InspectedAt). A missing value means "use the default order" (nil); an
// invalid value ("asc"/"desc" are the only accepted ones) is reported as a
// validation error the same way parseSearch reports one.
func parseSortOrder(r *http.Request, fields map[string]string) (*string, map[string]string) {
	s := r.URL.Query().Get("sortOrder")
	if s == "" {
		return nil, fields
	}
	if s != "asc" && s != "desc" {
		if fields == nil {
			fields = map[string]string{}
		}
		fields["sortOrder"] = CodeInvalidSortOrder
		return nil, fields
	}
	return &s, fields
}

// Update handles PUT /inspections/{inspectionID}.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	userID, token, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	inspectionID, ok := h.pathInspectionID(w, r)
	if !ok {
		return
	}

	var req UpdateRequest
	if !decodeAndValidate(w, r, &req) {
		return
	}
	inspectedAt, _ := time.Parse(time.RFC3339, req.InspectedAt)

	var images *[]uuid.UUID
	if req.Images != nil {
		parsed := make([]uuid.UUID, len(req.Images))
		for i, s := range req.Images {
			parsed[i], _ = uuid.Parse(s) // already validated by req.Validate
		}
		images = &parsed
	}

	updated, err := h.service.Update(r.Context(), userID, token, inspectionID, appinspection.UpdateInput{
		InspectedAt: inspectedAt,
		Notes:       req.Notes,
		Type:        inspection.Type(req.Type),
		Images:      images,
	})
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, newResponse(updated, h.publicBaseURL))
}

// Delete handles DELETE /inspections/{inspectionID}.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	inspectionID, ok := h.pathInspectionID(w, r)
	if !ok {
		return
	}

	if err := h.service.Delete(r.Context(), userID, inspectionID); err != nil {
		h.writeServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
	if h.reminders != nil {
		if err := h.reminders.Cleanup(r.Context(), "inspection", inspectionID); err != nil {
			h.log.Warn("reminder cleanup failed", "entity_type", "inspection", "entity_id", inspectionID, "error", err)
		}
	}
}

// DeleteByHive handles DELETE /hives/{hiveID}/inspections. It hard-deletes
// every inspection belonging to hiveID, used by hive-service to cascade a
// hive delete.
func (h *Handler) DeleteByHive(w http.ResponseWriter, r *http.Request) {
	userID, token, ok := h.requireAuth(w, r)
	if !ok {
		return
	}

	hiveID, err := uuid.Parse(chi.URLParam(r, "hiveID"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, CodeInvalidHiveID, "hive id must be a valid UUID")
		return
	}

	if _, err := h.service.DeleteByHive(r.Context(), userID, token, hiveID); err != nil {
		h.writeServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// requireAuth returns the authenticated user's ID (from context, set by
// httpmw.RequireAuth) and their raw access token (read back off the
// request's own Authorization header, which RequireAuth already
// validated) so it can be forwarded to hive-service.
// HiveInspectionStatus handles GET /api/v1/inspections/hive-status.
// Called by hive-service (to filter hive listings by "needs inspection")
// and statistics-service (to report the Dashboard's Needs Attention
// section), never directly by an end-user client.
func (h *Handler) HiveInspectionStatus(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}

	latestByHive, thresholdDays, err := h.service.HiveInspectionStatus(r.Context(), userID)
	if err != nil {
		httpx.WriteInternalError(w, h.log, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, newHiveInspectionStatusResponse(latestByHive, thresholdDays))
}

func (h *Handler) requireUserID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	userID, ok := httpmw.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpmw.CodeMissingAuthorization, "missing authentication")
		return uuid.Nil, false
	}
	return userID, true
}

func (h *Handler) requireAuth(w http.ResponseWriter, r *http.Request) (uuid.UUID, string, bool) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return uuid.Nil, "", false
	}

	const prefix = "Bearer "
	token := strings.TrimPrefix(r.Header.Get("Authorization"), prefix)

	return userID, token, true
}

func (h *Handler) pathInspectionID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "inspectionID"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, CodeInvalidInspectionID, "inspection id must be a valid UUID")
		return uuid.Nil, false
	}
	return id, true
}

func (h *Handler) writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, inspection.ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, CodeInspectionNotFound, "inspection not found")
	case errors.Is(err, appinspection.ErrHiveNotFound):
		httpx.WriteError(w, http.StatusNotFound, CodeHiveNotFound, "hive not found")
	case errors.Is(err, appinspection.ErrImageNotFound):
		httpx.WriteValidationError(w, map[string]string{"images": CodeImageNotFound})
	case errors.Is(err, appinspection.ErrMediaLimitReached):
		httpx.WriteError(w, http.StatusBadRequest, CodeMediaLimitReached, "maximum 5 photos allowed")
	default:
		httpx.WriteInternalError(w, h.log, err)
	}
}

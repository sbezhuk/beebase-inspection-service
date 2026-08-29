// Package inspection holds the HTTP handlers for inspection management.
// Handlers stay thin: they decode/validate the request, pull the
// authenticated user's ID (and, for Create, their raw access token,
// forwarded to hive-service) from the request, call into the application
// service, and map the result (or error) to a response. No business
// logic or repository access happens here.
package inspection

import (
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
)

// Handler exposes the inspection HTTP endpoints. Every method requires
// the request to have already passed through httpmw.RequireAuth.
type Handler struct {
	service *appinspection.Service
	log     *slog.Logger
}

// NewHandler returns a Handler backed by service.
func NewHandler(service *appinspection.Service, log *slog.Logger) *Handler {
	return &Handler{service: service, log: log}
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

	created, err := h.service.Create(r.Context(), userID, token, appinspection.CreateInput{
		HiveID:      hiveID,
		InspectedAt: inspectedAt,
		Notes:       req.Notes,
	})
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, newResponse(created))
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

	httpx.WriteJSON(w, http.StatusOK, newResponse(got))
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
	if len(fields) > 0 {
		httpx.WriteValidationError(w, fields)
		return
	}

	inspections, total, err := h.service.ListByHive(r.Context(), userID, hiveID, p)
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, pagination.NewResponse(newListResponse(inspections), p, total))
}

// Update handles PUT /inspections/{inspectionID}.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := h.requireAuth(w, r)
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

	updated, err := h.service.Update(r.Context(), userID, inspectionID, appinspection.UpdateInput{
		InspectedAt: inspectedAt,
		Notes:       req.Notes,
	})
	if err != nil {
		h.writeServiceError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, newResponse(updated))
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
}

// requireAuth returns the authenticated user's ID (from context, set by
// httpmw.RequireAuth) and their raw access token (read back off the
// request's own Authorization header, which RequireAuth already
// validated) so it can be forwarded to hive-service.
func (h *Handler) requireAuth(w http.ResponseWriter, r *http.Request) (uuid.UUID, string, bool) {
	userID, ok := httpmw.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, httpmw.CodeMissingAuthorization, "missing authentication")
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
	default:
		httpx.WriteInternalError(w, h.log, err)
	}
}

package inspection

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sbezhuk/beebase-common/httpx"
	"github.com/sbezhuk/beebase-inspection-service/internal/domain/inspection"
)

const maxNotesLength = 2000

// Field validation error codes. Each is a stable key a client can map to a
// localized message; the field carrying no error is simply absent from the
// response's "fields" map.
const (
	CodeHiveIDRequired      = "hive_id_required"
	CodeHiveIDInvalid       = "hive_id_invalid"
	CodeInspectedAtRequired = "inspected_at_required"
	CodeInspectedAtInvalid  = "inspected_at_invalid"
	CodeNotesRequired       = "notes_required"
	CodeNotesTooLong        = "notes_too_long"
	CodeTypeRequired        = "type_required"
	CodeTypeInvalid         = "type_invalid"
	CodeImagesInvalid       = "images_invalid"
)

// validatable is implemented by every request DTO in this package.
// Validate returns a map of field name to error code, empty if valid.
type validatable interface {
	Validate() map[string]string
}

// decodeAndValidate decodes the request body into dst and validates it,
// writing an appropriate error response and returning false if either step
// fails.
func decodeAndValidate(w http.ResponseWriter, r *http.Request, dst validatable) bool {
	defer func() { _ = r.Body.Close() }()

	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeInvalidBody, "request body must be valid JSON")
		return false
	}

	if fields := dst.Validate(); len(fields) > 0 {
		httpx.WriteValidationError(w, fields)
		return false
	}

	return true
}

// CreateRequest is the body of POST /inspections.
type CreateRequest struct {
	HiveID      string `json:"hive_id"`
	InspectedAt string `json:"inspected_at"` // RFC 3339
	Notes       string `json:"notes"`
	Type        string `json:"type"`
	// Images is the set of already-uploaded media ids to attach
	// immediately - unlike UpdateRequest.Images, there's no "leave alone"
	// case here since there's nothing to leave alone yet, so an absent/
	// empty images just means no photos.
	Images []string `json:"images"`
}

func (r *CreateRequest) Validate() map[string]string {
	fields := validateFields(r.InspectedAt, r.Notes, r.Type)

	switch {
	case strings.TrimSpace(r.HiveID) == "":
		fields["hive_id"] = CodeHiveIDRequired
	default:
		if _, err := uuid.Parse(r.HiveID); err != nil {
			fields["hive_id"] = CodeHiveIDInvalid
		}
	}

	validateImages(r.Images, fields)

	return fields
}

// UpdateRequest is the body of PUT /inspections/{inspectionID}. Update
// replaces every editable field (PUT semantics), not a partial patch.
// There's no hive_id here: an inspection can't be moved to a different
// hive.
type UpdateRequest struct {
	InspectedAt string `json:"inspected_at"`
	Notes       string `json:"notes"`
	Type        string `json:"type"`
	// Images, when present (even as an empty array), is the desired
	// final set of already-uploaded media IDs attached to this
	// inspection; omitting the field (or sending JSON null) leaves
	// currently attached media untouched. Go's json package already
	// distinguishes "absent/null" (nil slice) from "[]" (non-nil, empty
	// slice), which is exactly the distinction this needs.
	Images []string `json:"images"`
}

func (r *UpdateRequest) Validate() map[string]string {
	fields := validateFields(r.InspectedAt, r.Notes, r.Type)
	validateImages(r.Images, fields)
	return fields
}

// validateImages checks that every id in images is a well-formed UUID,
// setting fields["images"] on the first failure found.
func validateImages(images []string, fields map[string]string) {
	for _, id := range images {
		if _, err := uuid.Parse(id); err != nil {
			fields["images"] = CodeImagesInvalid
			return
		}
	}
}

func validateFields(inspectedAt, notes, typ string) map[string]string {
	fields := map[string]string{}

	switch {
	case strings.TrimSpace(inspectedAt) == "":
		fields["inspected_at"] = CodeInspectedAtRequired
	default:
		if _, err := time.Parse(time.RFC3339, inspectedAt); err != nil {
			fields["inspected_at"] = CodeInspectedAtInvalid
		}
	}

	switch {
	case strings.TrimSpace(notes) == "":
		fields["notes"] = CodeNotesRequired
	case len(notes) > maxNotesLength:
		fields["notes"] = CodeNotesTooLong
	}

	switch {
	case strings.TrimSpace(typ) == "":
		fields["type"] = CodeTypeRequired
	case !inspection.Type(typ).Valid():
		fields["type"] = CodeTypeInvalid
	}

	return fields
}

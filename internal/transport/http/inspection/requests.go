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
	CodeAssessmentInvalid   = "assessment_invalid"
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
	HiveID      string `json:"hiveId"`
	InspectedAt string `json:"inspectedAt"` // ISO 8601 calendar date (YYYY-MM-DD)
	Notes       string `json:"notes"`
	Type        string `json:"type"`
	// Images is the set of already-uploaded media ids to attach
	// immediately - unlike UpdateRequest.Images, there's no "leave alone"
	// case here since there's nothing to leave alone yet, so an absent/
	// empty images just means no photos.
	Images     []string           `json:"images"`
	Assessment *AssessmentRequest `json:"assessment"`
}

func (r *CreateRequest) Validate() map[string]string {
	fields := validateFields(r.InspectedAt, r.Notes, r.Type)

	switch {
	case strings.TrimSpace(r.HiveID) == "":
		fields["hiveId"] = CodeHiveIDRequired
	default:
		if _, err := uuid.Parse(r.HiveID); err != nil {
			fields["hiveId"] = CodeHiveIDInvalid
		}
	}

	validateImages(r.Images, fields)
	validateAssessment(r.Assessment, r.Type, fields)

	return fields
}

// UpdateRequest is the body of PUT /inspections/{inspectionID}. Update
// replaces every editable field (PUT semantics), not a partial patch.
// There's no hive_id here: an inspection can't be moved to a different
// hive.
type UpdateRequest struct {
	InspectedAt string `json:"inspectedAt"`
	Notes       string `json:"notes"`
	Type        string `json:"type"`
	// Images, when present (even as an empty array), is the desired
	// final set of already-uploaded media IDs attached to this
	// inspection; omitting the field (or sending JSON null) leaves
	// currently attached media untouched. Go's json package already
	// distinguishes "absent/null" (nil slice) from "[]" (non-nil, empty
	// slice), which is exactly the distinction this needs.
	Images     []string           `json:"images"`
	Assessment *AssessmentRequest `json:"assessment"`
}

func (r *UpdateRequest) Validate() map[string]string {
	fields := validateFields(r.InspectedAt, r.Notes, r.Type)
	validateImages(r.Images, fields)
	validateAssessment(r.Assessment, r.Type, fields)
	return fields
}

// AssessmentRequest is the wire shape of one assessment payload. Fields use
// the same named string types as the domain Assessment (not raw string) so
// the JSON tag is the only place the wire contract is defined - the type
// itself documents the closed set of accepted values, while
// encoding/json still decodes any JSON string into it unvalidated (that
// happens in Assessment.ValidateFor, via domain()). Wire representation is
// unchanged: these are still plain JSON strings/arrays of strings.
type AssessmentRequest struct {
	Version                *int                               `json:"version"`
	ColonyStrength         *inspection.ColonyStrength         `json:"colonyStrength"`
	QueenStatus            *inspection.QueenStatus            `json:"queenStatus"`
	BroodStatus            *inspection.BroodStatus            `json:"broodStatus"`
	FoodStores             *inspection.FoodStores             `json:"foodStores"`
	HealthConcerns         *inspection.HealthConcerns         `json:"healthConcerns"`
	QueenObserved          *inspection.QueenObserved          `json:"queenObserved"`
	EggsObserved           *inspection.EggsObserved           `json:"eggsObserved"`
	QueenCells             *inspection.QueenCells             `json:"queenCells"`
	QueenCondition         *inspection.QueenCondition         `json:"queenCondition"`
	BroodAmount            *inspection.BroodAmount            `json:"broodAmount"`
	BroodPattern           *inspection.BroodPattern           `json:"broodPattern"`
	BroodStages            *[]inspection.BroodStage           `json:"broodStages"`
	BroodConcerns          *inspection.BroodConcerns          `json:"broodConcerns"`
	HealthOverallCondition *inspection.HealthOverallCondition `json:"healthOverallCondition"`
	PestSigns              *[]inspection.PestSign             `json:"pestSigns"`
	HealthWarningSigns     *[]inspection.HealthWarningSign    `json:"healthWarningSigns"`
	HealthConcernLevel     *inspection.HealthConcernLevel     `json:"healthConcernLevel"`
	FeedingNeed            *inspection.FeedingNeed            `json:"feedingNeed"`
	FeedingPerformed       *inspection.FeedingPerformed       `json:"feedingPerformed"`
	FeedTypes              *[]inspection.FeedType             `json:"feedTypes"`
	Season                 *inspection.SeasonalPhase          `json:"season"`
	SeasonalStoreReadiness *inspection.SeasonalStoreReadiness `json:"seasonalStoreReadiness"`
	SeasonalReadiness      *inspection.SeasonalReadiness      `json:"seasonalReadiness"`
	SeasonalConcerns       *[]inspection.SeasonalConcern      `json:"seasonalConcerns"`
}

// domain copies r into a domain Assessment. Every field is already the
// domain's own named type, so this is a plain field-for-field copy - no
// string parsing or casting - and allowed values are still enforced by
// Assessment.ValidateFor, not here.
func (r *AssessmentRequest) domain() *inspection.Assessment {
	if r == nil {
		return nil
	}
	version := 0
	if r.Version != nil {
		version = *r.Version
	}
	return &inspection.Assessment{
		Version:                version,
		ColonyStrength:         r.ColonyStrength,
		QueenStatus:            r.QueenStatus,
		BroodStatus:            r.BroodStatus,
		FoodStores:             r.FoodStores,
		HealthConcerns:         r.HealthConcerns,
		QueenObserved:          r.QueenObserved,
		EggsObserved:           r.EggsObserved,
		QueenCells:             r.QueenCells,
		QueenCondition:         r.QueenCondition,
		BroodAmount:            r.BroodAmount,
		BroodPattern:           r.BroodPattern,
		BroodStages:            r.BroodStages,
		BroodConcerns:          r.BroodConcerns,
		HealthOverallCondition: r.HealthOverallCondition,
		PestSigns:              r.PestSigns,
		HealthWarningSigns:     r.HealthWarningSigns,
		HealthConcernLevel:     r.HealthConcernLevel,
		FeedingNeed:            r.FeedingNeed,
		FeedingPerformed:       r.FeedingPerformed,
		FeedTypes:              r.FeedTypes,
		Season:                 r.Season,
		SeasonalStoreReadiness: r.SeasonalStoreReadiness,
		SeasonalReadiness:      r.SeasonalReadiness,
		SeasonalConcerns:       r.SeasonalConcerns,
	}
}

func validateAssessment(r *AssessmentRequest, typ string, fields map[string]string) {
	if r != nil {
		if err := r.domain().ValidateFor(inspection.Type(typ)); err != nil {
			fields["assessment"] = CodeAssessmentInvalid
		}
	}
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
		fields["inspectedAt"] = CodeInspectedAtRequired
	default:
		if _, err := time.Parse("2006-01-02", inspectedAt); err != nil {
			fields["inspectedAt"] = CodeInspectedAtInvalid
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

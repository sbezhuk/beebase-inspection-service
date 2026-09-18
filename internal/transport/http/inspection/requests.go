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

type AssessmentRequest struct {
	Version                *int      `json:"version"`
	ColonyStrength         *string   `json:"colonyStrength"`
	QueenStatus            *string   `json:"queenStatus"`
	BroodStatus            *string   `json:"broodStatus"`
	FoodStores             *string   `json:"foodStores"`
	HealthConcerns         *string   `json:"healthConcerns"`
	QueenObserved          *string   `json:"queenObserved"`
	EggsObserved           *string   `json:"eggsObserved"`
	QueenCells             *string   `json:"queenCells"`
	QueenCondition         *string   `json:"queenCondition"`
	BroodAmount            *string   `json:"broodAmount"`
	BroodPattern           *string   `json:"broodPattern"`
	BroodStages            *[]string `json:"broodStages"`
	BroodConcerns          *string   `json:"broodConcerns"`
	HealthOverallCondition *string   `json:"healthOverallCondition"`
	PestSigns              *[]string `json:"pestSigns"`
	HealthWarningSigns     *[]string `json:"healthWarningSigns"`
	HealthConcernLevel     *string   `json:"healthConcernLevel"`
	FeedingNeed            *string   `json:"feedingNeed"`
	FeedingPerformed       *string   `json:"feedingPerformed"`
	FeedTypes              *[]string `json:"feedTypes"`
	Season                 *string   `json:"season"`
	SeasonalStoreReadiness *string   `json:"seasonalStoreReadiness"`
	SeasonalReadiness      *string   `json:"seasonalReadiness"`
	SeasonalConcerns       *[]string `json:"seasonalConcerns"`
}

func (r *AssessmentRequest) domain() *inspection.Assessment {
	if r == nil {
		return nil
	}
	a := &inspection.Assessment{}
	if r.Version != nil {
		a.Version = *r.Version
	}
	if r.ColonyStrength != nil {
		v := inspection.ColonyStrength(*r.ColonyStrength)
		a.ColonyStrength = &v
	}
	if r.QueenStatus != nil {
		v := inspection.QueenStatus(*r.QueenStatus)
		a.QueenStatus = &v
	}
	if r.BroodStatus != nil {
		v := inspection.BroodStatus(*r.BroodStatus)
		a.BroodStatus = &v
	}
	if r.FoodStores != nil {
		v := inspection.FoodStores(*r.FoodStores)
		a.FoodStores = &v
	}
	if r.HealthConcerns != nil {
		v := inspection.HealthConcerns(*r.HealthConcerns)
		a.HealthConcerns = &v
	}
	if r.QueenObserved != nil {
		v := inspection.QueenObserved(*r.QueenObserved)
		a.QueenObserved = &v
	}
	if r.EggsObserved != nil {
		v := inspection.EggsObserved(*r.EggsObserved)
		a.EggsObserved = &v
	}
	if r.QueenCells != nil {
		v := inspection.QueenCells(*r.QueenCells)
		a.QueenCells = &v
	}
	if r.QueenCondition != nil {
		v := inspection.QueenCondition(*r.QueenCondition)
		a.QueenCondition = &v
	}
	if r.BroodAmount != nil {
		v := inspection.BroodAmount(*r.BroodAmount)
		a.BroodAmount = &v
	}
	if r.BroodPattern != nil {
		v := inspection.BroodPattern(*r.BroodPattern)
		a.BroodPattern = &v
	}
	if r.BroodStages != nil {
		values := make([]inspection.BroodStage, len(*r.BroodStages))
		for index, stage := range *r.BroodStages {
			values[index] = inspection.BroodStage(stage)
		}
		a.BroodStages = &values
	}
	if r.BroodConcerns != nil {
		v := inspection.BroodConcerns(*r.BroodConcerns)
		a.BroodConcerns = &v
	}
	if r.HealthOverallCondition != nil {
		v := inspection.HealthOverallCondition(*r.HealthOverallCondition)
		a.HealthOverallCondition = &v
	}
	if r.PestSigns != nil {
		values := make([]inspection.PestSign, len(*r.PestSigns))
		for index, sign := range *r.PestSigns {
			values[index] = inspection.PestSign(sign)
		}
		a.PestSigns = &values
	}
	if r.HealthWarningSigns != nil {
		values := make([]inspection.HealthWarningSign, len(*r.HealthWarningSigns))
		for index, sign := range *r.HealthWarningSigns {
			values[index] = inspection.HealthWarningSign(sign)
		}
		a.HealthWarningSigns = &values
	}
	if r.HealthConcernLevel != nil {
		v := inspection.HealthConcernLevel(*r.HealthConcernLevel)
		a.HealthConcernLevel = &v
	}
	if r.FeedingNeed != nil {
		v := inspection.FeedingNeed(*r.FeedingNeed)
		a.FeedingNeed = &v
	}
	if r.FeedingPerformed != nil {
		v := inspection.FeedingPerformed(*r.FeedingPerformed)
		a.FeedingPerformed = &v
	}
	if r.FeedTypes != nil {
		values := make([]inspection.FeedType, len(*r.FeedTypes))
		for index, feedType := range *r.FeedTypes {
			values[index] = inspection.FeedType(feedType)
		}
		a.FeedTypes = &values
	}
	if r.Season != nil {
		v := inspection.SeasonalPhase(*r.Season)
		a.Season = &v
	}
	if r.SeasonalStoreReadiness != nil {
		v := inspection.SeasonalStoreReadiness(*r.SeasonalStoreReadiness)
		a.SeasonalStoreReadiness = &v
	}
	if r.SeasonalReadiness != nil {
		v := inspection.SeasonalReadiness(*r.SeasonalReadiness)
		a.SeasonalReadiness = &v
	}
	if r.SeasonalConcerns != nil {
		values := make([]inspection.SeasonalConcern, len(*r.SeasonalConcerns))
		for index, concern := range *r.SeasonalConcerns {
			values[index] = inspection.SeasonalConcern(concern)
		}
		a.SeasonalConcerns = &values
	}
	return a
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

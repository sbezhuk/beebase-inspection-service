package inspection

import (
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/sbezhuk/beebase-common/medialink"
	"github.com/sbezhuk/beebase-health/health"
	appinspection "github.com/sbezhuk/beebase-inspection-service/internal/application/inspection"
	"github.com/sbezhuk/beebase-inspection-service/internal/domain/inspection"
)

// ImageResponse is the public representation of one image attached to an
// inspection: its media id, plus the URL a client loads/caches it from.
// The URL is derived, not stored - it's always media-service's stable
// download route, built fresh on every response.
type ImageResponse struct {
	ID       uuid.UUID `json:"id"`
	ImageURL string    `json:"imageUrl"`
}

// Response is the public representation of an inspection. TypeLabel is
// derived from Type via inspection.Type.Label - the same single source of
// truth the type_invalid validation check reads from - so a client never
// has to maintain its own copy of the type-to-label mapping.
type Response struct {
	ID          uuid.UUID           `json:"id"`
	HiveID      uuid.UUID           `json:"hiveId"`
	InspectedAt string              `json:"inspectedAt"`
	Notes       string              `json:"notes"`
	Type        inspection.Type     `json:"type"`
	TypeLabel   string              `json:"typeLabel"`
	Images      []ImageResponse     `json:"images"`
	CreatedAt   time.Time           `json:"createdAt"`
	UpdatedAt   time.Time           `json:"updatedAt"`
	Assessment  *AssessmentResponse `json:"assessment,omitempty"`
}

type AssessmentResponse struct {
	Version                int                                `json:"version"`
	ColonyStrength         *inspection.ColonyStrength         `json:"colonyStrength,omitempty"`
	QueenStatus            *inspection.QueenStatus            `json:"queenStatus,omitempty"`
	BroodStatus            *inspection.BroodStatus            `json:"broodStatus,omitempty"`
	FoodStores             *inspection.FoodStores             `json:"foodStores,omitempty"`
	HealthConcerns         *inspection.HealthConcerns         `json:"healthConcerns,omitempty"`
	QueenObserved          *inspection.QueenObserved          `json:"queenObserved,omitempty"`
	EggsObserved           *inspection.EggsObserved           `json:"eggsObserved,omitempty"`
	QueenCells             *inspection.QueenCells             `json:"queenCells,omitempty"`
	QueenCondition         *inspection.QueenCondition         `json:"queenCondition,omitempty"`
	BroodAmount            *inspection.BroodAmount            `json:"broodAmount,omitempty"`
	BroodPattern           *inspection.BroodPattern           `json:"broodPattern,omitempty"`
	BroodStages            *[]inspection.BroodStage           `json:"broodStages,omitempty"`
	BroodConcerns          *inspection.BroodConcerns          `json:"broodConcerns,omitempty"`
	HealthOverallCondition *inspection.HealthOverallCondition `json:"healthOverallCondition,omitempty"`
	PestSigns              *[]inspection.PestSign             `json:"pestSigns,omitempty"`
	HealthWarningSigns     *[]inspection.HealthWarningSign    `json:"healthWarningSigns,omitempty"`
	HealthConcernLevel     *inspection.HealthConcernLevel     `json:"healthConcernLevel,omitempty"`
	FeedingNeed            *inspection.FeedingNeed            `json:"feedingNeed,omitempty"`
	FeedingPerformed       *inspection.FeedingPerformed       `json:"feedingPerformed,omitempty"`
	FeedTypes              *[]inspection.FeedType             `json:"feedTypes,omitempty"`
	Season                 *inspection.SeasonalPhase          `json:"season,omitempty"`
	SeasonalStoreReadiness *inspection.SeasonalStoreReadiness `json:"seasonalStoreReadiness,omitempty"`
	SeasonalReadiness      *inspection.SeasonalReadiness      `json:"seasonalReadiness,omitempty"`
	SeasonalConcerns       *[]inspection.SeasonalConcern      `json:"seasonalConcerns,omitempty"`
}

// ColonyHealthEvidenceSourceResponse is a compact provenance pointer to one
// piece of evidence that actually participated in a dimension's state -
// taken directly from health.DimensionEvaluation.ContributingEvidence,
// which the evaluator itself already computes (see evaluateDimension's own
// recency/latest-wins/supersession logic). It deliberately carries nothing
// beyond what identifies and dates the source: enough for a client to
// render "based on inspection from <date>" and open that inspection - never
// enough to re-derive health state, which stays the evaluator's job alone.
type ColonyHealthEvidenceSourceResponse struct {
	InspectionID   uuid.UUID       `json:"inspectionId"`
	InspectionType inspection.Type `json:"inspectionType"`
	InspectedAt    string          `json:"inspectedAt"`
	Field          string          `json:"field"`
}

type ColonyHealthDimensionResponse struct {
	Dimension health.HealthDimension  `json:"dimension"`
	State     health.DimensionState   `json:"state"`
	Coverage  health.EvidenceCoverage `json:"coverage"`
	// Sources is exactly health.DimensionEvaluation.ContributingEvidence,
	// mapped to its provenance fields only - never re-filtered or
	// re-derived here, so it can never diverge from what the evaluator
	// actually used for this exact state. Empty (never null) when nothing
	// contributed, e.g. an UNKNOWN dimension with no evidence at all.
	Sources []ColonyHealthEvidenceSourceResponse `json:"sources"`
}

// ColonyHealthResponse is a derived snapshot. The top-level state is the
// calculated Colony Health result; the OVERALL dimension remains the
// beekeeper's separate broad assessment.
type ColonyHealthResponse struct {
	AsOf       time.Time                       `json:"asOf"`
	State      health.DimensionState           `json:"state"`
	Coverage   health.EvidenceCoverage         `json:"coverage"`
	Dimensions []ColonyHealthDimensionResponse `json:"dimensions"`
}

func newColonyHealthResponse(asOf time.Time, evaluation health.ColonyHealthEvaluation) ColonyHealthResponse {
	return ColonyHealthResponse{
		AsOf:       asOf,
		State:      evaluation.State,
		Coverage:   evaluation.Coverage,
		Dimensions: newColonyHealthDimensionResponses(evaluation.Dimensions),
	}
}

func newColonyHealthDimensionResponses(dimensions []health.DimensionEvaluation) []ColonyHealthDimensionResponse {
	out := make([]ColonyHealthDimensionResponse, len(dimensions))
	for index, dimension := range dimensions {
		out[index] = ColonyHealthDimensionResponse{
			Dimension: dimension.Dimension,
			State:     dimension.State,
			Coverage:  dimension.Coverage,
			Sources:   newColonyHealthEvidenceSourceResponses(dimension.ContributingEvidence),
		}
	}
	return out
}

// newColonyHealthEvidenceSourceResponses maps evidence the evaluator itself
// already selected as contributing (see ColonyHealthDimensionResponse.
// Sources) to its provenance. InspectionID/InspectionType are only ever nil
// for evidence NormalizeInspection didn't produce from a real inspection -
// doesn't happen in practice (see health.sourceFor), but skipped rather
// than risking a nil-pointer dereference if that ever changed.
func newColonyHealthEvidenceSourceResponses(evidence []health.HealthEvidence) []ColonyHealthEvidenceSourceResponse {
	out := make([]ColonyHealthEvidenceSourceResponse, 0, len(evidence))
	for _, item := range evidence {
		if item.Source.InspectionID == nil || item.Source.InspectionType == nil {
			continue
		}
		out = append(out, ColonyHealthEvidenceSourceResponse{
			InspectionID:   *item.Source.InspectionID,
			InspectionType: inspection.Type(*item.Source.InspectionType),
			InspectedAt:    item.Source.OccurredAt.Format("2006-01-02"),
			Field:          string(item.Source.SourceField),
		})
	}
	return out
}

// IntervalDay is the interval represented by the internal report health
// history payload. Colony Health v1 calculates one point per calendar day.
const IntervalDay = "day"

// ColonyHealthHistoryPointResponse is one calendar day's Colony Health v1
// snapshot. It deliberately mirrors ColonyHealthResponse's own
// state/coverage/dimensions shape (state renamed from AsOf's sibling
// "state" - same field, same meaning) rather than inventing a parallel
// shape, so a client already rendering the live snapshot recognizes this
// immediately. OVERALL is not omitted: it comes through Dimensions as
// explanatory evidence, exactly as GetHiveHealth's response has always
// return it, and is still excluded from the aggregate State.
type ColonyHealthHistoryPointResponse struct {
	Date       string                          `json:"date"`
	State      health.DimensionState           `json:"state"`
	Coverage   health.EvidenceCoverage         `json:"coverage"`
	Dimensions []ColonyHealthDimensionResponse `json:"dimensions"`
}

// ColonyHealthHistoryInspectionResponse is a compact marker for one
// inspection that occurred within the requested history range - enough
// for a client to open it (GET /api/v1/inspections/{id}), not a full
// inspection payload.
type ColonyHealthHistoryInspectionResponse struct {
	ID   uuid.UUID       `json:"id"`
	Date string          `json:"date"`
	Type inspection.Type `json:"type"`
}

// ColonyHealthHistoryResponse is the history representation embedded in
// trusted internal report data: one point per calendar day in [From, To],
// plus every inspection that occurred in that same range.
type ColonyHealthHistoryResponse struct {
	AlgorithmVersion string                                  `json:"algorithmVersion"`
	From             string                                  `json:"from"`
	To               string                                  `json:"to"`
	Interval         string                                  `json:"interval"`
	Points           []ColonyHealthHistoryPointResponse      `json:"points"`
	Inspections      []ColonyHealthHistoryInspectionResponse `json:"inspections"`
}

func newColonyHealthHistoryResponse(result appinspection.HealthHistoryResult) ColonyHealthHistoryResponse {
	points := make([]ColonyHealthHistoryPointResponse, len(result.Points))
	for index, point := range result.Points {
		points[index] = ColonyHealthHistoryPointResponse{
			Date:       point.Date.Format("2006-01-02"),
			State:      point.Evaluation.State,
			Coverage:   point.Evaluation.Coverage,
			Dimensions: newColonyHealthDimensionResponses(point.Evaluation.Dimensions),
		}
	}

	inspections := make([]ColonyHealthHistoryInspectionResponse, len(result.Inspections))
	for index, i := range result.Inspections {
		inspections[index] = ColonyHealthHistoryInspectionResponse{
			ID:   i.ID,
			Date: i.InspectedAt.Format("2006-01-02"),
			Type: i.Type,
		}
	}

	return ColonyHealthHistoryResponse{
		AlgorithmVersion: health.DefaultRecencyPolicyV1().Version,
		From:             result.From.Format("2006-01-02"),
		To:               result.To.Format("2006-01-02"),
		Interval:         strings.ToUpper(IntervalDay),
		Points:           points,
		Inspections:      inspections,
	}
}

// newResponse builds a Response for i. Images is read straight from i -
// never nil (Inspection.Images is always a real, possibly-empty slice) -
// so it renders as "images": [] rather than null when there are no
// photos.
func newResponse(i *inspection.Inspection, publicBaseURL string) Response {
	images := make([]ImageResponse, len(i.Images))
	for idx, id := range i.Images {
		images[idx] = ImageResponse{ID: id, ImageURL: medialink.DownloadURL(publicBaseURL, id)}
	}
	assessment := newAssessmentResponse(i.Assessment)
	return Response{
		ID:          i.ID,
		HiveID:      i.HiveID,
		InspectedAt: i.InspectedAt.Format("2006-01-02"),
		Notes:       i.Notes,
		Type:        i.Type,
		TypeLabel:   i.Type.Label(),
		Images:      images,
		CreatedAt:   i.CreatedAt,
		UpdatedAt:   i.UpdatedAt,
		Assessment:  assessment,
	}
}

func newAssessmentResponse(a *inspection.Assessment) *AssessmentResponse {
	if a == nil {
		return nil
	}
	return &AssessmentResponse{Version: a.Version, ColonyStrength: a.ColonyStrength, QueenStatus: a.QueenStatus, BroodStatus: a.BroodStatus, FoodStores: a.FoodStores, HealthConcerns: a.HealthConcerns, QueenObserved: a.QueenObserved, EggsObserved: a.EggsObserved, QueenCells: a.QueenCells, QueenCondition: a.QueenCondition, BroodAmount: a.BroodAmount, BroodPattern: a.BroodPattern, BroodStages: a.BroodStages, BroodConcerns: a.BroodConcerns, HealthOverallCondition: a.HealthOverallCondition, PestSigns: a.PestSigns, HealthWarningSigns: a.HealthWarningSigns, HealthConcernLevel: a.HealthConcernLevel, FeedingNeed: a.FeedingNeed, FeedingPerformed: a.FeedingPerformed, FeedTypes: a.FeedTypes, Season: a.Season, SeasonalStoreReadiness: a.SeasonalStoreReadiness, SeasonalReadiness: a.SeasonalReadiness, SeasonalConcerns: a.SeasonalConcerns}
}

func newListResponse(inspections []*inspection.Inspection, publicBaseURL string) []Response {
	out := make([]Response, len(inspections))
	for idx, i := range inspections {
		out[idx] = newResponse(i, publicBaseURL)
	}
	return out
}

// HiveInspectionStatusItem is one hive's latest inspection date, as
// reported in HiveInspectionStatusResponse.Hives.
type HiveInspectionStatusItem struct {
	HiveID            uuid.UUID `json:"hiveId"`
	LatestInspectedAt time.Time `json:"latestInspectedAt"`
}

// HiveInspectionStatusResponse is the public representation of GET
// /api/v1/inspections/hive-status: the caller's currently configured
// inspection warning threshold, plus the latest inspection date for
// every hive they've ever inspected. A hive that's never been inspected
// is simply absent from Hives - not a zero-value entry - so a caller
// applying the "needs inspection" rule (see
// beebase-common/inspectionwarning) treats any hive id missing here as
// never inspected.
type HiveInspectionStatusResponse struct {
	ThresholdDays int                        `json:"thresholdDays"`
	Hives         []HiveInspectionStatusItem `json:"hives"`
}

// newHiveInspectionStatusResponse builds a HiveInspectionStatusResponse.
// Hives is never nil, so it renders as "[]" rather than "null" when the
// caller has no inspections at all.
func newHiveInspectionStatusResponse(latestByHive map[uuid.UUID]time.Time, thresholdDays int) HiveInspectionStatusResponse {
	hives := make([]HiveInspectionStatusItem, 0, len(latestByHive))
	for hiveID, latest := range latestByHive {
		hives = append(hives, HiveInspectionStatusItem{HiveID: hiveID, LatestInspectedAt: latest})
	}
	sort.Slice(hives, func(i, j int) bool { return hives[i].HiveID.String() < hives[j].HiveID.String() })

	return HiveInspectionStatusResponse{ThresholdDays: thresholdDays, Hives: hives}
}

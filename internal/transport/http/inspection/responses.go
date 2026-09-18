package inspection

import (
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/sbezhuk/beebase-common/medialink"
	"github.com/sbezhuk/beebase-inspection-service/internal/domain/health"
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

type ColonyHealthDimensionResponse struct {
	Dimension health.HealthDimension  `json:"dimension"`
	State     health.DimensionState   `json:"state"`
	Coverage  health.EvidenceCoverage `json:"coverage"`
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
	dimensions := make([]ColonyHealthDimensionResponse, len(evaluation.Dimensions))
	for index, dimension := range evaluation.Dimensions {
		dimensions[index] = ColonyHealthDimensionResponse{
			Dimension: dimension.Dimension,
			State:     dimension.State,
			Coverage:  dimension.Coverage,
		}
	}
	return ColonyHealthResponse{
		AsOf:       asOf,
		State:      evaluation.State,
		Coverage:   evaluation.Coverage,
		Dimensions: dimensions,
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
	var assessment *AssessmentResponse
	if i.Assessment != nil {
		assessment = &AssessmentResponse{Version: i.Assessment.Version, ColonyStrength: i.Assessment.ColonyStrength, QueenStatus: i.Assessment.QueenStatus, BroodStatus: i.Assessment.BroodStatus, FoodStores: i.Assessment.FoodStores, HealthConcerns: i.Assessment.HealthConcerns, QueenObserved: i.Assessment.QueenObserved, EggsObserved: i.Assessment.EggsObserved, QueenCells: i.Assessment.QueenCells, QueenCondition: i.Assessment.QueenCondition, BroodAmount: i.Assessment.BroodAmount, BroodPattern: i.Assessment.BroodPattern, BroodStages: i.Assessment.BroodStages, BroodConcerns: i.Assessment.BroodConcerns, HealthOverallCondition: i.Assessment.HealthOverallCondition, PestSigns: i.Assessment.PestSigns, HealthWarningSigns: i.Assessment.HealthWarningSigns, HealthConcernLevel: i.Assessment.HealthConcernLevel, FeedingNeed: i.Assessment.FeedingNeed, FeedingPerformed: i.Assessment.FeedingPerformed, FeedTypes: i.Assessment.FeedTypes, Season: i.Assessment.Season, SeasonalStoreReadiness: i.Assessment.SeasonalStoreReadiness, SeasonalReadiness: i.Assessment.SeasonalReadiness, SeasonalConcerns: i.Assessment.SeasonalConcerns}
	}
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

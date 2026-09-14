package inspection

import (
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/sbezhuk/beebase-common/medialink"
	"github.com/sbezhuk/beebase-inspection-service/internal/domain/inspection"
)

// ImageResponse is the public representation of one image attached to an
// inspection: its media id, plus the URL a client loads/caches it from.
// The URL is derived, not stored - it's always media-service's stable
// download route, built fresh on every response.
type ImageResponse struct {
	ID       uuid.UUID `json:"id"`
	ImageURL string    `json:"image_url"`
}

// Response is the public representation of an inspection. TypeLabel is
// derived from Type via inspection.Type.Label - the same single source of
// truth the type_invalid validation check reads from - so a client never
// has to maintain its own copy of the type-to-label mapping.
type Response struct {
	ID          uuid.UUID       `json:"id"`
	HiveID      uuid.UUID       `json:"hive_id"`
	InspectedAt time.Time       `json:"inspected_at"`
	Notes       string          `json:"notes"`
	Type        inspection.Type `json:"type"`
	TypeLabel   string          `json:"type_label"`
	Images      []ImageResponse `json:"images"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
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
	return Response{
		ID:          i.ID,
		HiveID:      i.HiveID,
		InspectedAt: i.InspectedAt,
		Notes:       i.Notes,
		Type:        i.Type,
		TypeLabel:   i.Type.Label(),
		Images:      images,
		CreatedAt:   i.CreatedAt,
		UpdatedAt:   i.UpdatedAt,
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
	HiveID            uuid.UUID `json:"hive_id"`
	LatestInspectedAt time.Time `json:"latest_inspected_at"`
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
	ThresholdDays int                        `json:"threshold_days"`
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

package inspection

import (
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

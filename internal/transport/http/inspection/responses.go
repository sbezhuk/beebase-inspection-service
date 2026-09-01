package inspection

import (
	"time"

	"github.com/google/uuid"

	"github.com/sbezhuk/beebase-inspection-service/internal/domain/inspection"
)

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
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

func newResponse(i *inspection.Inspection) Response {
	return Response{
		ID:          i.ID,
		HiveID:      i.HiveID,
		InspectedAt: i.InspectedAt,
		Notes:       i.Notes,
		Type:        i.Type,
		TypeLabel:   i.Type.Label(),
		CreatedAt:   i.CreatedAt,
		UpdatedAt:   i.UpdatedAt,
	}
}

func newListResponse(inspections []*inspection.Inspection) []Response {
	out := make([]Response, len(inspections))
	for idx, i := range inspections {
		out[idx] = newResponse(i)
	}
	return out
}

package inspection

import (
	"time"

	"github.com/google/uuid"

	"github.com/sbezhuk/beebase-inspection-service/internal/domain/inspection"
)

// Response is the public representation of an inspection.
type Response struct {
	ID          uuid.UUID `json:"id"`
	HiveID      uuid.UUID `json:"hive_id"`
	InspectedAt time.Time `json:"inspected_at"`
	Notes       string    `json:"notes"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func newResponse(i *inspection.Inspection) Response {
	return Response{
		ID:          i.ID,
		HiveID:      i.HiveID,
		InspectedAt: i.InspectedAt,
		Notes:       i.Notes,
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

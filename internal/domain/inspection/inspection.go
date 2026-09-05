// Package inspection holds the Inspection entity and the port through
// which the rest of the application persists and retrieves it. It has no
// dependency on HTTP, PostgreSQL, or any other infrastructure concern.
package inspection

import (
	"time"

	"github.com/google/uuid"
)

// Inspection is a beekeeper's record of inspecting one hive on a given
// date. It is a synchronizable entity (UUID, created_at, updated_at,
// deleted_at) per the project's offline-sync plan, even though full sync
// isn't implemented yet.
type Inspection struct {
	ID     uuid.UUID
	UserID uuid.UUID // denormalized owner; see Repository doc comment
	HiveID uuid.UUID // immutable after creation

	InspectedAt time.Time // when the inspection took place, not a bookkeeping timestamp
	Notes       string
	Type        Type
	// Images is the set of media ids attached to this inspection - the
	// source of truth for what's attached (nothing asks media-service on
	// every read). Never nil; empty when there are no photos.
	Images []uuid.UUID

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

// New constructs an Inspection owned by userID for hiveID, with a freshly
// generated ID and CreatedAt/UpdatedAt set to now. Callers must have
// already verified that hiveID belongs to userID, and that t is Valid(),
// before calling New.
func New(userID, hiveID uuid.UUID, inspectedAt time.Time, notes string, t Type) *Inspection {
	now := time.Now().UTC()
	return &Inspection{
		ID:          uuid.New(),
		UserID:      userID,
		HiveID:      hiveID,
		InspectedAt: inspectedAt,
		Notes:       notes,
		Type:        t,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

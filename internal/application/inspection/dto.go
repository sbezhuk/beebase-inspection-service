package inspection

import (
	"time"

	"github.com/google/uuid"
)

// CreateInput is the input to Service.Create.
type CreateInput struct {
	HiveID      uuid.UUID
	InspectedAt time.Time
	Notes       string
}

// UpdateInput is the input to Service.Update. Update replaces both
// editable fields (PUT semantics), not a partial patch. HiveID isn't
// here: an inspection can't be moved to a different hive.
type UpdateInput struct {
	InspectedAt time.Time
	Notes       string
}

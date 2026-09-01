package inspection

import (
	"time"

	"github.com/google/uuid"

	"github.com/sbezhuk/beebase-inspection-service/internal/domain/inspection"
)

// CreateInput is the input to Service.Create.
type CreateInput struct {
	HiveID      uuid.UUID
	InspectedAt time.Time
	Notes       string
	Type        inspection.Type
}

// UpdateInput is the input to Service.Update. Update replaces every
// editable field (PUT semantics), not a partial patch. HiveID isn't
// here: an inspection can't be moved to a different hive.
type UpdateInput struct {
	InspectedAt time.Time
	Notes       string
	Type        inspection.Type
}

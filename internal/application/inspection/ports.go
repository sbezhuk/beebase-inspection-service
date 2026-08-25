package inspection

import (
	"context"

	"github.com/google/uuid"
)

// HiveVerifier confirms that a hive belongs to whoever presented
// accessToken. It's a port because hives (and, transitively, apiaries)
// live in a different service; this service never queries hive or apiary
// ownership itself, it only ever asks hive-service.
type HiveVerifier interface {
	Verify(ctx context.Context, accessToken string, hiveID uuid.UUID) error
}

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

// MediaClient is inspection-service's dependency on media-service.
// media-service has no notion of hives or inspections at all - it only
// knows which files belong to which uploader - so inspection-service is
// fully self-sufficient for "what's attached to this inspection" (see
// Inspection.Images, its own local column and the sole source of truth
// for reads); this client exists purely to verify a caller's ownership of
// newly-referenced media ids before persisting them, and to hard-delete
// an inspection's files when it's cascade-deleted alongside its hive.
type MediaClient interface {
	// VerifyOwnership confirms every id in ids belongs to whoever
	// presented accessToken, by asking media-service directly - it's the
	// only remaining source of truth for "does this media id exist and
	// belong to me". Returns ErrImageNotFound if any id doesn't (unknown,
	// deleted, or someone else's - indistinguishable, by the same
	// non-leaking convention inspection.ErrNotFound already follows).
	VerifyOwnership(ctx context.Context, accessToken string, ids []uuid.UUID) error
	// DeleteByIDs hard-deletes every media item in ids, used when the
	// inspections under a hive are being cascade-deleted (DeleteByHive).
	DeleteByIDs(ctx context.Context, accessToken string, ids []uuid.UUID) error
}

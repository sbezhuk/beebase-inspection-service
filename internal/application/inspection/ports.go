package inspection

import (
	"context"

	"github.com/google/uuid"

	"github.com/sbezhuk/beebase-inspection-service/internal/domain/inspection"
)

// HiveVerifier confirms that a hive belongs to whoever presented
// accessToken, and resolves its current Free/Pro writability. It's a port
// because hives (and, transitively, apiaries) live in a different
// service; this service never queries hive/apiary ownership or
// entitlement itself, it only ever asks hive-service - the sole source of
// truth for both, and in particular the sole source of truth for whether
// a hive's parent apiary is also within the caller's entitlement (see
// hive-service's own transitive apiary check).
type HiveVerifier interface {
	// Verify confirms hiveID belongs to whoever presented accessToken,
	// and reports whether hive-service currently considers it writable
	// (always true under Pro; under Free, true only when its parent
	// apiary is itself writable and it ranks within the caller's Free
	// hive entitlement). Returns ErrHiveNotFound if it doesn't belong to
	// them (or doesn't exist).
	Verify(ctx context.Context, accessToken string, hiveID uuid.UUID) (writable bool, err error)
}

// HealthInspectionReader supplies the complete non-deleted inspection
// history for a hive. It is separate from the paginated UI-list contract so
// a derived health result cannot accidentally depend on page size or page
// number.
type HealthInspectionReader interface {
	ListAllByHive(ctx context.Context, userID, hiveID uuid.UUID) ([]*inspection.Inspection, error)
}

type InternalReportReader interface {
	ListAllByHiveInternal(ctx context.Context, hiveID uuid.UUID) ([]*inspection.Inspection, error)
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

// Entitlement values returned by subscription-service, mirroring the
// convention hive-service and apiary-service already established for
// their own EntitlementResolver ports.
const (
	EntitlementFree = "free"
	EntitlementPro  = "pro"
)

// EntitlementResolver resolves the subscription entitlement for a user by
// their access token. It's a port because subscriptions live in a
// different service; this service never evaluates entitlement itself, it
// only ever asks subscription-service, the sole source of truth. Unlike
// HiveVerifier's writability (which gates how many Free resources exist),
// this gates an entire feature - Colony Health history - behind Pro
// regardless of resource counts, so it's checked independently.
type EntitlementResolver interface {
	GetEntitlement(ctx context.Context, accessToken string) (string, error)
}

const (
	// MaxMediaAttachments is the maximum number of media attachments allowed per inspection.
	MaxMediaAttachments = 5
)

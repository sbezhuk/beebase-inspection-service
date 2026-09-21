package inspection

import "errors"

// ErrHiveNotFound is returned when the hive an inspection is being
// created under doesn't exist, doesn't belong to the caller (which also
// covers its apiary not belonging to the caller, since hive-service's own
// ownership check is transitive), or its ownership couldn't be
// confirmed. As with inspection.ErrNotFound, these cases are deliberately
// indistinguishable: a caller must not be able to tell whether another
// user's hive ID exists at all.
var ErrHiveNotFound = errors.New("hive not found")

// ErrImageNotFound is returned when an ID in CreateInput.Images or
// UpdateInput.Images doesn't belong to the caller, verified via a read
// against media-service (GET /api/v1/media?ids=) - whether because it
// doesn't exist, was deleted, or belongs to a different user, without
// distinguishing why, by the same non-leaking convention
// inspection.ErrNotFound already follows.
var ErrImageNotFound = errors.New("image not found")

// ErrMediaLimitReached is returned when an attempt is made to attach more
// photos than permitted by the media attachment limit.
var ErrMediaLimitReached = errors.New("media limit reached")

// ErrHiveReadOnly is returned when a free-tier user attempts to create or
// update an inspection whose parent hive currently falls outside their
// Free entitlement (see hive-service's writable selection) - i.e. the
// hive itself, or its own parent apiary, requires Pro. Distinct from
// ErrHiveNotFound: the hive exists and belongs to the caller, it's simply
// not writable right now.
var ErrHiveReadOnly = errors.New("hive is read-only under the free plan")

var ErrAssessmentInvalid = errors.New("invalid inspection assessment")

// ErrHealthHistoryProRequired is returned when a Free-entitlement caller
// requests Colony Health history for a hive they own. Distinct from
// ErrHiveReadOnly: it doesn't matter whether the hive itself is currently
// writable under Free - health history is gated on the caller's own
// subscription entitlement, not on hive/apiary resource limits.
var ErrHealthHistoryProRequired = errors.New("colony health history requires pro")
